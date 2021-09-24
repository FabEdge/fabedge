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
	IPSetEdgeNodeCIDR       = "FABEDGE-EDGE-NODE-CIDR"
	IPSetCloudPodCIDR       = "FABEDGE-CLOUD-POD-CIDR"
	IPSetCloudNodeCIDR      = "FABEDGE-CLOUD-NODE-CIDR"
	IPSetEdgePodCIDR        = "FABEDGE-EDGE-POD-CIDR"
)

func (m *Manager) clearFabedgeIptablesChains() error {
	err := m.ipt.ClearChain(TableFilter, ChainFabEdgeInput)
	if err != nil {
		return err
	}
	err = m.ipt.ClearChain(TableFilter, ChainFabEdgeForward)
	if err != nil {
		return err
	}
	return m.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (m *Manager) ensureForwardIPTablesRules() (err error) {
	// ensure rules exist
	if err = m.ipt.AppendUnique(TableFilter, ChainForward, "-j", ChainFabEdgeForward); err != nil {
		return err
	}

	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
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

func (m *Manager) ensureNatIPTablesRules() (err error) {
	if err = m.ipt.ClearChain(TableNat, ChainFabEdgePostRouting); err != nil {
		return err
	}

	if err = m.ipt.AppendUnique(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting); err != nil {
		return err
	}

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	if err = m.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", IPSetEdgePodCIDR, "src", "-m", "set", "--match-set", IPSetCloudNodeCIDR, "dst", "-j", "MASQUERADE"); err != nil {
		return err
	}

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	return m.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", IPSetEdgeNodeCIDR, "src", "-m", "set", "--match-set", IPSetCloudPodCIDR, "dst", "-j", "MASQUERADE")

}

func (m *Manager) ensureInputIPTablesRules() (err error) {
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
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "esp", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = m.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "ah", "-j", "ACCEPT"); err != nil {
		return err
	}
	return nil
}

func (m *Manager) syncEdgeNodeCIDRSet() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetEdgeNodeCIDR, ipset.HashNet)
	if err != nil {
		return err
	}

	allEdgeNodeCIDRs := m.getAllEdgeNodeCIDRs()

	oldEdgeNodeCIDRs, err := m.getOldEdgeNodeCIDRs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allEdgeNodeCIDRs, oldEdgeNodeCIDRs, ipset.HashNet)
}

func (m *Manager) getAllEdgeNodeCIDRs() sets.String {
	cidrs := sets.NewString()
	for _, c := range m.connections {
		for _, subnet := range c.RemoteNodeSubnets {
			// translate the IP address to CIDR is needed
			// because FABEDGE-EDGE-NODE-CIDR ipset type is hash:net
			if _, _, err := net.ParseCIDR(subnet); err != nil {
				subnet = m.ipset.ConvertIPToCIDR(subnet)
			}
			cidrs.Insert(subnet)
		}
	}
	return cidrs
}

func (m *Manager) getOldEdgeNodeCIDRs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetEdgeNodeCIDR, ipset.HashNet)
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

func (m *Manager) syncEdgePodCIDRSet() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetEdgePodCIDR, ipset.HashNet)
	if err != nil {
		return err
	}

	allEdgePodCIDRs := m.getAllEdgePodCIDRs()

	oldEdgePodCIDRs, err := m.getOldEdgePodCIDRs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allEdgePodCIDRs, oldEdgePodCIDRs, ipset.HashNet)
}

func (m *Manager) getAllEdgePodCIDRs() sets.String {
	cidrs := sets.NewString()
	for _, c := range m.connections {
		cidrs.Insert(c.RemoteSubnets...)
	}
	return cidrs
}

func (m *Manager) getOldEdgePodCIDRs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetEdgePodCIDR, ipset.HashNet)
}
