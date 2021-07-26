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
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
)

var _ = Describe("loadBalanceConfigKeeper", func() {
	var (
		keeper    *loadBalanceConfigKeeper
		ctx       context.Context
		cancel    context.CancelFunc
		namespace string
		interval  time.Duration
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		namespace = "default"
		interval = 500 * time.Millisecond

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())

		keeper = &loadBalanceConfigKeeper{
			interval:      interval,
			namespace:     namespace,
			nodeSet:       make(EdgeNodeSet),
			ipvsScheduler: "rr",

			client: k8sClient,
			log:    mgr.GetLogger(),
		}
		Expect(mgr.Add(manager.RunnableFunc(keeper.Start))).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()
	})

	AfterEach(func() {
		cancel()
	})

	It("should write load balance rules to node's agent config", func() {
		edge1 := newEdgeNode("edge1")

		By("create agent configmap")
		configName := getAgentConfigMapName(edge1.Name)
		agentConfig := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: namespace,
				Labels: map[string]string{
					constants.KeyFabedgeAPP: constants.AppAgent,
					constants.KeyCreatedBy:  constants.AppOperator,
				},
			},
			Data: map[string]string{
				"tunnels.yaml":                 "",
				agentConfigLoadBalanceFileName: "",
			},
		}
		configKey := ObjectKey{Name: agentConfig.Name, Namespace: agentConfig.Namespace}
		Expect(k8sClient.Create(context.Background(), &agentConfig)).ShouldNot(HaveOccurred())

		By("add edge node")
		spn := ServicePortName{
			NamespacedName: ObjectKey{Name: "nginx", Namespace: "default"},
			Port:           80,
			Protocol:       corev1.ProtocolTCP,
		}
		edge1.ServicePortMap[spn] = ServicePort{
			ClusterIP:       "2.2.2.2",
			Port:            80,
			Protocol:        corev1.ProtocolTCP,
			SessionAffinity: corev1.ServiceAffinityNone,
		}
		endpointSet := make(EndpointSet)
		endpointSet.Add(Endpoint{IP: "10.40.20.181", Port: 80})
		endpointSet.Add(Endpoint{IP: "10.40.20.182", Port: 80})
		edge1.EndpointMap[spn] = endpointSet

		keeper.AddNode(edge1)
		Expect(len(keeper.nodeSet)).To(Equal(1))

		time.Sleep(2 * interval)

		Expect(k8sClient.Get(context.Background(), configKey, &agentConfig)).ShouldNot(HaveOccurred())
		Expect(len(keeper.nodeSet)).To(Equal(0))

		var servers netconf.VirtualServers
		configData := agentConfig.Data[agentConfigLoadBalanceFileName]
		Expect(yaml.Unmarshal([]byte(configData), &servers)).ShouldNot(HaveOccurred())
		Expect(len(servers)).To(Equal(1))

		server := servers[0]
		Expect(server.IP).To(Equal("2.2.2.2"))
		Expect(server.Port).To(Equal(int32(80)))
		Expect(server.Scheduler).To(Equal(keeper.ipvsScheduler))
		Expect(server.Protocol).To(Equal(corev1.ProtocolTCP))
		Expect(server.StickyMaxAgeSeconds).To(Equal(int32(0)))
		Expect(server.SessionAffinity).To(Equal(corev1.ServiceAffinityNone))
		Expect(len(server.RealServers)).To(Equal(2))
		Expect(server.RealServers).To(ContainElement(netconf.RealServer{
			IP:   "10.40.20.181",
			Port: 80,
		}))
		Expect(server.RealServers).To(ContainElement(netconf.RealServer{
			IP:   "10.40.20.182",
			Port: 80,
		}))

		By("remove services of edge node")
		edge1 = newEdgeNode(edge1.Name)
		keeper.AddNode(edge1)

		time.Sleep(2 * interval)

		Expect(k8sClient.Get(context.Background(), configKey, &agentConfig)).ShouldNot(HaveOccurred())
		configData = agentConfig.Data[agentConfigLoadBalanceFileName]
		Expect(yaml.Unmarshal([]byte(configData), &servers)).ShouldNot(HaveOccurred())
		Expect(len(servers)).To(Equal(0))
	})
})
