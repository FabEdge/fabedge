package types_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ = Describe("PodCIDRStore", func() {
	It("Should support append, remove and get", func() {
		store := types.NewPodCIDRStore()
		nodeName := "node1"

		store.Append(nodeName, "10.10.10.10/26", "10.10.10.20/26", "10.10.10.30/26", "10.10.10.40/26")
		Expect(store.Get(nodeName)).To(ConsistOf("10.10.10.10/26", "10.10.10.20/26", "10.10.10.30/26", "10.10.10.40/26"))

		store.Remove(nodeName, "10.10.10.10/26", "10.10.10.40/26")
		Expect(store.Get(nodeName)).To(ConsistOf("10.10.10.20/26", "10.10.10.30/26"))

		name, ok := store.GetNodeNameByPodCIDR("10.10.10.20/26")
		Expect(ok).Should(BeTrue())
		Expect(name).Should(Equal(nodeName))

		store.RemoveByPodCIDR("10.10.10.30/26")
		Expect(store.Get(nodeName)).To(ConsistOf("10.10.10.20/26"))

		store.RemoveAll(nodeName)
		Expect(store.Get(nodeName)).To(BeNil())
	})
})
