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
	const (
		testCIDRIPv4            = "2.2.0.0/16"
		testSubnetMaskSizeIPv4  = 26
		testCIDRIPv6            = "fd85:ee78:d8a6:8607::1:0000/112"
		testSubnetsMaskSizeIPv6 = 122
	)

	It("Method new should return an error given wrong cidr", func() {
		_, err := allocator.New("2.2.2.2.2", testSubnetMaskSizeIPv4)
		Expect(err).Should(HaveOccurred())
	})

	It("Method new should return an error given invalid leading ones of mask", func() {
		_, err := allocator.New(testCIDRIPv4, 36)
		Expect(err).Should(HaveOccurred())

		_, err = allocator.New(testCIDRIPv6, -1)
		Expect(err).Should(HaveOccurred())
	})

	It("should support recording subnet allocation and reclaim subnet [ipv4]", func() {
		testSubnetAllocationAndReclaim(testCIDRIPv4, testSubnetMaskSizeIPv4, "2.2.2.1/26")
	})

	It("should support recording subnet allocation and reclaim subnet [ipv6]", func() {
		testSubnetAllocationAndReclaim(testCIDRIPv6, testSubnetsMaskSizeIPv6, "fd85:ee78:d8a6:8607::1:0001/122")
	})

	It("can check where an subnet is in pool's range [ipv4]", func() {
		testSubnetInRangeOfPool(testCIDRIPv4, testSubnetMaskSizeIPv4, "2.2.2.1/26")
	})

	It("can check where an subnet is in pool's range [ipv6]", func() {
		testSubnetInRangeOfPool(testCIDRIPv6, testSubnetsMaskSizeIPv6, "fd85:ee78:d8a6:8607::1:0001/122")
	})

	It("should get different subnet every time [ipv4]", func() {
		testRandomSubnet(testCIDRIPv4, testSubnetMaskSizeIPv4)
	})

	It("should get different subnet every time [ipv6]", func() {
		testRandomSubnet(testCIDRIPv6, testSubnetsMaskSizeIPv6)
	})

	It("should return error if no available subnets [ipv4]", func() {
		testSubnetAvailability("2.2.2.1/26", testSubnetMaskSizeIPv4)
	})

	It("should return error if no available subnets [ipv6]", func() {
		testSubnetAvailability("fd85:ee78:d8a6:8607::1:0001/122", testSubnetsMaskSizeIPv6)
	})
})

func testSubnetAllocationAndReclaim(netPool string, leadingOnes int, subnetCIDR string) {
	alloc, _ := allocator.New(netPool, leadingOnes)
	_, subnet, _ := net.ParseCIDR(subnetCIDR)

	alloc.Record(*subnet)
	Expect(alloc.IsAllocated(*subnet)).To(BeTrue())

	alloc.Reclaim(*subnet)
	Expect(alloc.IsAllocated(*subnet)).To(BeFalse())
}

func testSubnetInRangeOfPool(netPool string, leadingOnes int, subnetCIDR string) {
	alloc, _ := allocator.New(netPool, leadingOnes)
	_, subnet, _ := net.ParseCIDR(subnetCIDR)

	Expect(alloc.Contains(*subnet)).To(BeTrue())

	if alloc.Version() == 4 {
		_, subnet, _ = net.ParseCIDR("2.3.2.1/26")
	} else {
		_, subnet, _ = net.ParseCIDR("fabc:ee78:d8a6:8607::1:0001/122")
	}
	Expect(alloc.Contains(*subnet)).To(BeFalse())
}

func testRandomSubnet(netPool string, leadingOnes int) {
	alloc, _ := allocator.New(netPool, leadingOnes)

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
}

func testSubnetAvailability(netPool string, leadingOnes int) {
	alloc, _ := allocator.New(netPool, leadingOnes)

	_, err := alloc.GetFreeSubnetBlock("node")
	Expect(err).ShouldNot(HaveOccurred())

	_, err = alloc.GetFreeSubnetBlock("node")
	Expect(allocator.IsNoTAvailable(err)).Should(BeTrue())
}
