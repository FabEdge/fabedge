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
	"bytes"
	"text/template"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/util/ipset"
	ipsetutil "github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/pkg/util/iptables"
)

type IptablesHandler struct {
	ipsetName  string
	hashFamily string
	ipset      ipsetutil.Interface

	helper    iptables.ApplierCleaner
	rulesData []byte
}

func newIptableHandler() (*IptablesHandler, error) {
	names := ipset.Names4
	rulesData := bytes.NewBuffer(nil)
	if err := tmpl.Execute(rulesData, names); err != nil {
		return nil, err
	}

	return &IptablesHandler{
		ipset:      ipsetutil.New(),
		ipsetName:  ipset.RemotePodCIDR,
		hashFamily: ipsetutil.ProtocolFamilyIPV4,
		helper:     iptables.NewApplierCleaner(iptables.ProtocolIPv4, jumpChains, rulesData.Bytes()),
		rulesData:  rulesData.Bytes(),
	}, nil
}

func newIp6tableHandler() (*IptablesHandler, error) {
	names := ipset.Names6
	rulesData := bytes.NewBuffer(nil)
	if err := tmpl.Execute(rulesData, names); err != nil {
		return nil, err
	}

	return &IptablesHandler{
		ipset:      ipsetutil.New(),
		ipsetName:  ipset.RemotePodCIDR6,
		hashFamily: ipsetutil.ProtocolFamilyIPV6,
		helper:     iptables.NewApplierCleaner(iptables.ProtocolIPv6, jumpChains, rulesData.Bytes()),
		rulesData:  rulesData.Bytes(),
	}, nil
}

var tmpl = template.Must(template.New("iptables").Parse(`
*filter
:FABEDGE-FORWARD - [0:0]

-A FABEDGE-FORWARD -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
-A FABEDGE-FORWARD -m set --match-set {{ .RemotePodCIDR }} src -j ACCEPT
-A FABEDGE-FORWARD -m set --match-set {{ .RemotePodCIDR }} dst -j ACCEPT
COMMIT

*nat
:FABEDGE-POSTROUTING - [0:0]
-A FABEDGE-POSTROUTING -m mark --mark 0x4000/0x4000 -j KUBE-POSTROUTING
-A FABEDGE-POSTROUTING -m set --match-set {{ .RemotePodCIDR }} dst -j ACCEPT
-A FABEDGE-POSTROUTING -m set --match-set {{ .RemotePodCIDR }} src -j ACCEPT
COMMIT
`))

var jumpChains = []iptables.JumpChain{
	{Table: iptables.TableFilter, SrcChain: iptables.ChainForward, DstChain: iptables.ChainFabEdgeForward, Position: iptables.Append},
	{Table: iptables.TableNat, SrcChain: iptables.ChainPostRouting, DstChain: iptables.ChainFabEdgePostRouting, Position: iptables.Prepend},
}

func (h IptablesHandler) maintainRules(remotePodCIDRs []string) {
	if err := h.ensureIPSet(remotePodCIDRs); err != nil {
		logger.Error(err, "failed to sync ipset", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	} else {
		logger.V(5).Info("ipset is synced", "setName", h.ipsetName, "remotePodCIDRs", remotePodCIDRs)
	}

	if err := h.helper.Apply(); err != nil {
		logger.Error(err, "failed to sync iptables rules")
	} else {
		logger.V(5).Info("iptables rules is synced")
	}
}

func (h IptablesHandler) ensureIPSet(remotePodCIDRs []string) error {
	set := &ipsetutil.IPSet{
		Name:       h.ipsetName,
		HashFamily: h.hashFamily,
		SetType:    ipsetutil.HashNet,
	}

	return h.ipset.EnsureIPSet(set, sets.NewString(remotePodCIDRs...))
}

func (h IptablesHandler) clearRules() error {
	return h.helper.Remove()
}
