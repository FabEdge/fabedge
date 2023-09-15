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
	"github.com/fabedge/fabedge/pkg/util/rule"
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
		helper        *rule.IPTablesHelper
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
			helper:  rule.NewIPTablesHelper(m.ipt4),
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
			helper:  rule.NewIPTablesHelper(m.ipt6),
		},
	}

	clearOutgoingChain := !m.areSubnetsEqual(current.Subnets, m.lastSubnets)

	for _, c := range configs {
		c.helper.Mutex.Lock()
		if err := m.ensureIPForwardRules(c.helper, c.subnets); err != nil {
			c.helper.Mutex.Unlock()
			return err
		}

		if m.MASQOutgoing {
			if err := m.configureOutboundRules(c.helper, c.peerIPSet, c.subnets, clearOutgoingChain); err != nil {
				c.helper.Mutex.Unlock()
				return err
			}
		}
		c.helper.Mutex.Unlock()
	}
	// must be done after configureOutboundRules are executed
	m.lastSubnets = current.Subnets

	return nil
}

func (m *Manager) ensureIPForwardRules(helper *rule.IPTablesHelper, subnets []string) error {
	if err := helper.CheckOrCreateFabEdgeForwardChain(); err != nil {
		m.log.Error(err, "failed to check or create iptables chain", "table", rule.TableFilter, "chain", rule.ChainFabEdgeForward)
		return err
	}

	if err := helper.PrepareForwardChain(); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", rule.TableFilter, "chain", rule.ChainForward, "rule", "-j FABEDGE-FORWARD")
		return err
	}

	// subnets won't change most of the time, and is append-only, so for now we don't need
	// to handle removing old subnet
	if err, errRule := helper.MaintainForwardRulesForSubnets(subnets); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", rule.TableFilter, "chain", rule.ChainFabEdgeForward, "rule", errRule)
		return err
	}

	return nil
}

// outbound NAT from pods to outside the cluster
func (m *Manager) configureOutboundRules(helper *rule.IPTablesHelper, peerIPSet IPSet, subnets []string, clearFabEdgeNatOutgoingChain bool) error {
	if clearFabEdgeNatOutgoingChain {
		m.log.V(3).Info("Subnets are changed, clear iptables chain FABEDGE-NAT-OUTGOING")
		if err := helper.ClearOrCreateFabEdgeNatOutgoingChain(); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", rule.TableNat, "chain", rule.ChainFabEdgeNatOutgoing)
			return err
		}
	} else {
		if err := helper.CheckOrCreateFabEdgeNatOutgoingChain(); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", rule.TableNat, "chain", rule.ChainFabEdgeNatOutgoing)
			return err
		}
	}

	if err := m.ipset.EnsureIPSet(peerIPSet.IPSet, peerIPSet.EntrySet); err != nil {
		m.log.Error(err, "failed to sync ipset", "ipsetName", peerIPSet.IPSet.Name)
		return err
	}

	m.log.V(3).Info("configure outgoing NAT iptables rules")
	if err, errRule := helper.MaintainNatOutgoingRulesForSubnets(subnets, peerIPSet.IPSet.Name); err != nil {
		m.log.Error(err, "failed to append rule", "table", rule.TableNat, "chain", rule.ChainFabEdgeNatOutgoing, "rule", errRule)
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
