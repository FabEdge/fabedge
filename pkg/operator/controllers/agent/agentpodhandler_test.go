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
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("AgentPodHandler", func() {
	var (
		agentName string
		node      corev1.Node

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
			argMap:          argMap,
			agentNameSet:    types.NewSafeStringSet(),
			client:          k8sClient,
			log:             klogr.New().WithName("agentPodHandler"),
		}

		nodeName := getNodeName()
		agentName = getAgentName(nodeName)
		node = newNode(nodeName, "10.40.20.181", "2.2.2.2/26")
		node.UID = "123456"

		Expect(handler.Do(context.TODO(), node)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testutil.PurgeAllPods(k8sClient)).To(Succeed())
	})

	It("should create a agent pod if it's not exists", func() {
		Expect(handler.agentNameSet.Has(agentName)).To(BeTrue())
		pod, err := handler.getAgentPod(context.Background(), agentName)
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
		Expect(epContainer.Command).To(ConsistOf("/plugins/env_prepare.sh"))
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
		Expect(agentContainer.ReadinessProbe).To(BeNil())
		Expect(agentContainer.LivenessProbe).To(BeNil())
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

	It("should create readinessProbe and livenessProbe on agent container if both enable-dns and dns-probe are true", func() {
		handler.argMap.Set("enable-dns", "true")
		handler.argMap.Set("dns-probe", "true")
		handler.args = handler.argMap.ArgumentArray()

		nodeName := getNodeName()
		agentName := getAgentName(nodeName)
		node = newNode(nodeName, "10.40.20.182", "2.2.2.3/26")
		node.UID = "234567"

		Expect(handler.Do(context.TODO(), node)).To(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentName)
		Expect(err).Should(BeNil())
		agentContainer := pod.Spec.Containers[0]
		Expect(agentContainer.Args).To(ConsistOf(
			"--dns-probe=true",
			"--enable-dns=true",
			"--enable-hairpinmode=true",
			"--enable-proxy=false",
			"--masq-outgoing=false",
			"--network-plugin-mtu=1400",
			"--use-xfrm=false",
			"--v=3",
		))
		Expect(*agentContainer.LivenessProbe).To(Equal(corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			FailureThreshold: 3,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			TimeoutSeconds:   1,
		}))
		Expect(*agentContainer.ReadinessProbe).To(Equal(corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/ready",
					Port:   intstr.FromInt(8181),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			FailureThreshold: 3,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			TimeoutSeconds:   1,
		}))
	})

	It("should mount /run/xtables.lock file agent container if enable-proxy is true", func() {
		handler.argMap.Set("enable-proxy", "true")
		handler.argMap.Set("proxy-cluster-cidr", "2.2.0.0/16")
		handler.args = handler.argMap.ArgumentArray()

		nodeName := getNodeName()
		agentName := getAgentName(nodeName)
		node = newNode(nodeName, "10.40.20.182", "2.2.2.3/26")
		node.UID = "234567"

		Expect(handler.Do(context.TODO(), node)).To(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentName)
		Expect(err).Should(BeNil())

		xtableLockVolume := pod.Spec.Volumes[len(pod.Spec.Volumes)-1]
		xtablesHostType := corev1.HostPathFileOrCreate
		Expect(xtableLockVolume).To(Equal(corev1.Volume{
			Name: "xtables-lock",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Type: &xtablesHostType,
					Path: "/run/xtables.lock",
				},
			},
		}))

		agentContainer := pod.Spec.Containers[0]
		xtableLockVolumeMount := agentContainer.VolumeMounts[len(agentContainer.VolumeMounts)-1]
		Expect(agentContainer.Args).To(ConsistOf(
			"--enable-hairpinmode=true",
			"--enable-proxy=true",
			"--masq-outgoing=false",
			"--network-plugin-mtu=1400",
			"--proxy-cluster-cidr=2.2.0.0/16",
			"--use-xfrm=false",
			"--v=3",
		))
		Expect(xtableLockVolumeMount).To(Equal(corev1.VolumeMount{
			Name:      "xtables-lock",
			MountPath: "/run/xtables.lock",
		}))
	})

	It("should use arguments from node's annotation and default arguments to build agent pod", func() {
		nodeName := getNodeName()
		agentName = getAgentName(nodeName)
		node = newNode(nodeName, "10.40.20.182", "2.2.2.3/26")
		node.UID = "234567"
		node.Annotations = map[string]string{
			"argument.fabedge.io/cni-version": "0.3.2",
		}

		Expect(handler.Do(context.TODO(), node)).To(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentName)
		Expect(err).Should(BeNil())
		Expect(pod.Spec.Containers[0].Args).To(ConsistOf(
			"--cni-version=0.3.2",
			"--enable-hairpinmode=true",
			"--enable-proxy=false",
			"--masq-outgoing=false",
			"--network-plugin-mtu=1400",
			"--use-xfrm=false",
			"--v=3",
		))
	})

	It("return errRequeueRequest if an agent pod is already created but agentPodHandler is not able to get it", func() {
		nodeName := getNodeName()
		agentName = getAgentName(nodeName)
		node = newNode(nodeName, "10.40.20.182", "2.2.3.2/26")
		node.UID = "2382203"

		handler.agentNameSet.Insert(agentName)
		Expect(handler.Do(context.Background(), node)).To(Equal(errRequeueRequest))
		Expect(handler.agentNameSet.Has(agentName)).To(BeFalse())
	})

	It("should delete agent pod if errRestartAgent is passed in context", func() {
		ctx := context.WithValue(context.Background(), keyRestartAgent, errRestartAgent)
		Expect(handler.Do(ctx, node)).Should(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentName)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
		Expect(handler.agentNameSet.Has(agentName)).To(BeFalse())
	})

	It("should delete agent pod if is not matched to expected pod spec", func() {
		pod, err := handler.getAgentPod(context.Background(), agentName)

		pod.Labels[constants.KeyPodHash] = "different-hash"
		Expect(k8sClient.Update(context.Background(), &pod)).Should(Succeed())

		Expect(handler.Do(context.Background(), node)).Should(Succeed())

		pod, err = handler.getAgentPod(context.Background(), agentName)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
		Expect(handler.agentNameSet.Has(agentName)).To(BeFalse())
	})

	It("is able to delete agent pod for specified node", func() {
		Expect(handler.Undo(context.TODO(), node.Name)).To(Succeed())

		pod, err := handler.getAgentPod(context.Background(), agentName)
		Expect(errors.IsNotFound(err) || pod.DeletionTimestamp != nil).Should(BeTrue())
		Expect(handler.agentNameSet.Has(agentName)).To(BeFalse())
	})
})
