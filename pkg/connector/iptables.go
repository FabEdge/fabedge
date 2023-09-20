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
	"sync"

	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/pkg/util/iptables"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
)

const (
	IPSetEdgePodCIDR    = "FABEDGE-EDGE-POD-CIDR"
	IPSetEdgePodCIDR6   = "FABEDGE-EDGE-POD-CIDR6"
	IPSetEdgeNodeCIDR   = "FABEDGE-EDGE-NODE-CIDR"
	IPSetEdgeNodeCIDR6  = "FABEDGE-EDGE-NODE-CIDR6"
	IPSetCloudPodCIDR   = "FABEDGE-CLOUD-POD-CIDR"
	IPSetCloudPodCIDR6  = "FABEDGE-CLOUD-POD-CIDR6"
	IPSetCloudNodeCIDR  = "FABEDGE-CLOUD-NODE-CIDR"
	IPSetCloudNodeCIDR6 = "FABEDGE-CLOUD-NODE-CIDR6"
)

type IPSetSpec struct {
	Name     string
	EntrySet sets.String
}

type IPSetNames struct {
	EdgePodCIDR   string
	EdgeNodeCIDR  string
	CloudPodCIDR  string
	CloudNodeCIDR string
}

type IPTablesHandler struct {
	ipset ipset.Interface
	log   logr.Logger

	names      IPSetNames
	hashFamily string

	specs []IPSetSpec
	lock  sync.RWMutex

	helper *iptables.IPTablesHelper
}

func newIP4TablesHandler() (*IPTablesHandler, error) {
	iptHelper, err := iptables.NewIPTablesHelper()
	if err != nil {
		return nil, err
	}

	return &IPTablesHandler{
		log:        klogr.New().WithName("iptablesHandler"),
		ipset:      ipset.New(),
		hashFamily: ipset.ProtocolFamilyIPV4,
		names: IPSetNames{
			EdgeNodeCIDR:  IPSetEdgeNodeCIDR,
			EdgePodCIDR:   IPSetEdgePodCIDR,
			CloudPodCIDR:  IPSetCloudPodCIDR,
			CloudNodeCIDR: IPSetCloudNodeCIDR,
		},
		helper: iptHelper,
	}, nil
}

func newIP6TablesHandler() (*IPTablesHandler, error) {
	iptHelper, err := iptables.NewIP6TablesHelper()
	if err != nil {
		return nil, err
	}

	return &IPTablesHandler{
		log:        klogr.New().WithName("ip6tablesHandler"),
		ipset:      ipset.New(),
		hashFamily: ipset.ProtocolFamilyIPV6,
		names: IPSetNames{
			EdgeNodeCIDR:  IPSetEdgeNodeCIDR6,
			EdgePodCIDR:   IPSetEdgePodCIDR6,
			CloudPodCIDR:  IPSetCloudPodCIDR6,
			CloudNodeCIDR: IPSetCloudNodeCIDR6,
		},
		helper: iptHelper,
	}, nil
}

func (h *IPTablesHandler) setIPSetEntrySet(edgePodCIDRSet, edgeNodeCIDRSet, cloudPodCIDRSet, cloudNodeCIDRSet sets.String) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.specs = []IPSetSpec{
		{
			Name:     h.names.EdgePodCIDR,
			EntrySet: edgePodCIDRSet,
		},
		{
			Name:     h.names.EdgeNodeCIDR,
			EntrySet: edgeNodeCIDRSet,
		},
		{
			Name:     h.names.CloudPodCIDR,
			EntrySet: cloudPodCIDRSet,
		},
		{
			Name:     h.names.CloudNodeCIDR,
			EntrySet: cloudNodeCIDRSet,
		},
	}
}

func (h *IPTablesHandler) clearFabEdgeIptablesChains() error {
	err := h.helper.ClearOrCreateFabEdgeInputChain()
	if err != nil {
		return err
	}
	err = h.helper.ClearOrCreateFabEdgeForwardChain()
	if err != nil {
		return err
	}
	return h.helper.ClearOrCreateFabEdgePostRoutingChain()
}

func (h *IPTablesHandler) ensureForwardIPTablesRules() (err error) {
	// ensure rules exist
	if err = h.helper.MaintainForwardRulesForIPSet([]string{h.names.CloudPodCIDR, h.names.CloudNodeCIDR}); err != nil {
		return err
	}

	return nil
}

func (h *IPTablesHandler) ensureNatIPTablesRules() (err error) {
	if err = h.helper.PreparePostRoutingChain(); err != nil {
		return err
	}

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	if err = h.helper.AllowPostRoutingForIPSet(h.names.CloudPodCIDR, h.names.EdgePodCIDR); err != nil {
		return err
	}

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	if err = h.helper.AllowPostRoutingForIPSet(h.names.EdgePodCIDR, h.names.CloudPodCIDR); err != nil {
		return err
	}

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	if err = h.helper.AllowPostRoutingForIPSet(h.names.CloudPodCIDR, h.names.EdgeNodeCIDR); err != nil {
		return err
	}

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	if err = h.helper.MasqueradePostRoutingForIPSet(h.names.EdgePodCIDR, h.names.CloudNodeCIDR); err != nil {
		return err
	}

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	return h.helper.MasqueradePostRoutingForIPSet(h.names.EdgeNodeCIDR, h.names.CloudPodCIDR)
}

func (h *IPTablesHandler) ensureIPSpecInputRules() (err error) {
	return h.helper.AllowIPSec()
}

func (h *IPTablesHandler) maintainIPTables() {
	if err := h.ensureForwardIPTablesRules(); err != nil {
		h.log.Error(err, "failed to add iptables forward rules")
	} else {
		h.log.V(5).Info("iptables forward rules are added")
	}

	if err := h.ensureNatIPTablesRules(); err != nil {
		h.log.Error(err, "failed to add iptables nat rules")
	} else {
		h.log.V(5).Info("iptables nat rules are added")
	}

	if err := h.ensureIPSpecInputRules(); err != nil {
		h.log.Error(err, "failed to add iptables input rules")
	} else {
		h.log.V(5).Info("iptables input rules are added")
	}
}

func (h *IPTablesHandler) maintainIPSet() {
	var specs []IPSetSpec

	h.lock.RLock()
	specs = h.specs
	h.lock.RUnlock()

	for _, spec := range specs {
		set := &ipset.IPSet{
			Name:       spec.Name,
			HashFamily: h.hashFamily,
			SetType:    ipset.HashNet,
		}

		if err := h.ipset.EnsureIPSet(set, spec.EntrySet); err != nil {
			h.log.Error(err, "failed to sync ipset", "name", spec.Name)
		} else {
			h.log.V(5).Info("ipset are synced", "name", spec.Name)
		}
	}
}
