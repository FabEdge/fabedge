package ipamblockmonitor

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fabedge/fabedge/pkg/operator/types"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

var _ = Describe("IPAMBlockMonitor", func() {
	var (
		monitor *ipamBlockMonitor
	)

	BeforeEach(func() {
		monitor = &ipamBlockMonitor{
			Config: Config{
				Store: types.NewPodCIDRStore(),
			},
			client: k8sClient,
			log:    klogr.New(),
		}
	})

	Context("A IPAM block is bound to a node", func() {
		var (
			nodeName string
			block    calicoapi.IPAMBlock
			request  reconcile.Request
		)

		BeforeEach(func() {
			nodeName = "node1"
			affinity := fmt.Sprintf("host:%s", nodeName)

			block = calicoapi.IPAMBlock{
				ObjectMeta: metav1.ObjectMeta{
					Name: "10-233-70-0-24",
				},
				Spec: calicoapi.IPAMBlockSpec{
					CIDR:        "10.233.70.0/24",
					Affinity:    &affinity,
					Unallocated: []int{},
					Allocations: []*int{},
					Attributes:  []calicoapi.AllocationAttribute{},
				},
			}

			request = reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name: block.Name,
				},
			}

			Expect(k8sClient.Create(context.Background(), &block)).To(Succeed())

			_, err := monitor.Reconcile(context.Background(), request)
			Expect(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			_ = k8sClient.Delete(context.Background(), &block)
		})

		It("should record it to store", func() {
			Expect(monitor.Store.Get(nodeName)).To(ConsistOf(block.Spec.CIDR))
		})

		It("delete record when IPAMBlock's delete field is true", func() {
			block.Spec.Deleted = true
			Expect(k8sClient.Update(context.Background(), &block)).To(Succeed())

			_, err := monitor.Reconcile(context.Background(), request)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(monitor.Store.Get(nodeName)).To(ConsistOf())
		})

		It("delete record when IPAMBlock is deleted", func() {
			Expect(k8sClient.Delete(context.Background(), &block)).To(Succeed())

			_, err := monitor.Reconcile(context.Background(), request)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(monitor.Store.Get(nodeName)).To(ConsistOf())
		})
	})
})
