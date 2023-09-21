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

	ipt, err := iptables.NewIPTablesHelper()
	if err != nil {
		return err
	}

	ipt6, err := iptables.NewIP6TablesHelper()
	if err != nil {
		return err
	}

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
			helper:  ipt,
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
			helper:  ipt6,
		},
	}

	// As we will generate the full rule set, we du not need to calculate if subnets are equal
	// clearOutgoingChain := !m.areSubnetsEqual(current.Subnets, m.lastSubnets)

	for _, c := range configs {
		c.helper.ClearAllRules()

		// ensureIPForwardRules
		c.helper.CreateFabEdgeForwardChain()
		c.helper.NewPrepareForwardChain()

		// subnets won't change most of the time, and is append-only, so for now we don't need
		// to handle removing old subnet
		c.helper.NewMaintainForwardRulesForSubnets(c.subnets)

		if m.MASQOutgoing {
			c.helper.CreateFabEdgeNatOutgoingChain()
			if err := m.ipset.EnsureIPSet(c.peerIPSet.IPSet, c.peerIPSet.EntrySet); err != nil {
				m.log.Error(err, "failed to sync ipset", "ipsetName", c.peerIPSet.IPSet.Name)
				return err
			}
			c.helper.NewMaintainNatOutgoingRulesForSubnets(c.subnets, c.peerIPSet.IPSet.Name)
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

func (m *Manager) ensureIPForwardRules(helper *iptables.IPTablesHelper, subnets []string) error {
	if err := helper.CheckOrCreateFabEdgeForwardChain(); err != nil {
		m.log.Error(err, "failed to check or create iptables chain", "table", iptables.TableFilter, "chain", iptables.ChainFabEdgeForward)
		return err
	}

	if err := helper.PrepareForwardChain(); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", iptables.TableFilter, "chain", iptables.ChainForward, "rule", "-j FABEDGE-FORWARD")
		return err
	}

	// subnets won't change most of the time, and is append-only, so for now we don't need
	// to handle removing old subnet
	if err, errRule := helper.MaintainForwardRulesForSubnets(subnets); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", iptables.TableFilter, "chain", iptables.ChainFabEdgeForward, "rule", errRule)
		return err
	}

	return nil
}

// outbound NAT from pods to outside the cluster
func (m *Manager) configureOutboundRules(helper *iptables.IPTablesHelper, peerIPSet IPSet, subnets []string, clearFabEdgeNatOutgoingChain bool) error {
	if clearFabEdgeNatOutgoingChain {
		m.log.V(3).Info("Subnets are changed, clear iptables chain FABEDGE-NAT-OUTGOING")
		if err := helper.ClearOrCreateFabEdgeNatOutgoingChain(); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", iptables.TableNat, "chain", iptables.ChainFabEdgeNatOutgoing)
			return err
		}
	} else {
		if err := helper.CheckOrCreateFabEdgeNatOutgoingChain(); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", iptables.TableNat, "chain", iptables.ChainFabEdgeNatOutgoing)
			return err
		}
	}

	if err := m.ipset.EnsureIPSet(peerIPSet.IPSet, peerIPSet.EntrySet); err != nil {
		m.log.Error(err, "failed to sync ipset", "ipsetName", peerIPSet.IPSet.Name)
		return err
	}

	m.log.V(3).Info("configure outgoing NAT iptables rules")
	if err, errRule := helper.MaintainNatOutgoingRulesForSubnets(subnets, peerIPSet.IPSet.Name); err != nil {
		m.log.Error(err, "failed to append rule", "table", iptables.TableNat, "chain", iptables.ChainFabEdgeNatOutgoing, "rule", errRule)
		return err
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
