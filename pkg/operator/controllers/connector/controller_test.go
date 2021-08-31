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
)

var _ = Describe("Controller", func() {
	var (
		edge1Endpoint       types.Endpoint
		edge2Endpoint       types.Endpoint
		connectorEndpoint   types.Endpoint
		store               storepkg.Interface
		ctx                 context.Context
		cancel              context.CancelFunc
		namespace           string
		connectorConfigName string
		interval            time.Duration
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		namespace = "default"
		connectorConfigName = "cloud-connector-config"
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
		connectorEndpoint = getConnectorEndpoint()

		store = storepkg.NewStore()
		store.SaveEndpoint(edge1Endpoint)
		store.SaveEndpoint(edge2Endpoint)

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())
		err = AddToManager(Config{
			Manager:              mgr,
			Store:                store,
			GetConnectorEndpoint: getConnectorEndpoint,

			ConnectorConfigName: connectorConfigName,
			Namespace:           namespace,

			Interval: interval,
		})
		Expect(err).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()
	})

	AfterEach(func() {
		cancel()
	})

	It("should synchronize connector configmap according to endpoints in store", func() {
		time.Sleep(interval + time.Second)

		key := client.ObjectKey{
			Name:      connectorConfigName,
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
		Expect(conf.TunnelEndpoint).To(Equal(connectorEndpoint.ConvertToTunnelEndpoint()))
		Expect(conf.Peers).To(ContainElement(edge1Endpoint.ConvertToTunnelEndpoint()))
		Expect(conf.Peers).To(ContainElement(edge2Endpoint.ConvertToTunnelEndpoint()))

		By("remove edge2 endpoint")
		store.DeleteEndpoint(edge2Endpoint.Name)

		time.Sleep(2 * interval)

		cm = corev1.ConfigMap{}
		Expect(k8sClient.Get(context.Background(), key, &cm)).ShouldNot(HaveOccurred())

		conf = getNetworkConf()
		Expect(conf.TunnelEndpoint).To(Equal(connectorEndpoint.ConvertToTunnelEndpoint()))
		Expect(conf.Peers).To(ContainElement(edge1Endpoint.ConvertToTunnelEndpoint()))

		Expect(conf.Peers).NotTo(ContainElement(edge2Endpoint.ConvertToTunnelEndpoint()))
	})
})

func getConnectorEndpoint() types.Endpoint {
	return types.Endpoint{
		ID:          "C=CN, O=StrongSwan, CN=cloud-connector",
		Name:        "cloud-connector",
		IP:          "192.168.1.1",
		Subnets:     []string{"2.2.1.1/26"},
		NodeSubnets: []string{"192.168.1.0/24"},
	}
}
