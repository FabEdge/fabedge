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
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/pkg/util/iptables"
)

type IPSet struct {
	IPSet    *ipset.IPSet
	EntrySet sets.String
}

func (m *Manager) ensureIPTablesRules() error {
	current := m.getCurrentEndpoint()

	peerIPSet4, peerIPSet6 := m.getAllPeerCIDRs()
	subnetsIP4, subnetsIP6 := classifySubnets(current.Subnets)

	configs := []struct {
		peerIPSet     IPSet
		loopbackIPSet IPSet
		subnets       []string
		helper        *iptables.IPTablesHelper
	}{
		{
			peerIPSet: IPSet{
				IPSet: &ipset.IPSet{
					Name:       IPSetFabEdgePeerCIDR,
					SetType:    ipset.HashNet,
					HashFamily: ipset.ProtocolFamilyIPV4,
				},
				EntrySet: peerIPSet4,
			},
			subnets: subnetsIP4,
			helper:  iptables.NewIPTablesHelper(),
		},
		{
			peerIPSet: IPSet{
				IPSet: &ipset.IPSet{
					Name:       IPSetFabEdgePeerCIDR6,
					SetType:    ipset.HashNet,
					HashFamily: ipset.ProtocolFamilyIPV6,
				},
				EntrySet: peerIPSet6,
			},
			subnets: subnetsIP6,
			helper:  iptables.NewIP6TablesHelper(),
		},
	}

	// As we will generate the full rule set, we du not need to calculate if subnets are equal
	// clearOutgoingChain := !m.areSubnetsEqual(current.Subnets, m.lastSubnets)

	for _, c := range configs {
		c.helper.ClearAllRules()

		// ensureIPForwardRules
		c.helper.CreateFabEdgeForwardChain()
		c.helper.PrepareForwardChain()

		// subnets won't change most of the time, and is append-only, so for now we don't need
		// to handle removing old subnet
		c.helper.MaintainForwardRulesForSubnets(c.subnets)

		if m.MASQOutgoing {
			// outbound NAT from pods to outside the cluster
			c.helper.CreateFabEdgeNatOutgoingChain()
			if err := m.ipset.EnsureIPSet(c.peerIPSet.IPSet, c.peerIPSet.EntrySet); err != nil {
				m.log.Error(err, "failed to sync ipset", "ipsetName", c.peerIPSet.IPSet.Name)
				return err
			}
			c.helper.MaintainNatOutgoingRulesForSubnets(c.subnets, c.peerIPSet.IPSet.Name)
		}

		if err := c.helper.ReplaceRules(); err != nil {
			m.log.Error(err, "failed to sync iptables rules")
		} else {
			m.log.V(5).Info("iptables rules is synced")
		}
	}

	// must be done after configureOutboundRules are executed
	// m.lastSubnets = current.Subnets

	return nil
}

func (m *Manager) areSubnetsEqual(sa1, sa2 []string) bool {
	if len(sa1) != len(sa2) {
		return false
	}

	for i := range sa1 {
		if sa1[i] != sa2[i] {
			return false
		}
	}

	return true
}

func (m *Manager) getAllPeerCIDRs() (cidrSet4, cidrSet6 sets.String) {
	cidrSet4, cidrSet6 = sets.NewString(), sets.NewString()

	for _, peer := range m.getPeerEndpoints() {
		for _, nodeSubnet := range peer.NodeSubnets {
			if isIPv6(nodeSubnet) {
				cidrSet6.Insert(nodeSubnet)
			} else {
				cidrSet4.Insert(nodeSubnet)
			}
		}

		for _, subnet := range peer.Subnets {
			if isIPv6(subnet) {
				cidrSet6.Insert(subnet)
			} else {
				cidrSet4.Insert(subnet)
			}
		}
	}

	return cidrSet4, cidrSet6
}

func classifySubnets(subnets []string) (ipv4, ipv6 []string) {
	for _, subnet := range subnets {
		if isIPv6(subnet) {
			ipv6 = append(ipv6, subnet)
		} else {
			ipv4 = append(ipv4, subnet)
		}
	}

	return ipv4, ipv6
}

func isIPv6(addr string) bool {
	return strings.Index(addr, ":") != -1
}
