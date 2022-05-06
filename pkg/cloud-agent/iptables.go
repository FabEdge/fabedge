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

package cloud_agent

import (
	"github.com/coreos/go-iptables/iptables"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"

	"github.com/fabedge/fabedge/pkg/connector/routing"
	ipsetUtil "github.com/fabedge/fabedge/pkg/util/ipset"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
)

const (
	TableFilter             = "filter"
	TableNat                = "nat"
	ChainForward            = "FORWARD"
	ChainPostRouting        = "POSTROUTING"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting = "FABEDGE-POSTROUTING"
	IPSetRemotePodCIDR      = "FABEDGE-REMOTE-POD-CIDR"
	IPSetRemotePodCIDRIPV6  = "FABEDGE-REMOTE-POD-CIDR-IPV6"
)

var (
	ipt   *iptables.IPTables
	ip6t  *iptables.IPTables
	ipset = ipsetUtil.New()
)

func init() {
	var err error
	ipt, err = iptables.New()
	if err != nil {
		klog.Exit("failed to get iptables client:%s", err)
	}

	ip6t, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		klog.Exit("failed to get ip6tables client:%s", err)
	}
}

func syncForwardRules() error {
	// iptables
	err := forwardIPTablesRules(ipt)
	if err != nil {
		return err
	}

	// ip6tables
	return forwardIPTablesRules(ip6t)
}

func forwardIPTablesRules(ipts *iptables.IPTables) (err error) {
	if err = ipts.ClearChain(TableFilter, ChainFabEdgeForward); err != nil {
		return err
	}
	exists, err := ipts.Exists(TableFilter, ChainForward, "-j", ChainFabEdgeForward)
	if err != nil {
		return err
	}

	if !exists {
		if err = ipts.Insert(TableFilter, ChainForward, 1, "-j", ChainFabEdgeForward); err != nil {
			return err
		}
	}

	ipsetRemotePodCIDR := IPSetRemotePodCIDR

	if ipts.Proto() == iptables.ProtocolIPv6 {
		ipsetRemotePodCIDR = IPSetRemotePodCIDRIPV6
	}

	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetRemotePodCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = ipts.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetRemotePodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func syncPostRoutingRules() error {
	// iptables
	err := postRoutingIPTablesRules(ipt)
	if err != nil {
		return err
	}

	// ip6tables
	return postRoutingIPTablesRules(ip6t)
}

func postRoutingIPTablesRules(ipts *iptables.IPTables) (err error) {
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

	ipsetRemotePodCIDR := IPSetRemotePodCIDR

	if ipts.Proto() == iptables.ProtocolIPv6 {
		ipsetRemotePodCIDR = IPSetRemotePodCIDRIPV6
	}

	if err = ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetRemotePodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return ipts.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetRemotePodCIDR, "src", "-j", "ACCEPT")
}

func syncRemotePodCIDRSet(cp routing.ConnectorPrefixes) error {
	ipsetV4, err := ipset.EnsureIPSet(IPSetRemotePodCIDR, ipsetUtil.ProtocolFamilyIPV4, ipsetUtil.HashNet)
	if err != nil {
		return err
	}

	ipsetV6, err := ipset.EnsureIPSet(IPSetRemotePodCIDRIPV6, ipsetUtil.ProtocolFamilyIPV6, ipsetUtil.HashNet)
	if err != nil {
		return err
	}

	for _, protocolVersion := range []netutil.ProtocolVersion{netutil.IPV4, netutil.IPV6} {
		setName := IPSetRemotePodCIDR
		ipsetObj := ipsetV4
		if protocolVersion == netutil.IPV6 {

			// for debug
			continue

			setName = IPSetRemotePodCIDRIPV6
			ipsetObj = ipsetV6
		}

		// netlink.FAMILY_V4, need to check FAMILY_V6
		remoteCIDRs := sets.NewString(cp.RemotePrefixes...)

		oldRemoteCIDRs, err := ipset.ListEntries(setName, ipsetUtil.HashNet)
		if err != nil {
			return err
		}

		err = ipset.SyncIPSetEntries(ipsetObj, remoteCIDRs, oldRemoteCIDRs, ipsetUtil.HashNet)
		if err != nil {
			return err
		}
	}

	return nil
}
