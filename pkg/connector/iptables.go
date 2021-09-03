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

package connector

import (
	"net"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/util/ipset"
)

const (
	TableFilter             = "filter"
	TableNat                = "nat"
	ChainInput              = "INPUT"
	ChainForward            = "FORWARD"
	ChainPostRouting        = "POSTROUTING"
	ChainFabEdgeInput       = "FABEDGE-INPUT"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting = "FABEDGE-POSTROUTING"
	IPSetEdgeNodeIP         = "FABEDGE-EDGE-NODE-IP"
	IPSetCloudPodCIDR       = "FABEDGE-CLOUD-POD-CIDR"
	IPSetCloudNodeCIDR      = "FABEDGE-CLOUD-NODE-CIDR"
)

func (m *Manager) ensureForwardIPTablesRules() error {
	existed, err := m.ipt.ChainExists(TableFilter, ChainFabEdgeForward)
	if err != nil {
		return err
	}

	if !existed {
		return m.ipt.NewChain(TableFilter, ChainFabEdgeForward)
	}

	// ensure rules exist
	if err = m.ipt.AppendUnique(TableFilter, ChainForward, "-j", ChainFabEdgeForward); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", IPSetCloudPodCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", IPSetCloudPodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", IPSetCloudNodeCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", IPSetCloudNodeCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func (m *Manager) ensureSNatIPTablesRules() error {
	existed, err := m.ipt.ChainExists(TableNat, ChainFabEdgePostRouting)
	if err != nil {
		return err
	}

	if !existed {
		return m.ipt.NewChain(TableNat, ChainFabEdgePostRouting)
	}

	if err = m.ipt.AppendUnique(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting); err != nil {
		return err
	}

	for _, c := range m.connections {
		for _, addr := range c.LocalAddress {
			exists, err := m.ipt.Exists(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", IPSetEdgeNodeIP, "src", "-m", "set", "--match-set", IPSetCloudPodCIDR, "dst", "-j", "SNAT", "--to", addr)
			if err != nil {
				return err
			}

			if !exists {
				// TODO: fixed connector.IP update issue.
				// now c.LocalAddress contains only one connector.IP,
				// and if there are more than one connector.IP in c.LocalAddress,
				// the processing logic here is going to be problematic
				if err = m.ipt.ClearChain(TableNat, ChainFabEdgePostRouting); err != nil {
					return err
				}

				if err = m.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", IPSetEdgeNodeIP, "src", "-m", "set", "--match-set", IPSetCloudPodCIDR, "dst", "-j", "SNAT", "--to", addr); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (m *Manager) ensureInputIPTablesRules() error {
	existed, err := m.ipt.ChainExists(TableFilter, ChainFabEdgeInput)
	if err != nil {
		return err
	}

	if !existed {
		return m.ipt.NewChain(TableFilter, ChainFabEdgeInput)
	}

	// ensure rules exist
	if err = m.ipt.AppendUnique(TableFilter, ChainInput, "-j", ChainFabEdgeInput); err != nil {
		return err
	}

	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "500", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "4500", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "esp"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "ah"); err != nil {
		return err
	}
	return nil
}

func (m *Manager) syncEdgeNodeIPSet() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetEdgeNodeIP, ipset.HashIP)
	if err != nil {
		return err
	}

	allEdgeNodeIPs := m.getAllEdgeNodeIPs()

	oldEdgeNodeIPs, err := m.getOldEdgeNodeIPs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allEdgeNodeIPs, oldEdgeNodeIPs, ipset.HashIP)
}

func (m *Manager) getAllEdgeNodeIPs() sets.String {
	ips := sets.NewString()
	for _, c := range m.connections {
		ips.Insert(c.RemoteAddress...)
	}
	return ips
}

func (m *Manager) getOldEdgeNodeIPs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetEdgeNodeIP, ipset.HashIP)
}

func (m *Manager) syncCloudPodCIDRSet() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetCloudPodCIDR, ipset.HashNet)
	if err != nil {
		return err
	}

	allCloudPodCIDRs := m.getAllCloudPodCIDRs()

	oldCloudPodCIDRs, err := m.getOldCloudPodCIDRs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allCloudPodCIDRs, oldCloudPodCIDRs, ipset.HashNet)
}

func (m *Manager) getAllCloudPodCIDRs() sets.String {
	cidrs := sets.NewString()
	for _, c := range m.connections {
		cidrs.Insert(c.LocalSubnets...)
	}
	return cidrs
}

func (m *Manager) getOldCloudPodCIDRs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetCloudPodCIDR, ipset.HashNet)
}

func (m *Manager) CleanSNatIPTablesRules() error {
	return m.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (m *Manager) syncCloudNodeCIDRSet() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetCloudNodeCIDR, ipset.HashNet)
	if err != nil {
		return err
	}

	allCloudNodeCIDRs := m.getAllCloudNodeCIDRs()

	oldCloudNodeCIDRs, err := m.getOldCloudNodeCIDRs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allCloudNodeCIDRs, oldCloudNodeCIDRs, ipset.HashNet)
}

func (m *Manager) getAllCloudNodeCIDRs() sets.String {
	cidrs := sets.NewString()
	for _, c := range m.connections {
		for _, subnet := range c.LocalNodeSubnets {
			// translate the IP address to CIDR is needed
			// because FABEDGE-CLOUD-NODE-CIDR ipset type is hash:net
			if _, _, err := net.ParseCIDR(subnet); err != nil {
				subnet = m.ipset.ConvertIPToCIDR(subnet)
			}
			cidrs.Insert(subnet)
		}
	}
	return cidrs
}

func (m *Manager) getOldCloudNodeCIDRs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetCloudNodeCIDR, ipset.HashNet)
}
