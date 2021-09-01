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

package connector

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Controller", func() {
	var (
		edge1Endpoint types.Endpoint
		edge2Endpoint types.Endpoint
		store         storepkg.Interface
		ctx           context.Context
		cancel        context.CancelFunc
		namespace     string
		interval      time.Duration
		config        Config
		node1, edge1  corev1.Node

		getConnectorEndpoint types.EndpointGetter
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		namespace = "default"
		interval = 2 * time.Second

		edge1Endpoint = types.Endpoint{
			ID:      "edge1",
			Name:    "edge1",
			IP:      "10.20.40.181",
			Subnets: []string{"2.2.0.1/26"},
		}
		edge2Endpoint = types.Endpoint{
			ID:      "edge2",
			Name:    "edge2",
			IP:      "10.20.40.182",
			Subnets: []string{"2.2.0.65/26"},
		}

		store = storepkg.NewStore()
		store.SaveEndpoint(edge1Endpoint)
		store.SaveEndpoint(edge2Endpoint)

		node1 = newNormalNode("192.168.1.2", "10.10.10.64/26")
		edge1 = newEdgeNode("10.20.40.183", "2.2.0.65/26")
		Expect(k8sClient.Create(context.Background(), &node1)).To(Succeed())
		Expect(k8sClient.Create(context.Background(), &edge1)).To(Succeed())

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()

		mgr.GetCache().WaitForCacheSync(ctx)

		config = Config{
			Manager: mgr,
			Store:   store,

			ConnectorIP:         "192.168.1.1",
			ConnectorID:         "cloud-connector",
			ConnectorName:       "cloud-connector",
			ProvidedSubnets:     []string{"10.10.10.1/26"},
			CollectPodCIDRs:     true,
			ConnectorConfigName: "connector-config",
			Namespace:           namespace,

			Interval: interval,
		}
		getConnectorEndpoint, err = AddToManager(config)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterEach(func() {
		cancel()
	})

	It("build connector endpoint when node events come", func() {
		cep := getConnectorEndpoint()

		Expect(cep.IP).Should(Equal(config.ConnectorIP))
		Expect(cep.ID).Should(Equal(config.ConnectorID))
		Expect(cep.Name).Should(Equal(config.ConnectorName))
		Expect(cep.Subnets).Should(ConsistOf(config.ProvidedSubnets[0], nodeutil.GetPodCIDRs(node1)[0]))
		Expect(cep.NodeSubnets).Should(ConsistOf(nodeutil.GetIP(node1)))

		node2 := newNormalNode("192.168.1.3", "10.10.10.128/26")
		Expect(k8sClient.Create(context.Background(), &node2)).To(Succeed())
		time.Sleep(time.Second)

		cep = getConnectorEndpoint()
		Expect(cep.IP).Should(Equal(config.ConnectorIP))
		Expect(cep.ID).Should(Equal(config.ConnectorID))
		Expect(cep.Name).Should(Equal(config.ConnectorName))
		Expect(cep.Subnets).Should(ConsistOf(config.ProvidedSubnets[0], nodeutil.GetPodCIDRs(node1)[0], nodeutil.GetPodCIDRs(node2)[0]))
		Expect(cep.NodeSubnets).Should(ConsistOf(nodeutil.GetIP(node1), nodeutil.GetIP(node2)))

		Expect(k8sClient.Delete(context.Background(), &node2)).To(Succeed())
		time.Sleep(time.Second)

		cep = getConnectorEndpoint()
		Expect(cep.Subnets).ShouldNot(ContainElements(nodeutil.GetPodCIDRs(node2)))
		Expect(cep.NodeSubnets).ShouldNot(ContainElements(nodeutil.GetIP(node2)))
	})

	It("should synchronize connector configmap according to endpoints in store", func() {
		time.Sleep(interval + time.Second)

		key := client.ObjectKey{
			Name:      config.ConnectorConfigName,
			Namespace: namespace,
		}
		var cm corev1.ConfigMap
		Expect(k8sClient.Get(context.Background(), key, &cm)).ShouldNot(HaveOccurred())

		getNetworkConf := func() netconf.NetworkConf {
			conf := netconf.NetworkConf{}
			err := yaml.Unmarshal([]byte(cm.Data[constants.ConnectorConfigFileName]), &conf)
			Expect(err).ShouldNot(HaveOccurred())

			return conf
		}

		conf := getNetworkConf()
		cep := getConnectorEndpoint()
		Expect(conf.TunnelEndpoint).To(Equal(cep.ConvertToTunnelEndpoint()))
		Expect(conf.Peers).To(ContainElement(edge1Endpoint.ConvertToTunnelEndpoint()))
		Expect(conf.Peers).To(ContainElement(edge2Endpoint.ConvertToTunnelEndpoint()))

		By("remove edge2 endpoint")
		store.DeleteEndpoint(edge2Endpoint.Name)

		time.Sleep(2 * interval)

		cm = corev1.ConfigMap{}
		Expect(k8sClient.Get(context.Background(), key, &cm)).ShouldNot(HaveOccurred())

		conf = getNetworkConf()
		Expect(conf.TunnelEndpoint).To(Equal(getConnectorEndpoint().ConvertToTunnelEndpoint()))
		Expect(conf.Peers).To(ContainElement(edge1Endpoint.ConvertToTunnelEndpoint()))

		Expect(conf.Peers).NotTo(ContainElement(edge2Endpoint.ConvertToTunnelEndpoint()))
	})
})

func newNormalNode(ip, subnets string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: getNodeName(),
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  subnets,
			PodCIDRs: strings.Split(subnets, ","),
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: ip,
				},
			},
		},
	}
}

func newEdgeNode(ip, subnets string) corev1.Node {
	node := newNormalNode(ip, subnets)
	node.Name = getEdgeName()
	node.Labels = map[string]string{
		"node-role.kubernetes.io/edge": "",
	}

	return node
}
