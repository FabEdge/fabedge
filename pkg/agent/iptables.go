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
	"fmt"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/util/ipset"
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
		ipt           *iptables.IPTables
		peerIPSet     IPSet
		loopbackIPSet IPSet
		subnets       []string
	}{
		{
			ipt: m.ipt4,
			peerIPSet: IPSet{
				IPSet: &ipset.IPSet{
					Name:       IPSetFabEdgePeerCIDR,
					SetType:    ipset.HashNet,
					HashFamily: ipset.ProtocolFamilyIPV4,
				},
				EntrySet: peerIPSet4,
			},
			subnets: subnetsIP4,
		},
		{
			ipt: m.ipt6,
			peerIPSet: IPSet{
				IPSet: &ipset.IPSet{
					Name:       IPSetFabEdgePeerCIDR6,
					SetType:    ipset.HashNet,
					HashFamily: ipset.ProtocolFamilyIPV6,
				},
				EntrySet: peerIPSet6,
			},
			subnets: subnetsIP6,
		},
	}

	clearOutgoingChain := !m.areSubnetsEqual(current.Subnets, m.lastSubnets)
	for _, c := range configs {
		if err := m.ensureIPForwardRules(c.ipt, c.subnets); err != nil {
			return err
		}

		if m.MASQOutgoing {
			if err := m.configureOutboundRules(c.ipt, c.peerIPSet, c.subnets, clearOutgoingChain); err != nil {
				return err
			}
		}
	}
	// must be done after configureOutboundRules are executed
	m.lastSubnets = current.Subnets

	return nil
}

func (m *Manager) ensureIPForwardRules(ipt *iptables.IPTables, subnets []string) error {
	if err := ensureChain(ipt, TableFilter, ChainFabEdgeForward); err != nil {
		m.log.Error(err, "failed to check or create iptables chain", "table", TableFilter, "chain", ChainFabEdgeForward)
		return err
	}

	ensureRule := ipt.AppendUnique
	if err := ensureRule(TableFilter, ChainForward, "-j", ChainFabEdgeForward); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainForward, "rule", "-j FABEDGE")
		return err
	}

	// subnets won't change most of the time, and is append-only, so for now we don't need
	// to handle removing old subnet
	for _, subnet := range subnets {
		if err := ensureRule(TableFilter, ChainFabEdgeForward, "-s", subnet, "-j", "ACCEPT"); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainFabEdgeForward, "rule", fmt.Sprintf("-s %s -j ACCEPT", subnet))
			return err
		}

		if err := ensureRule(TableFilter, ChainFabEdgeForward, "-d", subnet, "-j", "ACCEPT"); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainFabEdgeForward, "rule", fmt.Sprintf("-d %s -j ACCEPT", subnet))
			return err
		}
	}

	return nil
}

// outbound NAT from pods to outside the cluster
func (m *Manager) configureOutboundRules(ipt *iptables.IPTables, peerIPSet IPSet, subnets []string, clearFabEdgeNatOutgoingChain bool) error {
	if clearFabEdgeNatOutgoingChain {
		m.log.V(3).Info("Subnets are changed, clear iptables chain FABEDGE-NAT-OUTGOING")
		if err := ipt.ClearChain(TableNat, ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing)
			return err
		}
	} else {
		if err := ensureChain(ipt, TableNat, ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing)
			return err
		}
	}

	if err := m.ipset.EnsureIPSet(peerIPSet.IPSet, peerIPSet.EntrySet); err != nil {
		m.log.Error(err, "failed to sync ipset", "ipsetName", peerIPSet.IPSet.Name)
		return err
	}

	for _, subnet := range subnets {
		m.log.V(3).Info("configure outgoing NAT iptables rules")

		ensureRule := ipt.AppendUnique
		if err := ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-m", "set", "--match-set", peerIPSet.IPSet.Name, "dst", "-j", "RETURN"); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -m set --match-set %s dst -j RETURN", subnet, peerIPSet.IPSet.Name))
			continue
		}

		if err := ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-d", subnet, "-j", "RETURN"); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -d %s -j RETURN", subnet, subnet))
			continue
		}

		if err := ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-j", ChainMasquerade); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -j %s", subnet, ChainMasquerade))
			continue
		}

		if err := ensureRule(TableNat, ChainPostRouting, "-j", ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainPostRouting, "rule", fmt.Sprintf("-j %s", ChainFabEdgeNatOutgoing))
			continue
		}
	}

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

func ensureChain(ipt *iptables.IPTables, table, chain string) error {
	exists, err := ipt.ChainExists(table, chain)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return ipt.NewChain(table, chain)
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
