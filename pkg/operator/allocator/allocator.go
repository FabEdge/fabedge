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

package allocator

import (
	"fmt"
	"hash/fnv"
	"math/big"
	"math/rand"
	"net"
	"sync"
	"time"
)

var (
	ErrNoAvailableSubnet     = fmt.Errorf("no subnet available")
	ErrInvalidSubnetMaskSize = fmt.Errorf("invalid subnet mask size")
)

type NextBlockFunc func() *net.IPNet
type Interface interface {
	Record(ipNet net.IPNet) error
	Reclaim(ipNet net.IPNet) error
	IsAllocated(net.IPNet) bool
	Contains(ipNet net.IPNet) bool
	GetFreeSubnetBlock(hostname string) (*net.IPNet, error)
}

var _ Interface = &allocator{}

type allocator struct {
	netCIDR   string
	pool      *net.IPNet
	blockMask net.IPMask

	subnetCache map[string]bool

	mux sync.RWMutex
}

func New(netCIDR string, subnetMaskSize int) (Interface, error) {
	_, pool, err := net.ParseCIDR(netCIDR)
	if err != nil {
		return nil, err
	}

	var blockMask net.IPMask
	if pool.IP.To4() != nil {
		maskSize, _ := pool.Mask.Size()
		if maskSize >= subnetMaskSize || subnetMaskSize > 32 {
			return nil, ErrInvalidSubnetMaskSize
		}
		blockMask = net.CIDRMask(subnetMaskSize, 32)
	} else {
		maskSize, _ := pool.Mask.Size()
		if maskSize >= subnetMaskSize || subnetMaskSize > 128 {
			return nil, ErrInvalidSubnetMaskSize
		}
		blockMask = net.CIDRMask(subnetMaskSize, 128)
	}

	return &allocator{
		netCIDR:     netCIDR,
		blockMask:   blockMask,
		pool:        pool,
		subnetCache: make(map[string]bool),
	}, nil
}

func (a *allocator) Record(ipNet net.IPNet) error {
	if !a.Contains(ipNet) {
		return fmt.Errorf("%s is out of range of %s", ipNet, a.pool)
	}

	a.mux.Lock()
	defer a.mux.Unlock()

	a.record(ipNet)
	return nil
}

func (a *allocator) record(ipNet net.IPNet) {
	a.subnetCache[ipNet.String()] = true
}

func (a *allocator) Reclaim(ipNet net.IPNet) error {
	if !a.Contains(ipNet) {
		return fmt.Errorf("%s is out of range of %s", ipNet, a.pool)
	}

	a.mux.Lock()
	defer a.mux.Unlock()

	a.reclaim(ipNet)
	return nil
}

func (a *allocator) reclaim(ipNet net.IPNet) {
	delete(a.subnetCache, ipNet.String())
}

func (a *allocator) IsAllocated(ipNet net.IPNet) bool {
	a.mux.RLock()
	defer a.mux.RUnlock()

	return a.isAllocated(ipNet)
}

func (a *allocator) isAllocated(ipNet net.IPNet) bool {
	return a.subnetCache[ipNet.String()]
}

func (a *allocator) Contains(sn net.IPNet) bool {
	return a.pool.Contains(sn.IP) && a.pool.Contains(lastIP(sn))
}

func (a *allocator) GetFreeSubnetBlock(hostname string) (*net.IPNet, error) {
	nextBlock := a.generateNextBlock(hostname)

	a.mux.Lock()
	defer a.mux.Unlock()

	for block := nextBlock(); block != nil; block = nextBlock() {
		if !a.isAllocated(*block) {
			a.record(*block)
			return block, nil
		}
	}

	return nil, ErrNoAvailableSubnet
}

func (a *allocator) generateNextBlock(hostname string) NextBlockFunc {
	pool, baseIP, blockMask := a.pool, a.pool.IP, a.blockMask

	// Determine the number of blocks within this pool.
	ones, size := pool.Mask.Size()
	numIP := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(size-ones)), nil)

	ones, size = blockMask.Size()
	blockSize := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(size-ones)), nil)

	numBlocks := new(big.Int)
	numBlocks.Div(numIP, blockSize)

	// Build a random number generator.
	seed := determineSeed(blockMask, hostname)
	randm := rand.New(rand.NewSource(seed))

	// initialIndex keeps track of the random starting point
	initialIndex := new(big.Int)
	initialIndex.Rand(randm, numBlocks)

	// i keeps track of current index while walking the blocks in a pool
	i := initialIndex

	// numReturned keeps track of number of blocks returned
	numReturned := big.NewInt(0)

	// numDiff = numBlocks - i
	numDiff := new(big.Int)

	return func() *net.IPNet {
		// The `big.NewInt(0)` part creates a temp variable and assigns the result of multiplication of `i` and `big.NewInt(blockSize)`
		// Note: we are not using `i.Mul()` because that will assign the result of the multiplication to `i`, which will cause unexpected issues
		ip := incrementIP(baseIP, big.NewInt(0).Mul(i, blockSize))
		if ip.To16() == nil && ip.To4() == nil {
			// no ip available
			return nil
		}

		ipNet := net.IPNet{IP: ip, Mask: blockMask}

		numDiff.Sub(numBlocks, i)

		if numDiff.Cmp(big.NewInt(1)) <= 0 {
			// Index has reached end of the blocks;
			// Loop back to beginning of pool rather than
			// increment, because incrementing would put us outside of the pool.
			i = big.NewInt(0)
		} else {
			// Increment to the next block
			i.Add(i, big.NewInt(1))
		}

		if numReturned.Cmp(numBlocks) >= 0 {
			// Index finished one full circle across the blocks
			// Used all of the blocks in this pool.
			return nil
		}

		numReturned.Add(numReturned, big.NewInt(1))

		// Return the block from this pool that corresponds with the index.
		return &ipNet
	}
}

func incrementIP(ip net.IP, increment *big.Int) net.IP {
	sum := big.NewInt(0).Add(ipToInt(ip), increment)
	return intToIP(sum)
}

func ipToInt(ip net.IP) *big.Int {
	if ip.To4() != nil {
		return big.NewInt(0).SetBytes(ip.To4())
	} else {
		return big.NewInt(0).SetBytes(ip.To16())
	}
}

func intToIP(ipInt *big.Int) net.IP {
	ip := net.IP(ipInt.Bytes())
	if ip.To4() != nil {
		return ip
	}
	a := ipInt.FillBytes(make([]byte, 16))
	return net.IP(a)
}

func determineSeed(mask net.IPMask, hostname string) int64 {
	if ones, bits := mask.Size(); ones == bits {
		// For small blocks, we don't care about the same host picking the same
		// block, so just use a seed based on timestamp. This optimization reduces
		// the number of reads required to find an unclaimed block on a host.
		return time.Now().UTC().UnixNano()
	}

	// Create a random number generator seed based on the hostname.
	// This is to avoid assigning multiple blocks when multiple
	// workloads request IPs around the same time.
	hostHash := fnv.New32()
	hostHash.Write([]byte(hostname))
	return int64(hostHash.Sum32())
}

// Determine the last IP of a subnet
func lastIP(subnet net.IPNet) net.IP {
	var end net.IP
	for i := 0; i < len(subnet.IP); i++ {
		end = append(end, subnet.IP[i]|^subnet.Mask[i])
	}

	return end
}

func IsNoTAvailable(err error) bool {
	return err == ErrNoAvailableSubnet
}
