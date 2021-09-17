package types_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ = Describe("SafeStringSet", func() {
	It("Should support add, remove and contains", func() {
		set := types.NewSafeStringSet("edge")
		Expect(set.Contains("edge")).Should(BeTrue())

		set.Add("edge3", "edge2", "edge1")
		Expect(set.Contains("edge1")).Should(BeTrue())
		Expect(set.Len()).Should(Equal(4))

		set.Remove("edge")
		Expect(set.Contains("edge")).Should(BeFalse())
		Expect(set.Len()).Should(Equal(3))

		Expect(set.Values()).Should(ConsistOf("edge1", "edge2", "edge3"))

		set2 := types.NewSafeStringSet("edge1", "edge2", "edge3")
		Expect(set.Equal(set2)).Should(BeTrue())
	})
})
