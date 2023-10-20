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
	"bytes"
	"sync"
	"text/template"

	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/util/iptables"
)

var tmpl = template.Must(template.New("iptables").Parse(`
*filter
:FABEDGE-INPUT - [0:0]
:FABEDGE-FORWARD - [0:0]

-A FABEDGE-INPUT -p udp -m udp --dport 500 -j ACCEPT
-A FABEDGE-INPUT -p udp -m udp --dport 4500 -j ACCEPT
-A FABEDGE-INPUT -p esp -j ACCEPT
-A FABEDGE-INPUT -p ah -j ACCEPT

-A FABEDGE-FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
-A FABEDGE-FORWARD -m set --match-set {{ .LocalPodCIDR }} src -j ACCEPT
-A FABEDGE-FORWARD -m set --match-set {{ .LocalPodCIDR }} dst -j ACCEPT
-A FABEDGE-FORWARD -m set --match-set {{ .LocalNodeCIDR }} src -j ACCEPT
-A FABEDGE-FORWARD -m set --match-set {{ .LocalNodeCIDR }} dst -j ACCEPT
COMMIT

*nat
:FABEDGE-POSTROUTING - [0:0]
-A FABEDGE-POSTROUTING -m set --match-set {{ .LocalPodCIDR }} src -m set --match-set {{ .RemotePodCIDR}} dst -j ACCEPT
-A FABEDGE-POSTROUTING -m set --match-set {{ .RemotePodCIDR }} src -m set --match-set {{ .LocalPodCIDR }} dst -j ACCEPT
-A FABEDGE-POSTROUTING -m set --match-set {{ .LocalPodCIDR }} src -m set --match-set {{ .RemoteNodeCIDR }} dst -j ACCEPT
-A FABEDGE-POSTROUTING -m set --match-set {{ .RemotePodCIDR }} src -m set --match-set {{ .LocalNodeCIDR }} dst -j MASQUERADE
-A FABEDGE-POSTROUTING -m set --match-set {{ .RemoteNodeCIDR }} src -m set --match-set {{ .LocalPodCIDR}} dst -j MASQUERADE
COMMIT
`))

var jumpChains = []iptables.JumpChain{
	{Table: iptables.TableFilter, SrcChain: iptables.ChainInput, DstChain: iptables.ChainFabEdgeInput, Position: iptables.Append},
	{Table: iptables.TableFilter, SrcChain: iptables.ChainForward, DstChain: iptables.ChainFabEdgeForward, Position: iptables.Append},
	{Table: iptables.TableNat, SrcChain: iptables.ChainPostRouting, DstChain: iptables.ChainFabEdgePostRouting, Position: iptables.Prepend},
}

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
	ipt   iptables.ApplierCleaner
	ipset ipset.Interface
	log   logr.Logger

	names      ipset.IPSetNames
	hashFamily string

	rulesData []byte

	specs []IPSetSpec
	lock  sync.RWMutex
}

func newIP4TablesHandler() (*IPTablesHandler, error) {
	names := ipset.Names4
	rulesData := bytes.NewBuffer(nil)
	if err := tmpl.Execute(rulesData, names); err != nil {
		return nil, err
	}

	return &IPTablesHandler{
		log:        klogr.New().WithName("iptables-handler"),
		ipt:        iptables.NewApplierCleaner(iptables.ProtocolIPv4, jumpChains, rulesData.Bytes()),
		ipset:      ipset.New(),
		hashFamily: ipset.ProtocolFamilyIPV4,
		names:      names,
		rulesData:  rulesData.Bytes(),
	}, nil
}

func newIP6TablesHandler() (*IPTablesHandler, error) {
	names := ipset.Names6
	rulesData := bytes.NewBuffer(nil)
	if err := tmpl.Execute(rulesData, names); err != nil {
		return nil, err
	}

	return &IPTablesHandler{
		log:        klogr.New().WithName("ip6tables-handler"),
		ipt:        iptables.NewApplierCleaner(iptables.ProtocolIPv6, jumpChains, rulesData.Bytes()),
		ipset:      ipset.New(),
		hashFamily: ipset.ProtocolFamilyIPV6,
		names:      names,
	}, nil
}

func (h *IPTablesHandler) setIPSetEntrySet(edgePodCIDRSet, edgeNodeCIDRSet, cloudPodCIDRSet, cloudNodeCIDRSet sets.String) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.specs = []IPSetSpec{
		{
			Name:     h.names.RemotePodCIDR,
			EntrySet: edgePodCIDRSet,
		},
		{
			Name:     h.names.RemoteNodeCIDR,
			EntrySet: edgeNodeCIDRSet,
		},
		{
			Name:     h.names.LocalPodCIDR,
			EntrySet: cloudPodCIDRSet,
		},
		{
			Name:     h.names.LocalNodeCIDR,
			EntrySet: cloudNodeCIDRSet,
		},
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

func (h *IPTablesHandler) maintainIPTables() {
	h.maintainIPSet()

	if err := h.ipt.Apply(); err != nil {
		h.log.Error(err, "failed to restore iptables rules")
	}
}

func (h *IPTablesHandler) getEdgeNodeCIDRs() []string {
	h.lock.RLock()
	specs := h.specs
	h.lock.RUnlock()

	for _, spec := range specs {
		if spec.Name == ipset.RemoteNodeCIDR {
			return spec.EntrySet.List()
		}
	}

	return nil
}
