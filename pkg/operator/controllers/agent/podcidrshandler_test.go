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
		alloc, _ := allocator.New("2.2.0.0/16")

		handler = &allocatablePodCIDRsHandler{
			store:       store,
			allocator:   alloc,
			newEndpoint: types.GenerateNewEndpointFunc("C=CN, O=fabedge.io, CN={node}", nodeutil.GetPodCIDRsFromAnnotation),
			client:      k8sClient,
			log:         klogr.New().WithName("podCIDRsHandler"),
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

			ep, ok := handler.store.GetEndpoint(nodeName)
			Expect(ok).To(BeTrue())
			Expect(ep.Subnets[0]).To(Equal(node.Annotations[constants.KeyPodSubnets]))

			_, ipNet, err := net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).Should(BeNil())
			Expect(handler.allocator.IsAllocated(*ipNet))
		})

		It("should allocate a subnet to a edge node if this node's subnet is invalid", func() {
			nodeName := getNodeName()
			node := newNode(nodeName, "10.40.20.181", "2.2.2.257/26")
			Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())

			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())

			ep, ok := handler.store.GetEndpoint(nodeName)
			Expect(ok).To(BeTrue())
			Expect(ep.Subnets[0]).To(Equal(node.Annotations[constants.KeyPodSubnets]))

			_, ipNet, err := net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).Should(BeNil())
			Expect(handler.allocator.IsAllocated(*ipNet))
		})

		It("should reallocate a subnet to a edge node if this node's subnet is out of expected range", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "2.3.2.1/26")
			Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())

			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())

			ep, ok := handler.store.GetEndpoint(nodeName)
			Expect(ok).To(BeTrue())
			Expect(ep.Subnets[0]).To(Equal(node.Annotations[constants.KeyPodSubnets]))

			_, ipNet, err := net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).Should(BeNil())
			Expect(handler.allocator.IsAllocated(*ipNet))
		})

		It("should reallocate a subnet to a edge node if this node's subnet is not match to record in store", func() {
			nodeName := getNodeName()
			handler.store.SaveEndpoint(types.Endpoint{
				Name:            nodeName,
				PublicAddresses: nil,
				Subnets:         []string{"2.2.2.2/26"},
				NodeSubnets:     nil,
			})
			node := newNode(nodeName, "10.40.20.181", "2.2.2.1/26")
			Expect(k8sClient.Create(context.Background(), &node)).ShouldNot(HaveOccurred())

			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			Expect(k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)).Should(Succeed())

			ep, ok := handler.store.GetEndpoint(nodeName)
			Expect(ok).To(BeTrue())
			Expect(ep.Subnets[0]).To(Equal(node.Annotations[constants.KeyPodSubnets]))

			_, ipNet, err := net.ParseCIDR(node.Annotations[constants.KeyPodSubnets])
			Expect(err).Should(BeNil())
			Expect(handler.allocator.IsAllocated(*ipNet))
		})
	})

	Context("Undo method", func() {
		It("can reclaim subnets allocated to a edge node", func() {
			nodeName := getNodeName()

			node := newNode(nodeName, "10.40.20.181", "")
			Expect(k8sClient.Create(context.Background(), &node)).Should(Succeed())
			Expect(handler.Do(context.TODO(), node)).Should(Succeed())

			ep, ok := handler.store.GetEndpoint(nodeName)
			Expect(ok).Should(BeTrue())

			_, ipNet, err := net.ParseCIDR(ep.Subnets[0])
			Expect(err).Should(BeNil())

			Expect(handler.Undo(context.TODO(), nodeName)).Should(Succeed())

			_, ok = handler.store.GetEndpoint(nodeName)
			Expect(ok).Should(BeFalse())

			Expect(handler.allocator.IsAllocated(*ipNet)).Should(BeFalse())
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

		handler = &rawPodCIDRsHandler{
			store:       store,
			newEndpoint: types.GenerateNewEndpointFunc("C=CN, O=fabedge.io, CN={node}", nodeutil.GetPodCIDRs),
		}
	})

	AfterEach(func() {
		Expect(testutil.PurgeAllNodes(k8sClient)).Should(Succeed())
	})

	It("build endpoint using spec.PodCIDRs", func() {
		nodeName := getNodeName()
		node := newNode(nodeName, "10.40.20.181", "2.2.2.2/26")

		Expect(handler.Do(context.TODO(), node)).Should(Succeed())

		ep, ok := handler.store.GetEndpoint(nodeName)
		Expect(ok).To(BeTrue())
		Expect(len(ep.Subnets)).Should(Equal(1))
		Expect(ep.Subnets).To(Equal(node.Spec.PodCIDRs))
	})
})
