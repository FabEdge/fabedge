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
	"fmt"
	"net"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/util/ipset"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
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
	IPSetEdgeNodeCIDRIPV6   = "FABEDGE-EDGE-NODE-CIDR-IPV6"
	IPSetCloudPodCIDR       = "FABEDGE-CLOUD-POD-CIDR"
	IPSetCloudPodCIDRIPV6   = "FABEDGE-CLOUD-POD-CIDR-IPV6"
	IPSetCloudNodeCIDR      = "FABEDGE-CLOUD-NODE-CIDR"
	IPSetCloudNodeCIDRIPV6  = "FABEDGE-CLOUD-NODE-CIDR-IPV6"
	IPSetEdgePodCIDR        = "FABEDGE-EDGE-POD-CIDR"
	IPSetEdgePodCIDRIPV6    = "FABEDGE-EDGE-POD-CIDR-IPV6"
)

func (m *Manager) clearFabedgeIPTablesChains() error {
	// iptables
	e1 := clearFabedgeIPTablesChains(m.ipt)

	// ip6tables
	e2 := clearFabedgeIPTablesChains(m.ip6t)

	if e1 != nil || e2 != nil {
		return fmt.Errorf("iptables error: %v, ip6tables error: %v", e1, e2)
	}

	return nil
}

func clearFabedgeIPTablesChains(ipts *iptables.IPTables) error {
	err := ipts.ClearChain(TableFilter, ChainFabEdgeInput)
	if err != nil {
		return err
	}
	err = ipts.ClearChain(TableFilter, ChainFabEdgeForward)
	if err != nil {
		return err
	}
	return ipts.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (m *Manager) ensureForwardIPTablesRules() error {
	// iptables
	err := ensureForwardIPTablesRules(m.ipt)
	if err != nil {
		return err
	}

	// ip6tables
	return ensureForwardIPTablesRules(m.ip6t)
}

func ensureForwardIPTablesRules(ipts *iptables.IPTables) (err error) {
	ipsetCloudPodCIDR, ipsetCloudNodeCIDR := IPSetCloudPodCIDR, IPSetCloudNodeCIDR

	if ipts.Proto() == iptables.ProtocolIPv6 {
		ipsetCloudPodCIDR, ipsetCloudNodeCIDR = IPSetCloudPodCIDRIPV6, IPSetCloudNodeCIDRIPV6
	}

	// ensure rules exist
	if err = ipts.AppendUnique(TableFilter, ChainForward, "-j", ChainFabEdgeForward); err != nil {
		return err
	}

	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetCloudPodCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetCloudPodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetCloudNodeCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetCloudNodeCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func (m *Manager) ensureNatIPTablesRules() error {
	// iptables
	err := ensureNatIPTablesRules(m.ipt)
	if err != nil {
		return err
	}

	// ip6tables
	return ensureNatIPTablesRules(m.ip6t)
}

func ensureNatIPTablesRules(ipts *iptables.IPTables) (err error) {
	if err = ipts.ClearChain(TableNat, ChainFabEdgePostRouting); err != nil {
		return err
	}
	exists, err := ipts.Exists(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	if err != nil {
		return err
	}

	if !exists {
		if err = ipts.Insert(TableNat, ChainPostRouting, 1, "-j", ChainFabEdgePostRouting); err != nil {
			return err
		}
	}

	ipsetCloudPodCIDR, ipsetCloudNodeCIDR := IPSetCloudPodCIDR, IPSetCloudNodeCIDR
	ipsetEdgePodCIDR, ipsetEdgeNodeCIDR := IPSetEdgePodCIDR, IPSetEdgeNodeCIDR

	if ipts.Proto() == iptables.ProtocolIPv6 {
		ipsetCloudPodCIDR, ipsetCloudNodeCIDR = IPSetCloudPodCIDRIPV6, IPSetCloudNodeCIDRIPV6
		ipsetEdgePodCIDR, ipsetEdgeNodeCIDR = IPSetEdgePodCIDRIPV6, IPSetEdgeNodeCIDRIPV6
	}

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	if err = ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetCloudPodCIDR, "src", "-m", "set", "--match-set", ipsetEdgePodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	if err = ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetEdgePodCIDR, "src", "-m", "set", "--match-set", ipsetCloudPodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	if err = ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetCloudPodCIDR, "src", "-m", "set", "--match-set", ipsetEdgeNodeCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	if err = ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetEdgePodCIDR, "src", "-m", "set", "--match-set", ipsetCloudNodeCIDR, "dst", "-j", "MASQUERADE"); err != nil {
		return err
	}

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	return ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetEdgeNodeCIDR, "src", "-m", "set", "--match-set", ipsetCloudPodCIDR, "dst", "-j", "MASQUERADE")
}

func (m *Manager) ensureInputIPTablesRules() error {
	// iptables
	err := ensureInputIPTablesRules(m.ipt)
	if err != nil {
		return err
	}

	// ip6tables
	return ensureInputIPTablesRules(m.ip6t)
}

func ensureInputIPTablesRules(ipts *iptables.IPTables) (err error) {
	// ensure rules exist
	if err = ipts.AppendUnique(TableFilter, ChainInput, "-j", ChainFabEdgeInput); err != nil {
		return err
	}

	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "500", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "4500", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "esp", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "ah", "-j", "ACCEPT"); err != nil {
		return err
	}
	return nil
}

func (m *Manager) syncEdgeNodeCIDRSet() error {
	ipsetV4, err := m.ipset.EnsureIPSet(IPSetEdgeNodeCIDR, ipset.ProtocolFamilyIPV4, ipset.HashNet)
	if err != nil {
		return err
	}

	ipsetV6, err := m.ipset.EnsureIPSet(IPSetEdgeNodeCIDRIPV6, ipset.ProtocolFamilyIPV6, ipset.HashNet)
	if err != nil {
		return err
	}

	allEdgeNodeCIDRs := m.getAllEdgeNodeCIDRs()

	for protocolVersion, edgePodCIDRs := range allEdgeNodeCIDRs {
		setName := IPSetEdgeNodeCIDR
		ipsetObj := ipsetV4
		if protocolVersion == netutil.IPV6 {
			setName = IPSetEdgeNodeCIDRIPV6
			ipsetObj = ipsetV6
		}

		oldEdgeNodeCIDRs, err := m.ipset.ListEntries(setName, ipset.HashNet)
		if err != nil {
			return err
		}

		err = m.ipset.SyncIPSetEntries(ipsetObj, edgePodCIDRs, oldEdgeNodeCIDRs, ipset.HashNet)
		if err != nil {
			return err
		}
	}

	return nil
}

func inSameCluster(c tunnel.ConnConfig) bool {
	if c.RemoteType == v1alpha1.Connector {
		return false
	}

	l := strings.Split(c.LocalID, ".")  // e.g. fabedge.connector
	r := strings.Split(c.RemoteID, ".") // e.g. fabedge.edge1

	return l[0] == r[0]
}

func (m *Manager) getAllEdgeNodeCIDRs() map[netutil.ProtocolVersion]sets.String {
	dualStackCIDRs := make(map[netutil.ProtocolVersion]sets.String, 2)

	ipv4CIDRs := sets.NewString()
	ipv6CIDRs := sets.NewString()

	for _, c := range m.connections {
		if !inSameCluster(c) {
			continue
		}
		for _, subnet := range c.RemoteNodeSubnets {
			// translate the IP address to CIDR is needed
			// because FABEDGE-EDGE-NODE-CIDR ipset type is hash:net
			_, _, err := net.ParseCIDR(subnet)
			if err != nil {
				subnet = m.ipset.ConvertIPToCIDR(subnet)
			}

			ip, _, _ := net.ParseCIDR(subnet)

			if netutil.IPVersion(ip) == netutil.IPV6 {
				ipv6CIDRs.Insert(subnet)
			} else {
				ipv4CIDRs.Insert(subnet)
			}
		}
	}

	dualStackCIDRs[netutil.IPV4] = ipv4CIDRs
	dualStackCIDRs[netutil.IPV6] = ipv6CIDRs

	return dualStackCIDRs
}

func (m *Manager) syncCloudPodCIDRSet() error {
	ipsetV4, err := m.ipset.EnsureIPSet(IPSetCloudPodCIDR, ipset.ProtocolFamilyIPV4, ipset.HashNet)
	if err != nil {
		return err
	}

	ipsetV6, err := m.ipset.EnsureIPSet(IPSetCloudPodCIDRIPV6, ipset.ProtocolFamilyIPV6, ipset.HashNet)
	if err != nil {
		return err
	}

	allCloudPodCIDRs := m.getAllCloudPodCIDRs()

	for protocolVersion, cloudPodCIDRs := range allCloudPodCIDRs {
		setName := IPSetCloudPodCIDR
		ipsetObj := ipsetV4
		if protocolVersion == netutil.IPV6 {
			setName = IPSetCloudPodCIDRIPV6
			ipsetObj = ipsetV6
		}

		oldCloudPodCIDRs, err := m.ipset.ListEntries(setName, ipset.HashNet)
		if err != nil {
			return err
		}

		err = m.ipset.SyncIPSetEntries(ipsetObj, cloudPodCIDRs, oldCloudPodCIDRs, ipset.HashNet)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) getAllCloudPodCIDRs() map[netutil.ProtocolVersion]sets.String {
	dualStackCIDRs := make(map[netutil.ProtocolVersion]sets.String, 2)

	ipv4CIDRs := sets.NewString()
	ipv6CIDRs := sets.NewString()

	for _, c := range m.connections {
		for _, subnet := range c.LocalSubnets {
			ip, _, _ := net.ParseCIDR(subnet)

			version := netutil.IPVersion(ip)

			if version == netutil.IPV6 {
				ipv6CIDRs.Insert(subnet)
			} else {
				ipv4CIDRs.Insert(subnet)
			}
		}
	}

	dualStackCIDRs[netutil.IPV4] = ipv4CIDRs
	dualStackCIDRs[netutil.IPV6] = ipv6CIDRs

	return dualStackCIDRs
}

func (m *Manager) CleanSNatIPTablesRules() error {
	// iptables
	e1 := m.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)

	// ip6tables
	e2 := m.ip6t.ClearChain(TableNat, ChainFabEdgePostRouting)

	if e1 != nil || e2 != nil {
		return fmt.Errorf("iptables error: %v, ip6tables error: %v", e1, e2)
	}

	return nil
}

func (m *Manager) syncCloudNodeCIDRSet() error {
	ipsetV4, err := m.ipset.EnsureIPSet(IPSetCloudNodeCIDR, ipset.ProtocolFamilyIPV4, ipset.HashNet)
	if err != nil {
		return err
	}

	ipsetV6, err := m.ipset.EnsureIPSet(IPSetCloudNodeCIDRIPV6, ipset.ProtocolFamilyIPV6, ipset.HashNet)
	if err != nil {
		return err
	}

	allCloudNodeCIDRs := m.getAllCloudNodeCIDRs()

	for protocolVersion, cloudPodCIDRs := range allCloudNodeCIDRs {
		setName := IPSetCloudNodeCIDR
		ipsetObj := ipsetV4
		if protocolVersion == netutil.IPV6 {
			setName = IPSetCloudNodeCIDRIPV6
			ipsetObj = ipsetV6
		}

		oldCloudNodeCIDRs, err := m.ipset.ListEntries(setName, ipset.HashNet)
		if err != nil {
			return err
		}

		err = m.ipset.SyncIPSetEntries(ipsetObj, cloudPodCIDRs, oldCloudNodeCIDRs, ipset.HashNet)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) getAllCloudNodeCIDRs() map[netutil.ProtocolVersion]sets.String {
	dualStackCIDRs := make(map[netutil.ProtocolVersion]sets.String, 2)

	ipv4CIDRs := sets.NewString()
	ipv6CIDRs := sets.NewString()

	for _, c := range m.connections {
		if !inSameCluster(c) {
			continue
		}
		for _, subnet := range c.LocalNodeSubnets {
			// translate the IP address to CIDR is needed
			// because FABEDGE-CLOUD-NODE-CIDR ipset type is hash:net
			_, _, err := net.ParseCIDR(subnet)
			if err != nil {
				subnet = m.ipset.ConvertIPToCIDR(subnet)
			}

			ip, _, _ := net.ParseCIDR(subnet)

			if netutil.IPVersion(ip) == netutil.IPV6 {
				ipv6CIDRs.Insert(subnet)
			} else {
				ipv4CIDRs.Insert(subnet)
			}
		}
	}

	dualStackCIDRs[netutil.IPV4] = ipv4CIDRs
	dualStackCIDRs[netutil.IPV6] = ipv6CIDRs

	return dualStackCIDRs
}

func (m *Manager) syncEdgePodCIDRSet() error {
	ipsetV4, err := m.ipset.EnsureIPSet(IPSetEdgePodCIDR, ipset.ProtocolFamilyIPV4, ipset.HashNet)
	if err != nil {
		return err
	}

	ipsetV6, err := m.ipset.EnsureIPSet(IPSetEdgePodCIDRIPV6, ipset.ProtocolFamilyIPV6, ipset.HashNet)
	if err != nil {
		return err
	}

	allEdgePodCIDRs := m.getAllEdgePodCIDRs()

	for protocolVersion, edgePodCIDRs := range allEdgePodCIDRs {
		setName := IPSetEdgePodCIDR
		ipsetObj := ipsetV4
		if protocolVersion == netutil.IPV6 {
			setName = IPSetEdgePodCIDRIPV6
			ipsetObj = ipsetV6
		}

		oldEdgePodCIDRs, err := m.ipset.ListEntries(setName, ipset.HashNet)
		if err != nil {
			return err
		}

		err = m.ipset.SyncIPSetEntries(ipsetObj, edgePodCIDRs, oldEdgePodCIDRs, ipset.HashNet)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) getAllEdgePodCIDRs() map[netutil.ProtocolVersion]sets.String {
	dualStackCIDRs := make(map[netutil.ProtocolVersion]sets.String, 2)

	ipv4CIDRs := sets.NewString()
	ipv6CIDRs := sets.NewString()
	for _, c := range m.connections {
		for _, subnet := range c.RemoteSubnets {
			ip, _, _ := net.ParseCIDR(subnet)

			version := netutil.IPVersion(ip)

			if version == netutil.IPV6 {
				ipv6CIDRs.Insert(subnet)
			} else {
				ipv4CIDRs.Insert(subnet)
			}
		}
	}

	dualStackCIDRs[netutil.IPV4] = ipv4CIDRs
	dualStackCIDRs[netutil.IPV6] = ipv6CIDRs

	return dualStackCIDRs
}
