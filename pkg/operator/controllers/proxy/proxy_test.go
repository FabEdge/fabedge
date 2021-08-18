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

package proxy

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/fabedge/fabedge/internal/util/ginkgoext"
	testutil "github.com/fabedge/fabedge/internal/util/test"
	"github.com/fabedge/fabedge/pkg/operator/predicates"
)

var _ = Describe("Proxy", func() {
	const defaultNamespace = "default"

	var (
		cancel           context.CancelFunc
		serviceMap       ServiceMap
		endpointSliceMap EndpointSliceMap
		edgeNodeSet      EdgeNodeSet
		keeper           *loadBalanceConfigKeeper
		px               *proxy
		mgr              manager.Manager

		getServiceName = testutil.GenerateGetNameFunc("nginx")
		getNodeName    = testutil.GenerateGetNameFunc("node")
	)

	BeforeEach(func() {
		var ctx context.Context
		var err error

		ctx, cancel = context.WithCancel(context.Background())
		mgr, err = manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		serviceMap = make(ServiceMap)
		endpointSliceMap = make(EndpointSliceMap)
		edgeNodeSet = make(EdgeNodeSet)
		keeper = &loadBalanceConfigKeeper{
			interval:  time.Second,
			namespace: defaultNamespace,
			nodeSet:   make(EdgeNodeSet),

			client: k8sClient,
			log:    mgr.GetLogger(),
		}

		px = &proxy{
			serviceMap:       serviceMap,
			endpointSliceMap: endpointSliceMap,
			nodeSet:          edgeNodeSet,
			keeper:           keeper,

			log:    mgr.GetLogger().WithName("fab-proxy"),
			client: mgr.GetClient(),
		}

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()
	})

	AfterEach(func() {
		ctx := context.Background()

		var services corev1.ServiceList
		Expect(k8sClient.List(ctx, &services)).ShouldNot(HaveOccurred())
		for _, obj := range services.Items {
			Expect(k8sClient.Delete(ctx, &obj)).ShouldNot(HaveOccurred())
		}

		var ess discoveryv1.EndpointSliceList
		Expect(k8sClient.List(ctx, &ess)).ShouldNot(HaveOccurred())
		for _, obj := range ess.Items {
			Expect(k8sClient.Delete(ctx, &obj)).ShouldNot(HaveOccurred())
		}

		cancel()
	})

	Context("service", func() {
		var (
			requests   chan reconcile.Request
			service    corev1.Service
			serviceKey ObjectKey
		)

		BeforeEach(func() {
			var reconciler reconcile.Func
			reconciler, requests = testutil.WrapReconcileFunc(px.OnServiceUpdate)
			err := addController("proxy-service", mgr, reconciler, &corev1.Service{})
			Expect(err).ShouldNot(HaveOccurred())

			service = corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getServiceName(),
					Namespace: defaultNamespace,
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"app": "nginx",
					},
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Name:     "health",
							Port:     8080,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			}
			serviceKey = ObjectKey{Name: service.Name, Namespace: service.Namespace}

			err = k8sClient.Create(context.Background(), &service)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(serviceKey))
		})

		AfterEach(func() {
			_ = k8sClient.Delete(context.Background(), &service)
		})

		It("should add service info to serviceMap when e service is updated", func() {
			serviceInfo, ok := serviceMap[serviceKey]
			Expect(ok).To(BeTrue())
			Expect(serviceInfo.ClusterIP).Should(Equal(service.Spec.ClusterIP))
			Expect(serviceInfo.SessionAffinity).Should(Equal(corev1.ServiceAffinityNone))
			Expect(serviceInfo.StickyMaxAgeSeconds).Should(Equal(int32(0)))
		})

		It("should update service info if either SessionAffinity or SessionAffinityConfig is changed", func() {
			By("update service")
			timeoutSeconds := int32(3600)
			service.Spec.SessionAffinity = corev1.ServiceAffinityClientIP
			service.Spec.SessionAffinityConfig = &corev1.SessionAffinityConfig{
				ClientIP: &corev1.ClientIPConfig{
					TimeoutSeconds: &timeoutSeconds,
				},
			}
			err := k8sClient.Update(context.Background(), &service)
			Expect(err).ShouldNot(HaveOccurred())

			testutil.DrainChan(requests, 5*time.Second)

			By("check service cache")
			serviceInfo, ok := serviceMap[serviceKey]
			Expect(ok).To(BeTrue())
			Expect(serviceInfo.ClusterIP).Should(Equal(service.Spec.ClusterIP))
			Expect(serviceInfo.SessionAffinity).Should(Equal(corev1.ServiceAffinityClientIP))
			Expect(serviceInfo.StickyMaxAgeSeconds).Should(Equal(timeoutSeconds))
		})

		Context("invalidate service", func() {
			const (
				endpointIP = "10.40.20.181"
				nodeName   = "node1"
			)

			BeforeEach(func() {
				serviceInfo := serviceMap[serviceKey]

				serviceInfo.EndpointMap = make(map[Port]EndpointSet)
				serviceInfo.EndpointToNodes = make(map[Endpoint]NodeName)
				node := newEdgeNode("node1")

				for _, port := range service.Spec.Ports {
					// prepare service info
					p := Port{Port: port.Port, Protocol: port.Protocol}
					endpoint := Endpoint{
						IP:   endpointIP,
						Port: port.Port,
					}
					endpointSet := make(EndpointSet)
					endpointSet.Add(endpoint)

					serviceInfo.EndpointMap[p] = endpointSet
					serviceInfo.EndpointToNodes[endpoint] = nodeName

					// prepare node info
					spn := ServicePortName{
						NamespacedName: serviceKey,
						Port:           port.Port,
						Protocol:       port.Protocol,
					}
					node.ServicePortMap[spn] = ServicePort{
						ClusterIP:           serviceInfo.ClusterIP,
						Port:                port.Port,
						Protocol:            port.Protocol,
						SessionAffinity:     serviceInfo.SessionAffinity,
						StickyMaxAgeSeconds: serviceInfo.StickyMaxAgeSeconds,
					}
					node.EndpointMap[spn] = endpointSet
				}

				serviceMap[serviceKey] = serviceInfo
				edgeNodeSet[nodeName] = node
			})

			It("should cleanup service info and endpoints in node when service is deleted", func() {
				Expect(k8sClient.Delete(context.Background(), &service)).ShouldNot(HaveOccurred())
				Eventually(requests, 2*time.Second).Should(ReceiveKey(serviceKey))

				Expect(len(serviceMap)).To(Equal(0))

				node := edgeNodeSet[nodeName]
				Expect(len(node.EndpointMap)).Should(Equal(0))
				Expect(len(node.ServicePortMap)).Should(Equal(0))

				Expect(keeper.nodeSet).Should(ContainElement(node))
			})

			It("should cleanup service info and endpoints in node when service has no selector", func() {
				service.Spec.Selector = nil
				Expect(k8sClient.Update(context.Background(), &service)).ShouldNot(HaveOccurred())
				Eventually(requests, 2*time.Second).Should(ReceiveKey(serviceKey))

				Expect(len(serviceMap)).To(Equal(0))

				node := edgeNodeSet[nodeName]
				Expect(len(node.EndpointMap)).Should(Equal(0))
				Expect(len(node.ServicePortMap)).Should(Equal(0))

				Expect(keeper.nodeSet).Should(ContainElement(node))
			})
		})
	})

	Context("node", func() {
		var requests chan reconcile.Request
		var node corev1.Node

		BeforeEach(func() {
			var reconciler reconcile.Func
			reconciler, requests = testutil.WrapReconcileFunc(px.onNodeUpdate)
			err := addController("proxy-node",
				mgr,
				reconciler,
				&corev1.Node{},
				predicates.EdgeNodePredicate(),
			)
			Expect(err).ShouldNot(HaveOccurred())

			node = corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: getNodeName(),
					Labels: map[string]string{
						"node-role.kubernetes.io/edge": "",
					},
				},
			}
			Expect(k8sClient.Create(context.Background(), &node)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(ObjectKey{Name: node.Name}))
		})

		AfterEach(func() {
			_ = k8sClient.Delete(context.Background(), &node)
		})

		It("should add a node to nodeSet", func() {
			n := px.nodeSet[node.Name]

			Expect(n.Name).To(Equal(node.Name))
			Expect(len(n.ServicePortMap)).To(Equal(0))
			Expect(len(n.EndpointMap)).To(Equal(0))

			Expect(len(keeper.nodeSet)).To(Equal(0))
		})

		It("should not change existing node in nodeSet", func() {
			n := px.nodeSet[node.Name]
			spn := ServicePortName{
				NamespacedName: ObjectKey{Name: "nginx", Namespace: defaultNamespace},
				Port:           80,
				Protocol:       corev1.ProtocolTCP,
			}
			sp := ServicePort{
				ClusterIP: "2.2.2.2",
				Port:      80,
				Protocol:  corev1.ProtocolTCP,
			}
			n.ServicePortMap[spn] = sp
			px.nodeSet[node.Name] = n

			node.Labels["purpose"] = "test"
			Expect(k8sClient.Update(context.Background(), &node)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(ObjectKey{Name: node.Name}))

			n = px.nodeSet[node.Name]
			Expect(n.ServicePortMap[spn]).To(Equal(sp))

			Expect(len(keeper.nodeSet)).To(Equal(0))
		})

		It("should delete node from nodeSet when node is deleted", func() {
			Expect(k8sClient.Delete(context.Background(), &node)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(ObjectKey{Name: node.Name}))

			_, exists := px.nodeSet[node.Name]
			Expect(exists).To(BeFalse())

			Expect(len(keeper.nodeSet)).To(Equal(0))
		})
	})

	Context("endpointslice", func() {
		var requests chan reconcile.Request

		BeforeEach(func() {
			var reconciler reconcile.Func
			reconciler, requests = testutil.WrapReconcileFunc(px.OnEndpointSliceUpdate)
			err := addController("proxy-endpointslice", mgr, reconciler, &discoveryv1.EndpointSlice{})
			Expect(err).ShouldNot(HaveOccurred())

			edgeNodeSet["node1"] = newEdgeNode("node1")
			edgeNodeSet["node2"] = newEdgeNode("node2")
			edgeNodeSet["node3"] = newEdgeNode("node3")
		})

		It("should synchronize service and endpoints info when an endpointslice is created or updated", func() {
			By("create service")
			timeoutSeconds := int32(3600)
			service := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getServiceName(),
					Namespace: defaultNamespace,
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"app": "nginx",
					},
					SessionAffinity: corev1.ServiceAffinityClientIP,
					SessionAffinityConfig: &corev1.SessionAffinityConfig{
						ClientIP: &corev1.ClientIPConfig{
							TimeoutSeconds: &timeoutSeconds,
						},
					},
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Name:     "health",
							Port:     8080,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			}

			err := k8sClient.Create(context.Background(), &service)
			Expect(err).ShouldNot(HaveOccurred())

			By("create endpointslice for service")
			endpointReady, endpointNotReady := true, false
			endpointSlice := discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      service.Name,
					Namespace: service.Namespace,
					Labels: map[string]string{
						LabelServiceName: service.Name,
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.40.20.181"},
						Topology: map[string]string{
							LabelHostname: "node1",
						},
					},
					{
						Addresses:  []string{"10.40.20.182"},
						Conditions: discoveryv1.EndpointConditions{Ready: &endpointReady},
						Topology: map[string]string{
							LabelHostname: "node2",
						},
					},
					{
						Addresses:  []string{"10.40.20.183"},
						Conditions: discoveryv1.EndpointConditions{Ready: &endpointNotReady},
						Topology: map[string]string{
							LabelHostname: "node3",
						},
					},
					{
						Addresses:  []string{"10.40.20.184"},
						Conditions: discoveryv1.EndpointConditions{Ready: &endpointNotReady},
						Topology: map[string]string{
							LabelHostname: "node4",
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{},
			}
			for _, port := range service.Spec.Ports {
				p := port
				endpointSlice.Ports = append(endpointSlice.Ports, discoveryv1.EndpointPort{
					Name:     &p.Name,
					Port:     &p.Port,
					Protocol: &p.Protocol,
				})
			}
			esKey := ObjectKey{Name: endpointSlice.Name, Namespace: endpointSlice.Namespace}
			Expect(k8sClient.Create(context.Background(), &endpointSlice)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check service info")
			serviceInfo, ok := serviceMap[ObjectKey{Name: service.Name, Namespace: service.Namespace}]

			Expect(ok).To(BeTrue())
			Expect(serviceInfo.ClusterIP).Should(Equal(service.Spec.ClusterIP))
			Expect(serviceInfo.SessionAffinity).Should(Equal(corev1.ServiceAffinityClientIP))
			Expect(serviceInfo.StickyMaxAgeSeconds).Should(Equal(timeoutSeconds))
			Expect(len(serviceInfo.EndpointMap)).Should(Equal(2))

			ess, ok := serviceInfo.EndpointMap[Port{Port: 80, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 80}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.182", Port: 80}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.183", Port: 80}))
			Expect(ess).ShouldNot(HaveKey(Endpoint{IP: "10.40.20.184", Port: 80}))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 8080, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 8080}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.182", Port: 8080}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.183", Port: 8080}))
			Expect(ess).ShouldNot(HaveKey(Endpoint{IP: "10.40.20.184", Port: 8080}))

			Expect(len(serviceInfo.EndpointToNodes)).To(Equal(6))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.181", Port: 80}, "node1"))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.181", Port: 8080}, "node1"))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.182", Port: 80}, "node2"))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.182", Port: 8080}, "node2"))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.183", Port: 80}, "node3"))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.183", Port: 8080}, "node3"))

			By("update endpoints in endpointslice")
			endpointSlice.Endpoints[2].Conditions.Ready = &endpointReady
			err = k8sClient.Update(context.Background(), &endpointSlice)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check service cache")
			serviceInfo, ok = serviceMap[ObjectKey{Name: service.Name, Namespace: service.Namespace}]
			Expect(ok).To(BeTrue())
			Expect(serviceInfo.ClusterIP).Should(Equal(service.Spec.ClusterIP))
			Expect(serviceInfo.SessionAffinity).Should(Equal(corev1.ServiceAffinityClientIP))
			Expect(serviceInfo.StickyMaxAgeSeconds).Should(Equal(timeoutSeconds))
			Expect(len(serviceInfo.EndpointMap)).Should(Equal(2))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 80, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 80}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.182", Port: 80}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.183", Port: 80}))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 8080, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 8080}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.182", Port: 8080}))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.183", Port: 8080}))

			Expect(len(serviceInfo.EndpointToNodes)).To(Equal(6))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.183", Port: 80}, "node3"))
			Expect(serviceInfo.EndpointToNodes).To(HaveKeyWithValue(Endpoint{IP: "10.40.20.183", Port: 8080}, "node3"))

			By("delete some endpoints in endpointslice")
			endpointSlice.Endpoints = endpointSlice.Endpoints[:1]
			Expect(k8sClient.Update(context.Background(), &endpointSlice)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check service cache")
			serviceInfo, ok = serviceMap[ObjectKey{Name: service.Name, Namespace: service.Namespace}]
			Expect(ok).To(BeTrue())
			Expect(len(serviceInfo.EndpointMap)).Should(Equal(2))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 80, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 80}))
			Expect(ess).ShouldNot(HaveKey(Endpoint{IP: "10.40.20.182", Port: 80}))
			Expect(ess).ShouldNot(HaveKey(Endpoint{IP: "10.40.20.183", Port: 80}))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 8080, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 8080}))
			Expect(ess).ShouldNot(HaveKey(Endpoint{IP: "10.40.20.182", Port: 8080}))
			Expect(ess).ShouldNot(HaveKey(Endpoint{IP: "10.40.20.183", Port: 8080}))

			Expect(len(serviceInfo.EndpointToNodes)).To(Equal(2))
			Expect(serviceInfo.EndpointToNodes).Should(HaveKeyWithValue(Endpoint{IP: "10.40.20.181", Port: 80}, "node1"))
			Expect(serviceInfo.EndpointToNodes).Should(HaveKeyWithValue(Endpoint{IP: "10.40.20.181", Port: 8080}, "node1"))

			By("remove port in endpointslice")
			endpointSlice.Ports = endpointSlice.Ports[:1]
			Expect(k8sClient.Update(context.Background(), &endpointSlice)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check service cache")
			serviceInfo, ok = serviceMap[ObjectKey{Name: service.Name, Namespace: service.Namespace}]
			Expect(ok).To(BeTrue())
			Expect(len(serviceInfo.EndpointMap)).Should(Equal(1))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 80, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeTrue())
			Expect(len(ess)).Should(Equal(1))
			Expect(ess).Should(HaveKey(Endpoint{IP: "10.40.20.181", Port: 80}))

			ess, ok = serviceInfo.EndpointMap[Port{Port: 8080, Protocol: corev1.ProtocolTCP}]
			Expect(ok).To(BeFalse())

			Expect(len(serviceInfo.EndpointToNodes)).Should(Equal(1))
			Expect(serviceInfo.EndpointToNodes).Should(HaveKeyWithValue(Endpoint{IP: "10.40.20.181", Port: 80}, "node1"))

			By("delete endpointslice")
			Expect(k8sClient.Delete(context.Background(), &endpointSlice)).NotTo(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check service info")
			serviceInfo, ok = serviceMap[ObjectKey{Name: service.Name, Namespace: service.Namespace}]
			Expect(ok).To(BeTrue())
			Expect(len(serviceInfo.EndpointMap)).Should(Equal(0))
			Expect(len(serviceInfo.EndpointToNodes)).To(Equal(0))
		})

		It("should synchronize service and endpoint's for node when endpointslice are updated", func() {
			By("create service")
			timeoutSeconds := int32(3600)
			service := corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      getServiceName(),
					Namespace: defaultNamespace,
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
					Selector: map[string]string{
						"app": "nginx",
					},
					SessionAffinity: corev1.ServiceAffinityClientIP,
					SessionAffinityConfig: &corev1.SessionAffinityConfig{
						ClientIP: &corev1.ClientIPConfig{
							TimeoutSeconds: &timeoutSeconds,
						},
					},
					Ports: []corev1.ServicePort{
						{
							Name:     "http",
							Port:     80,
							Protocol: corev1.ProtocolTCP,
						},
						{
							Name:     "health",
							Port:     8080,
							Protocol: corev1.ProtocolTCP,
						},
					},
				},
			}

			err := k8sClient.Create(context.Background(), &service)
			Expect(err).ShouldNot(HaveOccurred())

			By("create endpointslice for service")
			endpointSlice := discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Name:      service.Name,
					Namespace: service.Namespace,
					Labels: map[string]string{
						LabelServiceName: service.Name,
					},
				},
				AddressType: discoveryv1.AddressTypeIPv4,
				Endpoints: []discoveryv1.Endpoint{
					{
						Addresses: []string{"10.40.20.181"},
						Topology: map[string]string{
							LabelHostname: "node1",
						},
					},
					{
						Addresses: []string{"10.40.20.182"},
						Topology: map[string]string{
							LabelHostname: "node2",
						},
					},
				},
				Ports: []discoveryv1.EndpointPort{},
			}
			for _, port := range service.Spec.Ports {
				p := port
				endpointSlice.Ports = append(endpointSlice.Ports, discoveryv1.EndpointPort{
					Name:     &p.Name,
					Port:     &p.Port,
					Protocol: &p.Protocol,
				})
			}
			esKey := ObjectKey{Name: endpointSlice.Name, Namespace: endpointSlice.Namespace}
			keeper.nodeSet = make(EdgeNodeSet)
			Expect(k8sClient.Create(context.Background(), &endpointSlice)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check edge node services and endpoints")
			for _, port := range service.Spec.Ports {
				spn := ServicePortName{
					NamespacedName: ObjectKey{Name: service.Name, Namespace: service.Namespace},
					Port:           port.Port,
					Protocol:       port.Protocol,
				}

				for _, nodeName := range []string{"node1", "node2"} {
					node := edgeNodeSet[nodeName]
					Expect(len(node.ServicePortMap)).Should(Equal(2))
					Expect(len(node.EndpointMap)).Should(Equal(2))

					sp, ok := node.ServicePortMap[spn]
					Expect(ok).To(BeTrue())
					Expect(sp.Port).To(Equal(port.Port))
					Expect(sp.SessionAffinity).To(Equal(corev1.ServiceAffinityClientIP))
					Expect(sp.StickyMaxAgeSeconds).To(Equal(timeoutSeconds))

					for _, ep := range endpointSlice.Endpoints {
						if ep.Topology[LabelHostname] == nodeName {
							endpointSet := node.EndpointMap[spn]
							Expect(len(endpointSet)).Should(Equal(1))
							Expect(endpointSet).Should(HaveKey(Endpoint{
								IP:   ep.Addresses[0],
								Port: port.Port,
							}))
						}
					}

					Expect(keeper.nodeSet).Should(ContainElement(node))
				}
			}

			By("delete some endpoints in endpointslice")
			keeper.nodeSet = make(EdgeNodeSet)
			endpointSlice.Endpoints = endpointSlice.Endpoints[:1]
			Expect(k8sClient.Update(context.Background(), &endpointSlice)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check edge node2 services and endpoints")
			node2 := edgeNodeSet["node2"]
			Expect(len(node2.ServicePortMap)).Should(Equal(0))
			Expect(len(node2.EndpointMap)).Should(Equal(0))

			Expect(keeper.nodeSet).Should(ContainElement(node2))
			Expect(keeper.nodeSet).ShouldNot(HaveKey("node1"))

			By("remove port in endpointslice")
			keeper.nodeSet = make(EdgeNodeSet)
			endpointSlice.Ports = endpointSlice.Ports[:1]
			Expect(k8sClient.Update(context.Background(), &endpointSlice)).ShouldNot(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check node1's endpoints")
			node1 := edgeNodeSet["node1"]
			Expect(len(node1.ServicePortMap)).Should(Equal(1))
			Expect(len(node1.EndpointMap)).Should(Equal(1))
			Expect(keeper.nodeSet).Should(ContainElement(node1))
			Expect(keeper.nodeSet).ShouldNot(HaveKey("node2"))

			spn80 := ServicePortName{
				NamespacedName: ObjectKey{Name: service.Name, Namespace: service.Namespace},
				Port:           80,
				Protocol:       corev1.ProtocolTCP,
			}
			endpointSet := node1.EndpointMap[spn80]
			Expect(len(endpointSet)).Should(Equal(1))
			Expect(endpointSet).Should(HaveKey(Endpoint{
				IP:   "10.40.20.181",
				Port: 80,
			}))

			By("delete endpointslice")
			keeper.nodeSet = make(EdgeNodeSet)
			Expect(k8sClient.Delete(context.Background(), &endpointSlice)).NotTo(HaveOccurred())
			Eventually(requests, 2*time.Second).Should(ReceiveKey(esKey))

			By("check node1's endpoints")
			node1 = edgeNodeSet["node1"]
			Expect(len(node1.ServicePortMap)).Should(Equal(0))
			Expect(len(node1.EndpointMap)).Should(Equal(0))
		})
	})
})

var _ = Describe("Proxy's shouldSkipService", func() {
	px := &proxy{}

	It("should return true when service's clusterIP is None or blank", func() {
		svc := corev1.Service{
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: corev1.ClusterIPNone,
				Selector: map[string]string{
					"app": "nginx",
				},
			},
		}

		Expect(px.shouldSkipService(&svc)).To(BeTrue())

		svc.Spec.ClusterIP = ""
		Expect(px.shouldSkipService(&svc)).To(BeTrue())
	})

	It("should return true when service's selector is empty", func() {
		svc := corev1.Service{
			Spec: corev1.ServiceSpec{
				Type:      corev1.ServiceTypeClusterIP,
				ClusterIP: "10.0.0.181",
			},
		}

		Expect(px.shouldSkipService(&svc)).To(BeTrue())

		svc.Spec.Selector = map[string]string{}
		Expect(px.shouldSkipService(&svc)).To(BeTrue())
	})

	It("should return true when service's type is not ClusterIP", func() {
		svc := corev1.Service{
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeNodePort,
			},
		}

		Expect(px.shouldSkipService(&svc)).To(BeTrue())
	})
})
