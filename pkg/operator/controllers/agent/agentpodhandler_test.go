// Copyright 2021 FabEdge Team
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agent

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

var _ = Describe("AgentPodHandler", func() {
	var (
		agentPodName string
		node         corev1.Node

		namespace       = "default"
		agentImage      = "fabedge/agent:latest"
		strongswanImage = "strongswan:5.9.1"

		handler *agentPodHandler
		newNode = newNodePodCIDRsInAnnotations
		argMap  types.AgentArgumentMap
	)

	BeforeEach(func() {
		argMap = map[string]string{
			"enable-proxy":       "false",
			"enable-hairpinmode": "true",
			"masq-outgoing":      "false",
			"network-plugin-mtu": "1400",
			"use-xfrm":           "false",
			"log-level":          "3",
		}

		handler = &agentPodHandler{
			namespace:       namespace,
			agentImage:      agentImage,
			strongswanImage: strongswanImage,
			imagePullPolicy: corev1.PullIfNotPresent,
			args:            argMap.ArgumentArray(),
			client:          k8sClient,
			log:             klogr.New().WithName("agentPodHandler"),
		}

		nodeName := getNodeName()
		agentPodName = getAgentPodName(nodeName)
		node = newNode(nodeName, "10.40.20.181", "2.2.2.2/26")
		node.UID = "123456"

		Expect(handler.Do(context.TODO(), node)).To(Succeed())
	})

	It("should create a agent pod if it's not exists", func() {
		pod, err := handler.getAgentPod(context.Background(), agentPodName)
		Expect(err).Should(BeNil())

		expectOwnerReference(&pod, node)

		Expect(pod.Spec.NodeName).To(Equal(node.Name))
		Expect(pod.Namespace).To(Equal(namespace))
		Expect(pod.GenerateName).To(Equal(agentNamePrefix))
		Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
		Expect(pod.Labels[constants.KeyPodHash]).ShouldNot(BeEmpty())

		Expect(*pod.Spec.AutomountServiceAccountToken).To(BeFalse())

		hostPathDirectory := corev1.HostPathDirectory
		hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
		defaultMode := int32(420)
		edgeTunnelConfigMap := getAgentConfigMapName(node.Name)
		expectedVolumes := []corev1.Volume{
			{
				Name: "var-run",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			{
				Name: "lib-modules",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/lib/modules",
						Type: &hostPathDirectory,
					},
				},
			},
			{
				Name: "netconf",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: edgeTunnelConfigMap,
						},
						DefaultMode: &defaultMode,
					},
				},
			},
			{
				Name: "ipsec-d",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  getCertSecretName(node.Name),
						DefaultMode: &defaultMode,
						Items: []corev1.KeyToPath{
							{
								Key:  secretutil.KeyCACert,
								Path: "cacerts/ca.crt",
							},
							{
								Key:  corev1.TLSCertKey,
								Path: "certs/tls.crt",
							},
							{
								Key:  corev1.TLSPrivateKeyKey,
								Path: "private/tls.key",
							},
						},
					},
				},
			},
			{
				Name: "ipsec-secrets",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  getCertSecretName(node.Name),
						DefaultMode: &defaultMode,
						Items: []corev1.KeyToPath{
							{
								Key:  secretutil.KeyIPSecSecretsFile,
								Path: "ipsec.secrets",
							},
						},
					},
				},
			},
			{
				Name: "cni-config",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc/cni/net.d",
						Type: &hostPathDirectoryOrCreate,
					},
				},
			},
			{
				Name: "cni-bin",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/opt/cni/bin",
						Type: &hostPathDirectoryOrCreate,
					},
				},
			},
			{
				Name: "cni-cache",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/lib/cni/cache",
						Type: &hostPathDirectoryOrCreate,
					},
				},
			},
			{
				Name: "agent-workdir",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		}
		Expect(pod.Spec.Volumes).To(Equal(expectedVolumes))
		Expect(pod.Spec.Tolerations).To(ConsistOf(corev1.Toleration{
			Key:      "",
			Operator: corev1.TolerationOpExists,
		}))

		// install-cni initContainer
		Expect(len(pod.Spec.InitContainers)).To(Equal(1))
		epContainer := pod.Spec.InitContainers[0]
		Expect(epContainer.Name).To(Equal("environment-prepare"))
		Expect(epContainer.Image).To(Equal(agentImage))
		Expect(epContainer.ImagePullPolicy).To(Equal(handler.imagePullPolicy))
		Expect(epContainer.Command).To(ConsistOf("env_prepare.sh"))
		Expect(*epContainer.SecurityContext.Privileged).To(BeTrue())
		Expect(epContainer.VolumeMounts).To(Equal([]corev1.VolumeMount{
			{
				Name:      "cni-bin",
				MountPath: "/opt/cni/bin",
			},
			{
				Name:      "cni-cache",
				MountPath: "/var/lib/cni/cache",
			},
			{
				Name:      "cni-config",
				MountPath: "/etc/cni/net.d",
			},
		}))

		agentContainer := pod.Spec.Containers[0]
		Expect(agentContainer.Name).To(Equal("agent"))
		Expect(agentContainer.Image).To(Equal(agentImage))
		Expect(agentContainer.ImagePullPolicy).To(Equal(handler.imagePullPolicy))
		Expect(*agentContainer.SecurityContext.Privileged).To(BeTrue())
		Expect(agentContainer.Args).To(ConsistOf(
			"--enable-hairpinmode=true",
			"--enable-proxy=false",
			"--masq-outgoing=false",
			"--network-plugin-mtu=1400",
			"--use-xfrm=false",
			"--v=3",
		))
		Expect(agentContainer.VolumeMounts).To(Equal([]corev1.VolumeMount{
			{
				Name:      "netconf",
				MountPath: "/etc/fabedge",
			},
			{
				Name:      "cni-config",
				MountPath: "/etc/cni/net.d",
			},
			{
				Name:      "var-run",
				MountPath: "/var/run/",
			},
			{
				Name:      "lib-modules",
				MountPath: "/lib/modules",
				ReadOnly:  true,
			},
			{
				Name:      "ipsec-d",
				MountPath: "/etc/ipsec.d",
				ReadOnly:  true,
			},
			{
				Name:      "agent-workdir",
				MountPath: "/var/lib/fabedge",
			},
		}))

		ssContainer := pod.Spec.Containers[1]
		Expect(ssContainer.Name).To(Equal("strongswan"))
		Expect(ssContainer.Image).To(Equal(strongswanImage))
		Expect(*ssContainer.SecurityContext.Privileged).To(BeTrue())
		Expect(ssContainer.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		Expect(ssContainer.VolumeMounts).To(Equal([]corev1.VolumeMount{
			{
				Name:      "var-run",
				MountPath: "/var/run/",
			},
			{
				Name:      "ipsec-d",
				MountPath: "/etc/ipsec.d",
				ReadOnly:  true,
			},
			{
				Name:      "ipsec-secrets",
				MountPath: "/etc/ipsec.secrets",
				SubPath:   "ipsec.secrets",
				ReadOnly:  true,
			},
		}))
	})

	It("should delete agent pod if errRestartAgent is passed in context", func() {
		ctx := context.WithValue(context.Background(), keyRestartAgent, errRestartAgent)
		Expect(handler.Do(ctx, node)).Should(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentPodName)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
	})

	It("should delete agent pod if is not matched to expected pod spec", func() {
		pod, err := handler.getAgentPod(context.Background(), agentPodName)

		pod.Labels[constants.KeyPodHash] = "different-hash"
		Expect(k8sClient.Update(context.Background(), &pod)).Should(Succeed())

		Expect(handler.Do(context.Background(), node)).Should(Succeed())

		pod, err = handler.getAgentPod(context.Background(), agentPodName)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
	})

	It("is able to delete agent pod for specified node", func() {
		Expect(handler.Undo(context.TODO(), node.Name)).To(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentPodName)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
	})
})
