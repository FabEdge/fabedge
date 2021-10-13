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

	"github.com/jjeffery/stringset"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/netconf"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
)

var _ = Describe("ConfigHandler", func() {
	var (
		namespace       = "default"
		agentConfigName string

		node corev1.Node

		connectorEndpoint, edge2Endpoint types.Endpoint
		testCommunity                    types.Community

		newEndpoint = types.GenerateNewEndpointFunc("C=CN, O=StrongSwan, CN={node}", nodeutil.GetPodCIDRsFromAnnotation)
		newNode     = newNodePodCIDRsInAnnotations

		handler *configHandler
		store   storepkg.Interface
	)

	BeforeEach(func() {
		store = storepkg.NewStore()
		handler = &configHandler{
			namespace:            namespace,
			client:               k8sClient,
			store:                store,
			getConnectorEndpoint: getConnectorEndpoint,
			log:                  klogr.New().WithName("configHandler"),
		}

		nodeName := getNodeName()
		connectorEndpoint = getConnectorEndpoint()
		edge2Endpoint = types.Endpoint{
			ID:              "C=CN, O=StrongSwan, CN=edge2",
			Name:            "edge2",
			PublicAddresses: []string{"10.20.8.141"},
			Subnets:         []string{"2.2.1.65/26"},
			NodeSubnets:     []string{"10.20.8.141"},
			Type:            netconf.EdgeNode,
		}
		testCommunity = types.Community{
			Name:    "test",
			Members: stringset.New(edge2Endpoint.Name, nodeName),
		}

		agentConfigName = getAgentConfigMapName(nodeName)
		node = newNode(nodeName, "10.40.20.181", "2.2.1.128/26")

		store.SaveEndpoint(edge2Endpoint)
		store.SaveEndpoint(newEndpoint(node))
		store.SaveCommunity(testCommunity)

		Expect(handler.Do(context.TODO(), node)).To(Succeed())
	})

	It("Do should create agent configmap when it is not created yet", func() {
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

		expectedConf := netconf.NetworkConf{
			TunnelEndpoint: newEndpoint(node).ConvertToTunnelEndpoint(),
			Peers: []netconf.TunnelEndpoint{
				connectorEndpoint.ConvertToTunnelEndpoint(),
				edge2Endpoint.ConvertToTunnelEndpoint(),
			},
		}
		Expect(conf).Should(Equal(expectedConf))
		Expect(conf.Peers[0].Type).Should(Equal(netconf.Connector))
		Expect(conf.Peers[1].Type).Should(Equal(netconf.EdgeNode))
	})

	It("Do should update agent configmap when any endpoint changed", func() {
		By("changing edge2 ip address")
		edge2PublicAddresses := []string{"10.20.8.142"}
		edge2Endpoint.PublicAddresses = edge2PublicAddresses
		store.SaveEndpoint(edge2Endpoint)

		By("assign an IP address to node")
		node.Status.Addresses = []corev1.NodeAddress{
			{
				Type:    corev1.NodeInternalIP,
				Address: "10.40.20.182",
			},
		}
		store.SaveEndpoint(newEndpoint(node))

		By("re-executing Do method")
		Expect(handler.Do(context.TODO(), node)).To(Succeed())

		var cm corev1.ConfigMap
		err := k8sClient.Get(context.Background(), ObjectKey{Name: agentConfigName, Namespace: namespace}, &cm)
		Expect(err).ShouldNot(HaveOccurred())

		configData, ok := cm.Data[agentConfigTunnelFileName]
		Expect(ok).Should(BeTrue())

		var conf netconf.NetworkConf
		Expect(yaml.Unmarshal([]byte(configData), &conf)).ShouldNot(HaveOccurred())

		expectedConf := netconf.NetworkConf{
			TunnelEndpoint: newEndpoint(node).ConvertToTunnelEndpoint(),
			Peers: []netconf.TunnelEndpoint{
				connectorEndpoint.ConvertToTunnelEndpoint(),
				edge2Endpoint.ConvertToTunnelEndpoint(),
			},
		}
		Expect(conf).Should(Equal(expectedConf))
		Expect(conf.Peers[1].PublicAddresses).Should(Equal(edge2PublicAddresses))
	})

	It("Undo should delete configmap created by Do method", func() {
		Expect(handler.Undo(context.TODO(), node.Name)).To(Succeed())

		var cm corev1.ConfigMap
		err := k8sClient.Get(context.Background(), ObjectKey{Name: agentConfigName, Namespace: namespace}, &cm)
		Expect(errors.IsNotFound(err)).Should(BeTrue())
	})
})

func getConnectorEndpoint() types.Endpoint {
	return types.Endpoint{
		ID:              "C=CN, O=StrongSwan, CN=cloud-connector",
		Name:            "cloud-connector",
		PublicAddresses: []string{"192.168.1.1"},
		Subnets:         []string{"2.2.1.1/26"},
		NodeSubnets:     []string{"192.168.1.0/24"},
		Type:            netconf.Connector,
	}
}
