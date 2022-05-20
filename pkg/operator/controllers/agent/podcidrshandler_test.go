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
	"net"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("allocatablePodCIDRsHandler", func() {
	var (
		newNode    = newNodePodCIDRsInAnnotations
		newHandler = func(netCIDRs []string, subnetMaskSizes []int) (*allocatablePodCIDRsHandler, error) {
			var allocators []allocator.Interface
			for i, netCIDR := range netCIDRs {
				alloc, err := allocator.New(netCIDR, subnetMaskSizes[i])
				if err != nil {
					return nil, err
				}

				allocators = append(allocators, alloc)
			}
			getEndpointName, _, newEndpoint := types.NewEndpointFuncs("cluster", "C=CN, O=fabedge.io, CN={node}", nodeutil.GetPodCIDRsFromAnnotation)

			store := storepkg.NewStore()
			handler := &allocatablePodCIDRsHandler{
				store:           store,
				allocators:      allocators,
				getEndpointName: getEndpointName,
				newEndpoint:     newEndpoint,
				client:          k8sClient,
				log:             klogr.New().WithName("podCIDRsHandler"),
			}
			return handler, nil
		}
	)

	AfterEach(func() {
		Expect(testutil.PurgeAllNodes(k8sClient)).Should(Succeed())
	})

	Context("Do method", func() {
		DescribeTable("should allocate a subnet to a node if this node has no subnet",
			func(netCIDRs []string, subnetMaskSizes []int) {
				handler, err := newHandler(netCIDRs, subnetMaskSizes)
				Expect(err).Should(BeNil())

				nodeName := getNodeName()
				node := newNode(nodeName, "10.40.20.181", "")

				Expect(k8sClient.Create(context.TODO(), &node)).Should(Succeed())
				Expect(handler.Do(context.TODO(), node)).Should(Succeed())

				Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())
				Expect(node.Annotations[constants.KeyPodSubnets]).ShouldNot(BeEmpty())

				epName := handler.getEndpointName(nodeName)
				ep, ok := handler.store.GetEndpoint(epName)
				Expect(ok).To(BeTrue())

				podCIDRs := nodeutil.GetPodCIDRsFromAnnotation(node)
				for i, cidr := range podCIDRs {
					Expect(ep.Subnets[i]).To(Equal(cidr))

					_, ipNet, err := net.ParseCIDR(cidr)
					Expect(err).Should(BeNil())
					Expect(handler.allocators[i].IsAllocated(*ipNet)).To(BeTrue())
				}

				nameSet := handler.store.GetLocalEndpointNames()
				Expect(nameSet.Has(epName)).Should(BeTrue())
			},
			Entry("DualStack", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}),
			Entry("IPv4 Only", []string{"2.2.0.0/16"}, []int{26}),
			Entry("IPv6 Only", []string{"fd85:ee78:d8a6:8607::1:0000/112"}, []int{122}),
		)

		DescribeTable("should allocate a subnet to a edge node if this node's subnet is invalid",
			func(netCIDRs []string, subnetMaskSizes []int, oldNodePodCIDRs string) {
				handler, err := newHandler(netCIDRs, subnetMaskSizes)
				Expect(err).Should(BeNil())

				nodeName := getNodeName()
				node := newNode(nodeName, "10.40.20.181", oldNodePodCIDRs)
				Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())
				// record old node pod CIDRs
				for _, cidr := range strings.Split(oldNodePodCIDRs, ",") {
					if cidr == "" {
						continue
					}
					_, ipNet, err := net.ParseCIDR(cidr)
					Expect(err).To(Succeed())

					for _, alloc := range handler.allocators {
						if alloc.Contains(*ipNet) {
							Expect(alloc.Record(*ipNet)).To(Succeed())
						}
					}
				}

				Expect(handler.Do(context.TODO(), node)).Should(Succeed())
				Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())

				epName := handler.getEndpointName(nodeName)
				ep, ok := handler.store.GetEndpoint(epName)
				Expect(ok).To(BeTrue())

				podCIDRs := nodeutil.GetPodCIDRsFromAnnotation(node)
				for i, cidr := range podCIDRs {
					Expect(ep.Subnets[i]).To(Equal(cidr))

					_, ipNet, err := net.ParseCIDR(cidr)
					Expect(err).Should(BeNil())
					Expect(handler.allocators[i].IsAllocated(*ipNet)).To(BeTrue())
				}
				Expect(node.Annotations[constants.KeyPodSubnets]).NotTo(Equal(oldNodePodCIDRs))

				nameSet := handler.store.GetLocalEndpointNames()
				Expect(nameSet.Has(epName)).Should(BeTrue())

				for _, cidr := range strings.Split(oldNodePodCIDRs, ",") {
					if cidr == "" {
						continue
					}
					_, ipNet, err := net.ParseCIDR(cidr)
					Expect(err).To(Succeed())

					for _, alloc := range handler.allocators {
						if alloc.Contains(*ipNet) {
							Expect(alloc.IsAllocated(*ipNet)).To(BeFalse())
						}
					}
				}
			},
			Entry("[DualStack] No PodCIDRs", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}, ""),
			Entry("[DualStack] IPv4 PodCIDR is out of range", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}, "2.3.0.1/26,fd85:ee78:d8a6:8607::1:0001/122"),
			Entry("[DualStack] IPv6 PodCIDR is out of range", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}, "2.2.0.1/26,fd85:ee79:d8a6:8607::1:0001/122"),
			Entry("[DualStack] Only IPv4 PodCIDR allocated", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}, "2.2.0.1/26"),
			Entry("[DualStack] Only IPv6 PodCIDR allocated", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}, "fd85:ee79:d8a6:8607::1:0001/122"),
			Entry("[IPv4 Only] No PodCIDRs", []string{"2.2.0.0/16"}, []int{26}, ""),
			Entry("[IPv4 Only] IPv4 PodCIDR is out of range", []string{"2.2.0.0/16"}, []int{26}, "2.3.0.1/26"),
			Entry("[IPv4 Only] IPv6 PodCIDR is allocated", []string{"2.2.0.0/16"}, []int{26}, "fd85:ee78:d8a6:8607::1:0001/122"),
			Entry("[IPv6 Only] No PodCIDRs", []string{"fd85:ee78:d8a6:8607::1:0000/112"}, []int{122}, ""),
			Entry("[IPv6 Only] IPv4 PodCIDR is allocated", []string{"fd85:ee78:d8a6:8607::1:0000/112"}, []int{122}, "2.2.0.1/26"),
			Entry("[IPv6 Only] IPv6 PodCIDR is out of range", []string{"fd85:ee78:d8a6:8607::1:0000/112"}, []int{122}, "fd85:ee79:d8a6:8607::1:0001/122"),
		)
	})

	Context("Undo method", func() {
		DescribeTable("can reclaim subnets allocated to a edge node",
			func(netCIDRs []string, subnetMaskSizes []int) {
				handler, err := newHandler(netCIDRs, subnetMaskSizes)
				Expect(err).Should(BeNil())

				nodeName := getNodeName()
				node := newNode(nodeName, "10.40.20.181", "")
				Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())
				Expect(handler.Do(context.TODO(), node)).Should(Succeed())

				Expect(k8sClient.Get(context.TODO(), ObjectKey{Name: nodeName}, &node)).To(Succeed())
				podCIDRs := nodeutil.GetPodCIDRsFromAnnotation(node)

				Expect(handler.Undo(context.TODO(), nodeName)).Should(Succeed())

				epName := handler.getEndpointName(nodeName)
				_, ok := handler.store.GetEndpoint(epName)
				Expect(ok).Should(BeFalse())

				for i, cidr := range podCIDRs {
					_, ipNet, err := net.ParseCIDR(cidr)
					Expect(err).Should(Succeed())

					Expect(handler.allocators[i].IsAllocated(*ipNet)).Should(BeFalse())
				}
			},
			Entry("DualStack", []string{"2.2.0.0/16", "fd85:ee78:d8a6:8607::1:0000/112"}, []int{26, 122}),
			Entry("IPv4 Only", []string{"2.2.0.0/16"}, []int{26}),
			Entry("IPv6 Only", []string{"fd85:ee78:d8a6:8607::1:0000/112"}, []int{122}),
		)
	})
})

var _ = Describe("rawPodCIDRsHandler", func() {
	var (
		handler     *rawPodCIDRsHandler
		newNode     = newNodeUsingRawPodCIDRs
		getNodeName = testutil.GenerateGetNameFunc("edge")
	)

	BeforeEach(func() {
		store := storepkg.NewStore()
		getName, _, newEndpoint := types.NewEndpointFuncs("cluster", "C=CN, O=fabedge.io, CN={node}", nodeutil.GetPodCIDRs)

		handler = &rawPodCIDRsHandler{
			store:           store,
			getEndpointName: getName,
			newEndpoint:     newEndpoint,
		}
	})

	AfterEach(func() {
		Expect(testutil.PurgeAllNodes(k8sClient)).Should(Succeed())
	})

	It("build endpoint using spec.PodCIDRs", func() {
		nodeName := getNodeName()
		node := newNode(nodeName, "10.40.20.181", "2.2.2.2/26")

		Expect(handler.Do(context.TODO(), node)).Should(Succeed())

		epName := handler.getEndpointName(nodeName)
		ep, ok := handler.store.GetEndpoint(epName)
		Expect(ok).To(BeTrue())
		Expect(len(ep.Subnets)).Should(Equal(1))
		Expect(ep.Subnets).To(Equal(node.Spec.PodCIDRs))
	})
})
