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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/fabedge/fabedge/pkg/operator/allocator"
)

var _ = Describe("Allocator", func() {
	DescribeTable("function New should return error if either any parameter is invalid", func(netCIDR string, subnetMaskSize int) {
		_, err := allocator.New(netCIDR, subnetMaskSize)
		Expect(err).NotTo(BeNil())
	},
		Entry("invalid IPv4 netCIDR", "2.2.2.2.2/16", 24),
		Entry("invalid IPv4 subnetMaskSize", "2.2.2.2/16", 1),
		Entry("invalid IPv4 subnetMaskSize", "2.2.2.2/16", 16),
		Entry("invalid IPv4 subnetMaskSize", "2.2.2.2/16", 33),
		Entry("invalid IPv6 netCIDR", "fd85:ee78:d8a6:8607::1:0000/129", 125),
		Entry("invalid IPv6 subnetMaskSize", "fd85:ee78:d8a6:8607::1:0000/112", 1),
		Entry("invalid IPv6 subnetMaskSize", "fd85:ee78:d8a6:8607::1:0000/112", 112),
		Entry("invalid IPv6 subnetMaskSize", "fd85:ee78:d8a6:8607::1:0000/112", 129),
	)

	DescribeTable("support recording subnet allocation and reclaim subnet", func(netCIDR string, subnetMaskSize int, inRangePodCIDR, outRangePodCIDR string) {
		alloc, err := allocator.New(netCIDR, subnetMaskSize)
		Expect(err).To(BeNil())

		_, subnet, _ := net.ParseCIDR(inRangePodCIDR)
		Expect(alloc.Record(*subnet)).To(Succeed())
		Expect(alloc.IsAllocated(*subnet)).To(BeTrue())

		Expect(alloc.Reclaim(*subnet)).To(Succeed())
		Expect(alloc.IsAllocated(*subnet)).To(BeFalse())

		_, subnet, _ = net.ParseCIDR(outRangePodCIDR)
		Expect(alloc.Record(*subnet)).To(HaveOccurred())
		Expect(alloc.Reclaim(*subnet)).To(HaveOccurred())
	},
		Entry("", "2.2.0.0/16", 26, "2.2.2.1/26", "2.3.2.1/26"),
		Entry("", "fd85:ee78:d8a6:8607::1:0000/112", 122, "fd85:ee78:d8a6:8607::1:0001/122", "fd85:ee79:d8a6:8607::1:0001/122"),
	)

	DescribeTable("can check where an subnet is in pool's range", func(netCIDR string, subnetMaskSize int, inRangePodCIDR, outRangePodCIDR string) {
		alloc, err := allocator.New(netCIDR, subnetMaskSize)
		Expect(err).To(BeNil())

		_, subnet, _ := net.ParseCIDR(inRangePodCIDR)
		Expect(alloc.Contains(*subnet)).To(BeTrue())

		_, subnet, _ = net.ParseCIDR(outRangePodCIDR)
		Expect(alloc.Contains(*subnet)).To(BeFalse())
	},
		Entry("IPv4", "2.2.0.0/16", 26, "2.2.2.1/26", "2.3.2.1/26"),
		Entry("IPv6", "fd85:ee78:d8a6:8607::1:0000/112", 122, "fd85:ee78:d8a6:8607::1:0001/122", "fd85:ee79:d8a6:8607::1:0001/122"),
	)

	DescribeTable("should get different subnet every time", func(netCIDR string, subnetMaskSize int) {
		alloc, err := allocator.New(netCIDR, subnetMaskSize)
		Expect(err).To(BeNil())

		subnet, err := alloc.GetFreeSubnetBlock("node")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(alloc.IsAllocated(*subnet)).To(BeTrue())

		subnets := make(map[string]bool, 1024)
		for i := 0; i < 1023; i++ {
			sn, err := alloc.GetFreeSubnetBlock("node")
			Expect(err).To(BeNil())
			Expect(alloc.IsAllocated(*sn)).To(BeTrue())

			subnets[sn.String()] = true
		}

		Expect(subnets[subnet.String()]).Should(BeFalse())
	},
		Entry("IPv4", "2.2.0.0/16", 26),
		Entry("IPv6", "fd85:ee78:d8a6:8607::1:0000/112", 122),
	)
})
