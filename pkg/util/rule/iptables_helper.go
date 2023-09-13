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

package rule

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

type IPTablesHelper struct {
	ipt   *iptables.IPTables
	mutex sync.Mutex
}

func NewIPTablesHelper(t *iptables.IPTables) *IPTablesHelper {
	return &IPTablesHelper{
		ipt: t,
	}
}

func (h *IPTablesHelper) ClearFabEdgePostRouting() (err error) {
	return h.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (h *IPTablesHelper) ClearFabEdgeForward() (err error) {
	return h.ipt.ClearChain(TableFilter, ChainFabEdgeForward)
}

func (h *IPTablesHelper) PreparePostRoutingChain() (err error) {
	if err = h.ClearFabEdgePostRouting(); err != nil {
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

func (h *IPTablesHelper) prepareForwardChain() (err error) {
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

func (h *IPTablesHelper) MaintainForwardRules(ipsetNames []string) (err error) {
	if err = h.prepareForwardChain(); err != nil {
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
