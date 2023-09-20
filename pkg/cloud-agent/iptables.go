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

	h.helper.Mutex.Lock()
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
	h.helper.Mutex.Unlock()
}

func (h IptablesHandler) syncForwardRules() (err error) {
	if err = h.helper.ClearOrCreateFabEdgeForwardChain(); err != nil {
		return err
	}

	if err = h.helper.MaintainForwardRulesForIPSet([]string{h.ipsetName}); err != nil {
		return err
	}

	return nil
}

func (h IptablesHandler) syncPostRoutingRules() (err error) {
	if err = h.helper.PreparePostRoutingChain(); err != nil {
		return err
	}

	if err = h.helper.AddPostRoutingRuleForKubernetes(); err != nil {
		return err
	}

	return h.helper.AddPostRoutingRulesForIPSet(h.ipsetName)
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
	h.helper.Mutex.Lock()
	defer h.helper.Mutex.Unlock()

	if err := h.helper.ClearOrCreateFabEdgePostRoutingChain(); err != nil {
		return err
	}

	return h.helper.ClearOrCreateFabEdgeForwardChain()
}
