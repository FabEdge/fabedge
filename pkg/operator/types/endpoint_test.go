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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
)

var _ = Describe("Endpoint", func() {
	It("should equal if all fields are equal", func() {
		e1 := types.Endpoint{
			ID:              "test",
			Name:            "edge2",
			PublicAddresses: []string{"192.168.0.1"},
			Subnets:         []string{"2.2.0.0/64"},
			NodeSubnets:     []string{"192.168.0.1"},
		}

		e2 := types.Endpoint{
			ID:              "test",
			Name:            "edge2",
			PublicAddresses: []string{"192.168.0.1"},
			Subnets:         []string{"2.2.0.0/64"},
			NodeSubnets:     []string{"192.168.0.1"},
		}

		Expect(e1.Equal(e2)).Should(BeTrue())
	})

	DescribeTable("isValid should return false",
		func(ep types.Endpoint) {
			Expect(ep.IsValid()).Should(BeFalse())
		},

		Entry("with invalid subnets", types.Endpoint{
			PublicAddresses: []string{"2.2.2.255"},
			Subnets:         []string{"2.2.0.0/33"},
			NodeSubnets:     []string{"2.2.2.255"},
		}),

		Entry("with invalid node subnets", types.Endpoint{
			PublicAddresses: []string{"2.2.2.2", "www.google.com"},
			Subnets:         []string{"2.2.0.0/26"},
			NodeSubnets:     []string{"2.2.0.0/33"},
		}),

		Entry("with empty ip", types.Endpoint{
			PublicAddresses: nil,
			Subnets:         []string{"2.2.0.0/26"},
			NodeSubnets:     []string{"2.2.2.1/26"},
		}),
		Entry("with empty subnets", types.Endpoint{
			PublicAddresses: []string{"2.2.2.2"},
			Subnets:         nil,
			NodeSubnets:     []string{"2.2.2.1/26"},
		}),
		Entry("with empty ip", types.Endpoint{
			PublicAddresses: []string{"2.2.2.2"},
			Subnets:         []string{"2.2.0.0/26"},
			NodeSubnets:     nil,
		}),
	)
})

var _ = Describe("GenerateNewEndpointFunc", func() {
	newEndpoint := types.GenerateNewEndpointFunc("C=CN, O=StrongSwan, CN={node}", nodeutil.GetPodCIDRsFromAnnotation)
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

	It("should replace {node} in id format", func() {
		Expect(endpoint.ID).Should(Equal("C=CN, O=StrongSwan, CN=edge1"))
	})

	It("should extract subnets from annotations", func() {
		Expect(endpoint.Subnets).Should(ContainElement("2.2.0.1/26"))
		Expect(endpoint.Subnets).Should(ContainElement("2.2.0.128/26"))
	})

	It("should extract ip from node.status.address", func() {
		Expect(endpoint.PublicAddresses).Should(ConsistOf("192.168.1.1"))
	})
})
