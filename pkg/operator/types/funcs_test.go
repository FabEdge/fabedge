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

package types_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
)

var _ = Describe("EndpointFuncs", func() {
	_, _, newEndpoint := types.NewEndpointFuncs("cluster", "C=CN, O=StrongSwan, CN={node}", nodeutil.GetPodCIDRsFromAnnotation)

	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "edge1",
			Annotations: map[string]string{
				constants.KeyPodSubnets: "2.2.0.1/26,2.2.0.128/26",
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
	endpoint := newEndpoint(node)

	It("should mark endpoint as EdgeNode", func() {
		Expect(endpoint.Type).Should(Equal(apis.EdgeNode))
	})

	It("should replace {node} in id format", func() {
		Expect(endpoint.ID).Should(Equal("C=CN, O=StrongSwan, CN=cluster.edge1"))
	})

	It("should extract subnets from annotations", func() {
		Expect(endpoint.Subnets).Should(ContainElement("2.2.0.1/26"))
		Expect(endpoint.Subnets).Should(ContainElement("2.2.0.128/26"))
	})

	It("should read node subnets from node.status.address", func() {
		Expect(endpoint.NodeSubnets).Should(ConsistOf("192.168.1.1"))
	})

	It("should ues internal IP as public addresses if no public addresses in annotation", func() {
		Expect(endpoint.NodeSubnets).Should(ConsistOf("192.168.1.1"))
		Expect(endpoint.PublicAddresses).Should(ConsistOf("192.168.1.1"))
	})

	It("should read public addresses from annotation if it exists", func() {
		node = corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "edge1",
				Annotations: map[string]string{
					constants.KeyPodSubnets:          "2.2.0.1/26,2.2.0.128/26",
					constants.KeyNodePublicAddresses: "www.example.com,10.0.0.1",
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

		endpoint = newEndpoint(node)

		Expect(endpoint.PublicAddresses).Should(ConsistOf("www.example.com", "10.0.0.1"))
	})
})
