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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("allocatablePodCIDRsHandler", func() {
	var (
		handler *allocatablePodCIDRsHandler
		newNode = newNodePodCIDRsInAnnotations
	)

	BeforeEach(func() {
		store := storepkg.NewStore()
		alloc, _ := allocator.New("2.2.0.0/16", 26)
		allocV6, _ := allocator.New("fd85:ee78:d8a6:8607::1:0000/112", 122)

		getEndpointName, _, newEndpoint := types.NewEndpointFuncs("cluster", "C=CN, O=fabedge.io, CN={node}", nodeutil.GetPodCIDRsFromAnnotation)
		handler = &allocatablePodCIDRsHandler{
			store:           store,
			allocators:      []allocator.Interface{alloc, allocV6},
			getEndpointName: getEndpointName,
			newEndpoint:     newEndpoint,
			client:          k8sClient,
			log:             klogr.New().WithName("podCIDRsHandler"),
		}
	})

	AfterEach(func() {
		Expect(testutil.PurgeAllNodes(k8sClient)).Should(Succeed())
	})

	Context("Do method", func() {
		It("should allocate a subnet to a node if this node has no subnet", func() {
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
			Expect(ep.Subnets).To(ContainElements(podCIDRs))

			nameSet := handler.store.GetLocalEndpointNames()
			Expect(nameSet.Has(epName)).Should(BeTrue())

			for _, podCIDR := range podCIDRs {
				_, ipNet, err := net.ParseCIDR(podCIDR)
				Expect(err).Should(BeNil())

				version := netutil.IPVersion(ipNet.IP)
				isAllocated := false
				for _, allocator := range handler.allocators {
					if allocator.Version() == version {
						isAllocated = allocator.IsAllocated(*ipNet)
					}
				}
				Expect(isAllocated).To(BeTrue())
			}
		})

		It("should allocate a subnet to an edge node if this node's subnet is invalid", func() {
			nodeName := getNodeName()
			node := newNode(nodeName, "10.40.20.181", "2.2.2.257/26,fd85:ee78:d8a6:8607::1:0100/122")
			Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())

			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())

			epName := handler.getEndpointName(nodeName)
			ep, ok := handler.store.GetEndpoint(epName)
			Expect(ok).To(BeTrue())

			podCIDRs := nodeutil.GetPodCIDRsFromAnnotation(node)
			Expect(ep.Subnets).To(ContainElements(podCIDRs))

			for _, podCIDR := range podCIDRs {
				_, ipNet, err := net.ParseCIDR(podCIDR)
				Expect(err).Should(BeNil())

				version := netutil.IPVersion(ipNet.IP)
				isAllocated := false
				for _, allocator := range handler.allocators {
					if allocator.Version() == version {
						isAllocated = allocator.IsAllocated(*ipNet)
					}
				}
				Expect(isAllocated).To(BeTrue())
			}
		})

		It("should reallocate a subnet to an edge node if this node's subnet is out of expected range", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "2.3.2.1/26,fd89:ee78:d8a6:8607::1:0000/122")
			Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())

			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())

			epName := handler.getEndpointName(nodeName)
			ep, ok := handler.store.GetEndpoint(epName)
			Expect(ok).To(BeTrue())

			podCIDRs := nodeutil.GetPodCIDRsFromAnnotation(node)
			Expect(ep.Subnets).To(ContainElements(podCIDRs))

			for _, podCIDR := range podCIDRs {
				_, ipNet, err := net.ParseCIDR(podCIDR)
				Expect(err).Should(BeNil())

				version := netutil.IPVersion(ipNet.IP)
				isAllocated := false
				for _, allocator := range handler.allocators {
					if allocator.Version() == version {
						isAllocated = allocator.IsAllocated(*ipNet)
					}
				}
				Expect(isAllocated).To(BeTrue())
			}
		})

	})

	Context("Undo method", func() {
		It("can reclaim subnets allocated to an edge node", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "")
			Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())
			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			epName := handler.getEndpointName(nodeName)
			ep, ok := handler.store.GetEndpoint(epName)
			Expect(ok).Should(BeTrue())

			podCIDRs := nodeutil.GetPodCIDRsFromAnnotation(node)
			Expect(ep.Subnets).To(ContainElements(podCIDRs))

			Expect(handler.Undo(context.TODO(), nodeName)).Should(Succeed())

			_, ok = handler.store.GetEndpoint(epName)
			Expect(ok).Should(BeFalse())

			for _, podCIDR := range podCIDRs {
				_, ipNet, err := net.ParseCIDR(podCIDR)
				Expect(err).Should(BeNil())

				version := netutil.IPVersion(ipNet.IP)
				isAllocated := false
				for _, allocator := range handler.allocators {
					if allocator.Version() == version {
						isAllocated = allocator.IsAllocated(*ipNet)
					}
				}
				Expect(isAllocated).To(BeFalse())
			}
		})
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
