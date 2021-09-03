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
	)

	BeforeEach(func() {
		handler = &agentPodHandler{
			namespace:       namespace,
			agentImage:      agentImage,
			strongswanImage: strongswanImage,
			imagePullPolicy: corev1.PullIfNotPresent,
			logLevel:        3,
			client:          k8sClient,
			log:             klogr.New().WithName("agentPodHandler"),
		}

		nodeName := getNodeName()
		agentPodName = getAgentPodName(nodeName)
		node = newNode(nodeName, "10.40.20.181", "2.2.2.2/26")

		Expect(handler.Do(context.TODO(), node)).To(Succeed())
	})

	It("should create a agent pod if it's not exists", func() {
		var pod corev1.Pod
		agentPodName := getAgentPodName(node.Name)
		err := k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)
		Expect(err).ShouldNot(HaveOccurred())

		// pod
		Expect(pod.Spec.NodeName).To(Equal(node.Name))
		Expect(pod.Namespace).To(Equal(namespace))
		Expect(pod.Name).To(Equal(agentPodName))
		Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
		Expect(pod.Labels[constants.KeyPodHash]).ShouldNot(BeEmpty())

		Expect(len(pod.Spec.InitContainers)).To(Equal(1))
		Expect(len(pod.Spec.Containers)).To(Equal(2))
		Expect(len(pod.Spec.Volumes)).To(Equal(8))
		Expect(*pod.Spec.AutomountServiceAccountToken).To(BeFalse())

		hostPathDirectory := corev1.HostPathDirectory
		hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
		defaultMode := int32(420)
		edgeTunnelConfigMap := getAgentConfigMapName(node.Name)
		volumes := []corev1.Volume{
			{
				Name: "var-run",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
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
		}
		Expect(pod.Spec.Volumes).To(Equal(volumes))

		// k8s auto add tolerations for pod: node.kubernetes.io/not-ready:NoExecute and node.kubernetes.io/unreachable:NoExecute
		Expect(len(pod.Spec.Tolerations)).To(Equal(3))

		tolerations := corev1.Toleration{
			Key:    "node-role.kubernetes.io/edge",
			Effect: corev1.TaintEffectNoSchedule,
		}
		Expect(pod.Spec.Tolerations[0]).To(Equal(tolerations))

		// install-cni initContainer
		Expect(pod.Spec.InitContainers[0].Name).To(Equal("install-cni"))
		Expect(pod.Spec.Containers[0].Image).To(Equal(agentImage))
		Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(handler.imagePullPolicy))

		cpCommand := []string{
			"/bin/sh",
		}
		Expect(pod.Spec.InitContainers[0].Command).To(Equal(cpCommand))

		cpCommandArgs := []string{
			"-c",
			installCNIScript,
		}
		Expect(pod.Spec.InitContainers[0].Args).To(Equal(cpCommandArgs))

		Expect(len(pod.Spec.InitContainers[0].VolumeMounts)).To(Equal(3))
		cniVolumeMounts := []corev1.VolumeMount{
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
		}
		Expect(pod.Spec.InitContainers[0].VolumeMounts).To(Equal(cniVolumeMounts))

		// agent container
		Expect(pod.Spec.Containers[0].Name).To(Equal("agent"))
		Expect(pod.Spec.Containers[0].Image).To(Equal(agentImage))
		Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(handler.imagePullPolicy))
		args := []string{
			"-tunnels-conf",
			agentConfigTunnelsFilepath,
			"-services-conf",
			agentConfigServicesFilepath,
			"-local-cert",
			"tls.crt",
			"-masq-outgoing=false",
			"-use-xfrm=false",
			"-enable-proxy=false",
			"-v=3",
		}
		Expect(pod.Spec.Containers[0].Args).To(Equal(args))

		privileged := true
		Expect(pod.Spec.Containers[0].SecurityContext.Privileged).To(Equal(&privileged))

		Expect(len(pod.Spec.Containers[0].VolumeMounts)).To(Equal(5))
		agentVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "netconf",
				MountPath: "/etc/fabedge",
			},
			{
				Name:      "var-run",
				MountPath: "/var/run/",
			},
			{
				Name:      "cni-config",
				MountPath: "/etc/cni/net.d",
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
		}
		Expect(pod.Spec.Containers[0].VolumeMounts).To(Equal(agentVolumeMounts))

		// strongswan container
		Expect(pod.Spec.Containers[1].Name).To(Equal("strongswan"))
		Expect(pod.Spec.Containers[1].Image).To(Equal(strongswanImage))
		Expect(pod.Spec.Containers[1].SecurityContext.Privileged).To(Equal(&privileged))
		Expect(pod.Spec.Containers[1].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		Expect(len(pod.Spec.Containers[1].VolumeMounts)).To(Equal(3))

		strongswanVolumeMounts := []corev1.VolumeMount{
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
		}
		Expect(pod.Spec.Containers[1].VolumeMounts).To(Equal(strongswanVolumeMounts))
	})

	It("should delete agent pod if is not matched to expected pod spec", func() {
		var pod corev1.Pod
		Expect(k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)).Should(Succeed())

		pod.Labels[constants.KeyPodHash] = "different-hash"
		Expect(k8sClient.Update(context.Background(), &pod)).Should(Succeed())

		Expect(handler.Do(context.Background(), node)).Should(Succeed())

		pod = corev1.Pod{}
		err := k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
	})

	It("is able to delete agent pod for specified node", func() {
		Expect(handler.Undo(context.TODO(), node.Name)).To(Succeed())

		pod := corev1.Pod{}
		err := k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
	})
})
