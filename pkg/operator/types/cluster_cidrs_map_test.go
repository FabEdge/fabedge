package types_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ = Describe("ClusterCIDRsMap", func() {
	It("can set, get and delete CIDRs by cluster name", func() {
		cidrMap := types.NewClusterCIDRsMap()

		cidrMap.Set("beijing", []string{"192.168.0.0/18"})
		cidrs, found := cidrMap.Get("beijing")
		Expect(found).To(BeTrue())
		Expect(cidrs).To(ConsistOf("192.168.0.0/18"))

		cidrMap.Delete("beijing")
		_, found = cidrMap.Get("beijing")
		Expect(found).To(BeFalse())
	})

	It("GetCopy can get return an copy from inner data", func() {
		cidrMap := types.NewClusterCIDRsMap()

		cidrMap.Set("beijing", []string{"192.168.0.0/18"})
		cp := cidrMap.GetCopy()

		Expect(len(cp)).To(Equal(1))
		Expect(cp).To(HaveKeyWithValue("beijing", []string{"192.168.0.0/18"}))

		cidrMap.Set("shanghai", []string{"10.10.0.0/18"})
		Expect(len(cp)).To(Equal(1))

		cp2 := cidrMap.GetCopy()
		Expect(cp).NotTo(Equal(cp2))
		Expect(cp2).To(HaveKeyWithValue("beijing", []string{"192.168.0.0/18"}))
		Expect(cp2).To(HaveKeyWithValue("shanghai", []string{"10.10.0.0/18"}))

		cp3 := cidrMap.GetCopy()
		Expect(cp2).To(Equal(cp3))
	})
})
