// Copyright 2023 FabEdge Team
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

package iptables

import (
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	"sync"
)

const (
	TableFilter  = "filter"
	TableNat     = "nat"
	ChainInput   = "INPUT"
	ChainForward = "FORWARD"
)

const (
	ChainPostRouting        = "POSTROUTING"
	ChainMasquerade         = "MASQUERADE"
	ChainFabEdgeInput       = "FABEDGE-INPUT"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting = "FABEDGE-POSTROUTING"
	ChainFabEdgeNatOutgoing = "FABEDGE-NAT-OUTGOING"
)

const (
	IPTablesRestoreCommand  = "iptables-restore"
	IP6TablesRestoreCommand = "ip6tables-restore"
)

type IPTablesHelper struct {
	ipt   *iptables.IPTables
	Mutex sync.RWMutex
}

func NewIPTablesHelper() (*IPTablesHelper, error) {
	return doCreateIPTablesHelper(iptables.ProtocolIPv4)
}

func NewIP6TablesHelper() (*IPTablesHelper, error) {
	return doCreateIPTablesHelper(iptables.ProtocolIPv6)
}

func doCreateIPTablesHelper(protocol iptables.Protocol) (*IPTablesHelper, error) {
	t, err := iptables.NewWithProtocol(protocol)
	if err != nil {
		return nil, err
	}
	return &IPTablesHelper{
		ipt: t,
	}, err
}

func (h *IPTablesHelper) ClearOrCreateFabEdgePostRoutingChain() (err error) {
	return h.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (h *IPTablesHelper) ClearOrCreateFabEdgeInputChain() (err error) {
	return h.ipt.ClearChain(TableFilter, ChainFabEdgeInput)
}

func (h *IPTablesHelper) ClearOrCreateFabEdgeForwardChain() (err error) {
	return h.ipt.ClearChain(TableFilter, ChainFabEdgeForward)
}

func (h *IPTablesHelper) ClearOrCreateFabEdgeNatOutgoingChain() (err error) {
	return h.ipt.ClearChain(TableNat, ChainFabEdgeNatOutgoing)
}

func (h *IPTablesHelper) checkOrCreateChain(table, chain string) error {
	exists, err := h.ipt.ChainExists(table, chain)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return h.ipt.NewChain(table, chain)
}

func (h *IPTablesHelper) CheckOrCreateFabEdgeForwardChain() (err error) {
	return h.checkOrCreateChain(TableFilter, ChainFabEdgeForward)
}

func (h *IPTablesHelper) CheckOrCreateFabEdgeNatOutgoingChain() (err error) {
	return h.checkOrCreateChain(TableNat, ChainFabEdgeNatOutgoing)
}

func (h *IPTablesHelper) PreparePostRoutingChain() (err error) {
	if err = h.ClearOrCreateFabEdgePostRoutingChain(); err != nil {
		return err
	}
	exists, err := h.ipt.Exists(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	if err != nil {
		return err
	}

	if !exists {
		if err = h.ipt.Insert(TableNat, ChainPostRouting, 1, "-j", ChainFabEdgePostRouting); err != nil {
			return err
		}
	}
	return nil
}

func (h *IPTablesHelper) PrepareForwardChain() (err error) {
	exists, err := h.ipt.Exists(TableFilter, ChainForward, "-j", ChainFabEdgeForward)
	if err != nil {
		return err
	}

	if !exists {
		if err = h.ipt.Insert(TableFilter, ChainForward, 1, "-j", ChainFabEdgeForward); err != nil {
			return err
		}
	}
	return nil
}

func (h *IPTablesHelper) acceptForward(ipsetName string) (err error) {
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func (h *IPTablesHelper) addConnectionTrackRule() (err error) {
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func (h *IPTablesHelper) MaintainForwardRulesForIPSet(ipsetNames []string) (err error) {
	if err = h.PrepareForwardChain(); err != nil {
		return err
	}

	if err = h.addConnectionTrackRule(); err != nil {
		return err
	}

	for _, ipsetName := range ipsetNames {
		if err = h.acceptForward(ipsetName); err != nil {
			return err
		}
	}
	return nil
}

func (h *IPTablesHelper) MaintainForwardRulesForSubnets(subnets []string) (err error, errRule string) {
	for _, subnet := range subnets {
		if err := h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-s", subnet, "-j", "ACCEPT"); err != nil {
			return err, fmt.Sprintf("-s %s -j ACCEPT", subnet)
		}

		if err := h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-d", subnet, "-j", "ACCEPT"); err != nil {
			return err, fmt.Sprintf("-d %s -j ACCEPT", subnet)
		}
	}
	return nil, ""
}

func (h *IPTablesHelper) MaintainNatOutgoingRulesForSubnets(subnets []string, ipsetName string) (err error, errRule string) {
	for _, subnet := range subnets {
		if err := h.ipt.AppendUnique(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-m", "set", "--match-set", ipsetName, "dst", "-j", "RETURN"); err != nil {
			return err, fmt.Sprintf("-s %s -m set --match-set %s dst -j RETURN", subnet, ipsetName)
		}

		if err := h.ipt.AppendUnique(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-d", subnet, "-j", "RETURN"); err != nil {
			return err, fmt.Sprintf("-s %s -d %s -j RETURN", subnet, subnet)
		}

		if err := h.ipt.AppendUnique(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-j", ChainMasquerade); err != nil {
			return err, fmt.Sprintf("-s %s -j %s", subnet, ChainMasquerade)
		}

		if err := h.ipt.AppendUnique(TableNat, ChainPostRouting, "-j", ChainFabEdgeNatOutgoing); err != nil {
			return err, fmt.Sprintf("-j %s", ChainFabEdgeNatOutgoing)
		}
	}
	return nil, ""
}

func (h *IPTablesHelper) AddPostRoutingRuleForKubernetes() (err error) {
	// If packets have 0x4000/0x4000 mark, then traffic should be handled by KUBE-POSTROUTING chain,
	// otherwise traffic to nodePort service, sometimes load balancer service, won't be masqueraded,
	// and this would cause response packets are dropped
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "mark", "--mark", "0x4000/0x4000", "-j", "KUBE-POSTROUTING"); err != nil {
		return err
	}
	return nil
}

func (h *IPTablesHelper) AddPostRoutingRulesForIPSet(ipsetName string) (err error) {
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")
}

func (h *IPTablesHelper) AllowIPSec() (err error) {
	if err = h.ipt.AppendUnique(TableFilter, ChainInput, "-j", ChainFabEdgeInput); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "500", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "4500", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "esp", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeInput, "-p", "ah", "-j", "ACCEPT"); err != nil {
		return err
	}
	return nil
}

func (h *IPTablesHelper) AllowPostRoutingForIPSet(src, dst string) (err error) {
	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "ACCEPT")
}

func (h *IPTablesHelper) MasqueradePostRoutingForIPSet(src, dst string) (err error) {
	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "MASQUERADE")
}
