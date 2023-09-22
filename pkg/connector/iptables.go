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

	"github.com/coreos/go-iptables/iptables"
	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
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
	ipt   *iptables.IPTables
	ipset ipset.Interface
	log   logr.Logger

	names      IPSetNames
	hashFamily string

	specs []IPSetSpec
	lock  sync.RWMutex
}

func newIP4TablesHandler() (*IPTablesHandler, error) {
	ipt, err := iptables.New()
	if err != nil {
		return nil, err
	}

	return &IPTablesHandler{
		log:        klogr.New().WithName("iptablesHandler"),
		ipt:        ipt,
		ipset:      ipset.New(),
		hashFamily: ipset.ProtocolFamilyIPV4,
		names: IPSetNames{
			EdgeNodeCIDR:  IPSetEdgeNodeCIDR,
			EdgePodCIDR:   IPSetEdgePodCIDR,
			CloudPodCIDR:  IPSetCloudPodCIDR,
			CloudNodeCIDR: IPSetCloudNodeCIDR,
		},
	}, nil
}

func newIP6TablesHandler() (*IPTablesHandler, error) {
	ipt, err := iptables.New(iptables.IPFamily(iptables.ProtocolIPv6))
	if err != nil {
		return nil, err
	}

	return &IPTablesHandler{
		log:        klogr.New().WithName("ip6tablesHandler"),
		ipt:        ipt,
		ipset:      ipset.New(),
		hashFamily: ipset.ProtocolFamilyIPV6,
		names: IPSetNames{
			EdgeNodeCIDR:  IPSetEdgeNodeCIDR6,
			EdgePodCIDR:   IPSetEdgePodCIDR6,
			CloudPodCIDR:  IPSetCloudPodCIDR6,
			CloudNodeCIDR: IPSetCloudNodeCIDR6,
		},
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

func (h *IPTablesHandler) CleanSNatIPTablesRules() error {
	return h.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (h *IPTablesHandler) clearFabEdgeIptablesChains() error {
	err := h.ipt.ClearChain(TableFilter, ChainFabEdgeInput)
	if err != nil {
		return err
	}
	err = h.ipt.ClearChain(TableFilter, ChainFabEdgeForward)
	if err != nil {
		return err
	}
	return h.ipt.ClearChain(TableNat, ChainFabEdgePostRouting)
}

func (h *IPTablesHandler) ensureForwardIPTablesRules() (err error) {
	// ensure rules exist
	if err = h.ipt.AppendUnique(TableFilter, ChainForward, "-j", ChainFabEdgeForward); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", h.names.CloudPodCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", h.names.CloudPodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", h.names.CloudNodeCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", h.names.CloudNodeCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func (h *IPTablesHandler) ensureNatIPTablesRules() (err error) {
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

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.names.CloudPodCIDR, "src", "-m", "set", "--match-set", h.names.EdgePodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.names.EdgePodCIDR, "src", "-m", "set", "--match-set", h.names.CloudPodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.names.CloudPodCIDR, "src", "-m", "set", "--match-set", h.names.EdgeNodeCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.names.EdgePodCIDR, "src", "-m", "set", "--match-set", h.names.CloudNodeCIDR, "dst", "-j", "MASQUERADE"); err != nil {
		return err
	}

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", h.names.EdgeNodeCIDR, "src", "-m", "set", "--match-set", h.names.CloudPodCIDR, "dst", "-j", "MASQUERADE")

}

func (h *IPTablesHandler) ensureIPSpecInputRules() (err error) {
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

func (h *IPTablesHandler) getEdgeNodeCIDRs() []string {
	h.lock.RLock()
	specs := h.specs
	h.lock.RUnlock()

	for _, spec := range specs {
		if spec.Name == IPSetEdgeNodeCIDR {
			return spec.EntrySet.List()
		}
	}

	return nil
}
