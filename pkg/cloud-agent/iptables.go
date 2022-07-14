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
	"fmt"

	"github.com/coreos/go-iptables/iptables"
	ipsetutil "github.com/fabedge/fabedge/pkg/util/ipset"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	TableFilter             = "filter"
	TableNat                = "nat"
	ChainForward            = "FORWARD"
	ChainPostRouting        = "POSTROUTING"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting = "FABEDGE-POSTROUTING"
	IPSetRemotePodCIDR      = "FABEDGE-REMOTE-POD-CIDR"
	IPSetRemotePodCIDR6     = "FABEDGE-REMOTE-POD-CIDR6"
)

type IptablesHandler struct {
	ipt        *iptables.IPTables
	ipset      ipsetutil.Interface
	ipsetName  string
	hashFamily string
}

func newIptableHandler(version iptables.Protocol) (*IptablesHandler, error) {
	var (
		ipsetName  string
		hashFamily string
	)

	switch version {
	case iptables.ProtocolIPv4:
		ipsetName, hashFamily = IPSetRemotePodCIDR, ipsetutil.ProtocolFamilyIPV4
	case iptables.ProtocolIPv6:
		ipsetName, hashFamily = IPSetRemotePodCIDR6, ipsetutil.ProtocolFamilyIPV6
	default:
		return nil, fmt.Errorf("unknown version")
	}

	ipt, err := iptables.NewWithProtocol(version)
	if err != nil {
		return nil, err
	}

	return &IptablesHandler{
		ipt:        ipt,
		ipset:      ipsetutil.New(),
		ipsetName:  ipsetName,
		hashFamily: hashFamily,
	}, nil
}

func (h IptablesHandler) maintainRules(remotePodCIDRs []string) {
	if err := h.syncRemotePodCIDRSet(remotePodCIDRs); err != nil {
		logger.Error(err, "failed to sync ipset", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	} else {
		logger.V(5).Info("ipset is synced", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	}

	if err := h.syncForwardRules(); err != nil {
		logger.Error(err, "failed to sync iptables forward chain")
	} else {
		logger.V(5).Info("iptables forward chain is synced")
	}

	if err := h.syncPostRoutingRules(); err != nil {
		logger.Error(err, "failed to sync iptables post-routing chain")
	} else {
		logger.V(5).Info("iptables post-routing chain is synced")
	}
}

func (h IptablesHandler) syncForwardRules() (err error) {
	if err = h.ipt.ClearChain(TableFilter, ChainFabEdgeForward); err != nil {
		return err
	}
	exists, err := h.ipt.Exists(TableFilter, ChainForward, "-j", ChainFabEdgeForward)
	if err != nil {
		return err
	}

	if !exists {
		if err = h.ipt.Insert(TableFilter, ChainForward, 1, "-j", ChainFabEdgeForward); err != nil {
			return err
		}
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", h.ipsetName, "src", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", h.ipsetName, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func (h IptablesHandler) syncPostRoutingRules() (err error) {
	if err = h.ipt.ClearChain(TableNat, ChainFabEdgePostRouting); err != nil {
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

	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.ipsetName, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.ipsetName, "src", "-j", "ACCEPT")

}

func (h IptablesHandler) syncRemotePodCIDRSet(remotePodCIDRs []string) error {
	set := &ipsetutil.IPSet{
		Name:       h.ipsetName,
		HashFamily: h.hashFamily,
		SetType:    ipsetutil.HashNet,
	}

	return h.ipset.EnsureIPSet(set, sets.NewString(remotePodCIDRs...))
}
