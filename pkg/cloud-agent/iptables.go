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

func newIptableHandler() (*IptablesHandler, error) {
	iptHelper, err := iptables.NewIPTablesHelper()
	if err != nil {
		return nil, err
	}

	return &IptablesHandler{
		ipset:      ipsetutil.New(),
		ipsetName:  IPSetRemotePodCIDR,
		hashFamily: ipsetutil.ProtocolFamilyIPV4,
		helper:     iptHelper,
	}, nil
}

func newIp6tableHandler() (*IptablesHandler, error) {
	iptHelper, err := iptables.NewIP6TablesHelper()
	if err != nil {
		return nil, err
	}

	return &IptablesHandler{
		ipset:      ipsetutil.New(),
		ipsetName:  IPSetRemotePodCIDR6,
		hashFamily: ipsetutil.ProtocolFamilyIPV6,
		helper:     iptHelper,
	}, nil
}

func (h IptablesHandler) maintainRules(remotePodCIDRs []string) {
	if err := h.syncRemotePodCIDRSet(remotePodCIDRs); err != nil {
		logger.Error(err, "failed to sync ipset", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	} else {
		logger.V(5).Info("ipset is synced", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	}

	h.helper.ClearAllRules()
	h.helper.CreateFabEdgeForwardChain()
	h.helper.NewMaintainForwardRulesForIPSet([]string{h.ipsetName})
	h.helper.NewPreparePostRoutingChain()
	h.helper.NewAddPostRoutingRuleForKubernetes()
	h.helper.NewAddPostRoutingRulesForIPSet(h.ipsetName)

	if err := h.helper.ReplaceRules(); err != nil {
		logger.Error(err, "failed to sync iptables rules")
	} else {
		logger.V(5).Info("iptables rules is synced")
	}
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
