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
	h.helper.ClearAllRules()
	h.helper.CreateFabEdgeInputChain()
	h.helper.CreateFabEdgeForwardChain()
	h.helper.CreateFabEdgePostRoutingChain()
	return h.helper.ReplaceRules()
}

func (h *IPTablesHandler) maintainIPTables() {
	h.helper.ClearAllRules()

	// ensureForwardIPTablesRules
	// ensure rules exist
	h.helper.NewMaintainForwardRulesForIPSet([]string{h.names.CloudPodCIDR, h.names.CloudNodeCIDR})

	// ensureNatIPTablesRules
	h.helper.NewPreparePostRoutingChain()

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	h.helper.NewAllowPostRoutingForIPSet(h.names.CloudPodCIDR, h.names.EdgePodCIDR)

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	h.helper.NewAllowPostRoutingForIPSet(h.names.EdgePodCIDR, h.names.CloudPodCIDR)

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	h.helper.NewAllowPostRoutingForIPSet(h.names.CloudPodCIDR, h.names.EdgeNodeCIDR)

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	h.helper.NewMasqueradePostRoutingForIPSet(h.names.EdgePodCIDR, h.names.CloudNodeCIDR)

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	h.helper.NewMasqueradePostRoutingForIPSet(h.names.EdgeNodeCIDR, h.names.CloudPodCIDR)

	// ensureIPSpecInputRules
	h.helper.NewAllowIPSec()

	if err := h.helper.ReplaceRules(); err != nil {
		h.log.Error(err, "failed to sync iptables rules")
	} else {
		h.log.V(5).Info("iptables rules is synced")
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
