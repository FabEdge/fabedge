// Copyright 2021 BoCloud
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
	"net"
	"time"

	"github.com/jjeffery/stringset"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	. "github.com/fabedge/fabedge/internal/util/ginkgoext"
	testutil "github.com/fabedge/fabedge/internal/util/test"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	"github.com/fabedge/fabedge/pkg/operator/predicates"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ = Describe("AgentController", func() {
	const (
		timeout         = 5 * time.Second
		namespace       = "default"
		agentImage      = "fabedge/agent:latest"
		strongswanImage = "strongswan:5.9.1"
		edgePodCIDR     = "2.0.0.0"
	)

	var (
		requests chan reconcile.Request
		store    storepkg.Interface
		alloc    allocator.Interface
		ctx      context.Context
		cancel   context.CancelFunc

		newEndpoint = types.GenerateNewEndpointFunc("C=CN, O=StrongSwan, CN={node}")

		getNodeName = testutil.GenerateGetNameFunc("edge")
	)

	BeforeEach(func() {
		store = storepkg.NewStore()

		ctx, cancel = context.WithCancel(context.Background())

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		alloc, _ = allocator.New("2.2.0.0/16")

		reconciler := reconcile.Reconciler(&agentController{
			namespace:       namespace,
			agentImage:      agentImage,
			strongswanImage: strongswanImage,
			edgePodCIRD:     edgePodCIDR,

			client:      mgr.GetClient(),
			alloc:       alloc,
			store:       store,
			newEndpoint: newEndpoint,

			log: mgr.GetLogger().WithName(controllerName),
		})
		reconciler, requests = testutil.WrapReconcile(reconciler)
		c, err := controller.New(
			controllerName,
			mgr,
			controller.Options{
				Reconciler: reconciler,
			},
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = c.Watch(
			&source.Kind{Type: &corev1.Node{}},
			&handler.EnqueueRequestForObject{},
			predicates.EdgeNodePredicate(),
		)
		Expect(err).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()
	})

	AfterEach(func() {
		cancel()
	})

	When("a node is created", func() {
		It("skip reconciling if this node has no ip", func() {
			nodeName := getNodeName()
			node := newNode(nodeName, "", "")

			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			// create event
			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			node = corev1.Node{}
			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(node.Annotations[constants.KeyPodSubnets]).Should(BeEmpty())
		})

		It("should allocate a subnet to a edge node if this node has no subnet", func() {
			nodeName := getNodeName()
			node := newNode(nodeName, "10.40.20.181", "")

			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			// create event
			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			// update event
			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			// should not receive any request
			Eventually(requests, timeout).ShouldNot(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(node.Annotations[constants.KeyPodSubnets]).ShouldNot(BeEmpty())

			ep, ok := store.GetEndpoint(nodeName)
			Expect(ok).To(BeTrue())
			Expect(ep.Subnets[0]).To(Equal(node.Annotations[constants.KeyPodSubnets]))
		})

		It("should allocate a subnet to a edge node if this node's subnet is invalid", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "2.2.2.257/26")
			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(node.Annotations[constants.KeyPodSubnets]).ShouldNot(BeEmpty())

			_, _, err = net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should reallocate a subnet to a edge node if this node's subnet is out of expected range", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "2.3.2.1/26")
			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(node.Annotations[constants.KeyPodSubnets]).ShouldNot(BeEmpty())

			_, _, err = net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should reallocate a subnet to a edge node if this node's subnet is not match to record in store", func() {
			nodeName := getNodeName()
			store.SaveEndpoint(types.Endpoint{
				Name:    nodeName,
				IP:      "",
				Subnets: []string{"2.2.2.2/26"},
			})
			node := newNode(nodeName, "10.40.20.181", "2.2.2.1/26")
			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(node.Annotations[constants.KeyPodSubnets]).ShouldNot(BeEmpty())

			_, _, err = net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should create a agent pod to a edge node if this node's agent pod is not found", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "")
			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			// create event
			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			var pod corev1.Pod
			agentPodName := getAgentPodName(nodeName)
			err = k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)
			Expect(err).ShouldNot(HaveOccurred())

			// pod
			Expect(pod.Spec.NodeName).To(Equal(nodeName))
			Expect(pod.Namespace).To(Equal(namespace))
			Expect(pod.Name).To(Equal(agentPodName))
			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyAlways))
			Expect(pod.Labels[constants.KeyPodHash]).ShouldNot(BeEmpty())

			Expect(len(pod.Spec.InitContainers)).To(Equal(1))
			Expect(len(pod.Spec.Containers)).To(Equal(2))
			Expect(len(pod.Spec.Volumes)).To(Equal(8))

			hostPathDirectory := corev1.HostPathDirectory
			hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
			hostPathFile := corev1.HostPathFile
			defaultMode := int32(420)
			edgeTunnelConfigMap := getAgentConfigMapName(nodeName)
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
							Path: "/etc/cni",
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
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/fabedge/ipsec",
							Type: &hostPathDirectory,
						},
					},
				},
				{
					Name: "ipsec-secrets",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/fabedge/ipsec/ipsec.secrets",
							Type: &hostPathFile,
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
			Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))

			cpCommand := []string{
				"cp",
			}
			Expect(pod.Spec.InitContainers[0].Command).To(Equal(cpCommand))

			cpCommandArgs := []string{
				"-f",
				"/usr/local/bin/bridge",
				"/usr/local/bin/host-local",
				"/usr/local/bin/loopback",
				"/opt/cni/bin",
			}
			Expect(pod.Spec.InitContainers[0].Args).To(Equal(cpCommandArgs))

			Expect(len(pod.Spec.InitContainers[0].VolumeMounts)).To(Equal(2))
			cniVolumeMounts := []corev1.VolumeMount{
				{
					Name:      "cni-bin",
					MountPath: "/opt/cni/bin",
				},
				{
					Name:      "cni-cache",
					MountPath: "/var/lib/cni/cache",
				},
			}
			Expect(pod.Spec.InitContainers[0].VolumeMounts).To(Equal(cniVolumeMounts))

			// agent container
			Expect(pod.Spec.Containers[0].Name).To(Equal("agent"))
			Expect(pod.Spec.Containers[0].Image).To(Equal(agentImage))
			Expect(pod.Spec.Containers[0].ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			args := []string{
				"-tunnels-conf",
				agentConfigTunnelsFilepath,
				"-services-conf",
				agentConfigServicesFilepath,
				"-edge-pod-cidr",
				edgePodCIDR,
				"-masq-outgoing=false",
				"-use-xfrm=false",
				"-enable-proxy=false",
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
					MountPath: "/etc/cni",
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
					ReadOnly:  true,
				},
			}
			Expect(pod.Spec.Containers[1].VolumeMounts).To(Equal(strongswanVolumeMounts))
		})

		It("should delete agent pod if pod hash is changed", func() {
			nodeName := getNodeName()
			ctx := context.TODO()

			node := newNode(nodeName, "10.40.20.181", "")
			Expect(k8sClient.Create(ctx, &node)).Should(Succeed())
			Eventually(requests, timeout).Should(ReceiveKey(ObjectKey{Name: nodeName}))

			var pod corev1.Pod
			agentPodName := getAgentPodName(nodeName)
			Expect(k8sClient.Get(ctx, ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)).Should(Succeed())
			pod.Labels[constants.KeyPodHash] = "different-hash"
			Expect(k8sClient.Update(ctx, &pod)).Should(Succeed())

			// trigger reconciling
			node.ResourceVersion = ""
			node.Annotations = map[string]string{"something": "different"}
			Expect(k8sClient.Update(ctx, &node)).Should(Succeed())
			Eventually(requests, timeout).Should(ReceiveKey(ObjectKey{Name: nodeName}))

			pod = corev1.Pod{}
			Expect(k8sClient.Get(ctx, ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)).Should(Succeed())
			Expect(pod.DeletionTimestamp).ShouldNot(BeNil())
		})
	})

	Context("with community", func() {
		var (
			nodeName        string
			agentConfigName string

			connectorEndpoint, edge2Endpoint types.Endpoint
			testCommunity                    types.Community
		)

		BeforeEach(func() {
			nodeName = getNodeName()
			connectorEndpoint = types.Endpoint{
				ID:          "C=CN, O=StrongSwan, CN=connector",
				Name:        constants.ConnectorEndpointName,
				IP:          "192.168.1.1",
				Subnets:     []string{"2.2.1.1/26"},
				NodeSubnets: []string{"192.168.1.0/24"},
			}
			edge2Endpoint = types.Endpoint{
				ID:      "C=CN, O=StrongSwan, CN=edge2",
				Name:    "edge2",
				IP:      "10.20.8.141",
				Subnets: []string{"2.2.1.65/26"},
			}
			testCommunity = types.Community{
				Name:    "test",
				Members: stringset.New(edge2Endpoint.Name, nodeName),
			}

			store.SaveEndpoint(connectorEndpoint)
			store.SaveEndpoint(edge2Endpoint)
			store.SaveCommunity(testCommunity)

			agentConfigName = getAgentConfigMapName(nodeName)
			nodeEdge1 := newNode(nodeName, "10.40.20.181", "")

			err := k8sClient.Create(context.Background(), &nodeEdge1)
			Expect(err).ShouldNot(HaveOccurred())

			testutil.DrainChan(requests, timeout)
		})

		It("should create agent configmap when it is not created yet", func() {
			var cm corev1.ConfigMap

			err := k8sClient.Get(context.Background(), ObjectKey{Name: agentConfigName, Namespace: namespace}, &cm)
			Expect(err).ShouldNot(HaveOccurred())

			configData, ok := cm.Data[agentConfigServicesFileName]
			Expect(ok).Should(BeTrue())
			Expect(configData).Should(Equal(""))

			configData, ok = cm.Data[agentConfigTunnelFileName]
			Expect(ok).Should(BeTrue())

			var conf netconf.NetworkConf
			Expect(yaml.Unmarshal([]byte(configData), &conf)).ShouldNot(HaveOccurred())

			var node corev1.Node
			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())

			expectedConf := netconf.NetworkConf{
				TunnelEndpoint: newEndpoint(node).ConvertToTunnelEndpoint(),
				Peers: []netconf.TunnelEndpoint{
					connectorEndpoint.ConvertToTunnelEndpoint(),
					edge2Endpoint.ConvertToTunnelEndpoint(),
				},
			}
			Expect(conf).Should(Equal(expectedConf))
		})

		It("should update agent configmap when any endpoint changed", func() {
			By("changing edge2 ip address")
			edge2IP := "10.20.8.142"
			edge2Endpoint.IP = edge2IP
			store.SaveEndpoint(edge2Endpoint)

			By("assign an IP address to node")
			var node corev1.Node
			err := k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())
			node.Status.Addresses = []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "10.40.20.182",
				},
			}
			err = k8sClient.Status().Update(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))

			var cm corev1.ConfigMap
			err = k8sClient.Get(context.Background(), ObjectKey{Name: agentConfigName, Namespace: namespace}, &cm)
			Expect(err).ShouldNot(HaveOccurred())

			configData, ok := cm.Data[agentConfigTunnelFileName]
			Expect(ok).Should(BeTrue())

			var conf netconf.NetworkConf
			Expect(yaml.Unmarshal([]byte(configData), &conf)).ShouldNot(HaveOccurred())

			err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
			Expect(err).ShouldNot(HaveOccurred())

			expectedConf := netconf.NetworkConf{
				TunnelEndpoint: newEndpoint(node).ConvertToTunnelEndpoint(),
				Peers: []netconf.TunnelEndpoint{
					connectorEndpoint.ConvertToTunnelEndpoint(),
					edge2Endpoint.ConvertToTunnelEndpoint(),
				},
			}
			Expect(conf).Should(Equal(expectedConf))
			Expect(conf.Peers[1].IP).Should(Equal(edge2IP))
		})
	})

	When("a node is deleted", func() {
		const subnets = "2.2.0.0/16"

		var (
			nodeName string
			node     corev1.Node
		)

		BeforeEach(func() {
			nodeName = getNodeName()
			node = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					Labels: map[string]string{
						"node-role.kubernetes.io/edge": "",
					},
					Annotations: map[string]string{
						constants.KeyPodSubnets: subnets,
					},
				},
				Status: corev1.NodeStatus{
					Addresses: []corev1.NodeAddress{
						{
							Type:    corev1.NodeInternalIP,
							Address: "192.168.1.1",
						},
					},
				},
			}

			store.SaveEndpoint(newEndpoint(node))
			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			testutil.DrainChan(requests, 2*time.Second)

			err = k8sClient.Delete(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())

			Eventually(requests, timeout).Should(Receive(Equal(reconcile.Request{
				NamespacedName: ObjectKey{Name: nodeName},
			})))
		})

		It("should delete agent pod", func() {
			agentPodName := getAgentPodName(node.Name)

			var pod corev1.Pod
			err := k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(pod.DeletionTimestamp).ShouldNot(BeNil())
		})

		It("should reclaim subnets allocated to this node", func() {
			_, ok := store.GetEndpoint(nodeName)
			Expect(ok).To(BeFalse())

			_, cidr, _ := net.ParseCIDR(subnets)
			Expect(alloc.IsAllocated(*cidr)).To(BeFalse())
		})

		It("should delete agent configmap", func() {
			configName := getAgentConfigMapName(nodeName)

			err := k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: configName}, &corev1.ConfigMap{})
			Expect(err).Should(HaveOccurred())
			Expect(errors.IsNotFound(err)).Should(BeTrue())
		})
	})
})

func newNode(name, ip, subnets string) corev1.Node {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node-role.kubernetes.io/edge": "",
			},
		},
	}

	if subnets != "" {
		node.Annotations = map[string]string{
			constants.KeyPodSubnets: subnets,
		}
	}

	if ip != "" {
		node.Status = corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: "10.40.20.181",
				},
			},
		}
	}

	return node
}
