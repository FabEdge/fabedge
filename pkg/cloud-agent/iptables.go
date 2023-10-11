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
	"github.com/fabedge/fabedge/pkg/common/constants"
	ipsetutil "github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/pkg/util/iptables"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	IPSetRemotePodCIDR  = "FABEDGE-REMOTE-POD-CIDR"
	IPSetRemotePodCIDR6 = "FABEDGE-REMOTE-POD-CIDR6"
)

type IptablesHandler struct {
	ipset      ipsetutil.Interface
	ipsetName  string
	hashFamily string
	helper     *iptables.IPTablesHelper
}

func newIptableHandler() *IptablesHandler {
	return &IptablesHandler{
		ipset:      ipsetutil.New(),
		ipsetName:  IPSetRemotePodCIDR,
		hashFamily: ipsetutil.ProtocolFamilyIPV4,
		helper:     iptables.NewIP4TablesHelper(),
	}
}

func newIp6tableHandler() *IptablesHandler {
	return &IptablesHandler{
		ipset:      ipsetutil.New(),
		ipsetName:  IPSetRemotePodCIDR6,
		hashFamily: ipsetutil.ProtocolFamilyIPV6,
		helper:     iptables.NewIP6TablesHelper(),
	}
}

func (h IptablesHandler) maintainRules(remotePodCIDRs []string) {
	if err := h.syncRemotePodCIDRSet(remotePodCIDRs); err != nil {
		logger.Error(err, "failed to sync ipset", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	} else {
		logger.V(5).Info("ipset is synced", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	}

	h.helper.ClearAllRules()
	h.syncForwardRules()
	h.syncPostRoutingRules()

	if err := h.helper.ReplaceRules(); err != nil {
		logger.Error(err, "failed to sync iptables rules")
	} else {
		logger.V(5).Info("iptables rules is synced")
	}

}

func (h IptablesHandler) syncForwardRules() {
	h.helper.PrepareForwardChain()
	h.helper.MaintainForwardRulesForIPSet([]string{h.ipsetName})
}

func (h IptablesHandler) syncPostRoutingRules() {
	h.helper.PreparePostRoutingChain()

	// If packets have 0x4000/0x4000 mark, then traffic should be handled by KUBE-POSTROUTING chain,
	// otherwise traffic to nodePort service, sometimes load balancer service, won't be masqueraded,
	// and this would cause response packets are dropped
	h.helper.CreateChain(constants.TableNat, "KUBE-POSTROUTING")
	h.helper.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "mark", "--mark", "0x4000/0x4000", "-j", "KUBE-POSTROUTING")

	h.helper.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", h.ipsetName, "dst", "-j", "ACCEPT")
	h.helper.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", h.ipsetName, "src", "-j", "ACCEPT")

}

func (h IptablesHandler) syncRemotePodCIDRSet(remotePodCIDRs []string) error {
	set := &ipsetutil.IPSet{
		Name:       h.ipsetName,
		HashFamily: h.hashFamily,
		SetType:    ipsetutil.HashNet,
	}

	return h.ipset.EnsureIPSet(set, sets.NewString(remotePodCIDRs...))
}

func (h IptablesHandler) clearRules() error {
	h.helper.ClearAllRules()
	h.helper.CreateFabEdgePostRoutingChain()
	h.helper.CreateFabEdgeForwardChain()
	return h.helper.ReplaceRules()
}
