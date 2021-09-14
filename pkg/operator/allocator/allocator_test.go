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

package allocator_test

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/fabedge/fabedge/pkg/operator/allocator"
)

var _ = Describe("Allocator", func() {

	It("should support recording subnet allocation and reclaim subnet", func() {
		alloc, _ := allocator.New("2.2.0.0/16")
		_, subnet, _ := net.ParseCIDR("2.2.2.1/26")

		alloc.Record(*subnet)
		Expect(alloc.IsAllocated(*subnet)).To(BeTrue())

		alloc.Reclaim(*subnet)
		Expect(alloc.IsAllocated(*subnet)).To(BeFalse())
	})

	It("can check where an subnet is in pool's range", func() {
		alloc, _ := allocator.New("2.2.0.0/16")
		_, subnet, _ := net.ParseCIDR("2.2.2.1/26")

		Expect(alloc.Contains(*subnet)).To(BeTrue())

		_, subnet, _ = net.ParseCIDR("2.3.2.1/26")
		Expect(alloc.Contains(*subnet)).To(BeFalse())
	})

	It("should get different subnet every time", func() {
		alloc, _ := allocator.New("2.2.0.0/16")

		subnet, err := alloc.GetFreeSubnetBlock("node")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(alloc.IsAllocated(*subnet)).To(BeTrue())

		subnets := make(map[string]bool, 1024)
		for i := 0; i < 1023; i++ {
			sn, err := alloc.GetFreeSubnetBlock("node")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(alloc.IsAllocated(*sn)).To(BeTrue())

			subnets[sn.String()] = true
		}

		Expect(subnets[subnet.String()]).Should(BeFalse())
	})

	It("should return error if no available subnets", func() {
		alloc, _ := allocator.New("2.2.2.1/26")

		_, err := alloc.GetFreeSubnetBlock("node")
		Expect(err).ShouldNot(HaveOccurred())

		_, err = alloc.GetFreeSubnetBlock("node")
		Expect(allocator.IsNoTAvailable(err)).Should(BeTrue())
	})

	It("Method new should return an error given wrong cidr", func() {
		_, err := allocator.New("2.2.2.2.2")
		Expect(err).Should(HaveOccurred())
	})
})
