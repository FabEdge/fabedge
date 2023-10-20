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

package agent

import (
	"bytes"
	"strings"
	"text/template"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/pkg/util/iptables"
)

type IPSet struct {
	IPSet    *ipset.IPSet
	EntrySet sets.String
}

var tmpl = template.Must(template.New("iptables").Parse(`
*filter
:FABEDGE-FORWARD - [0:0]
{{- range $cidr := .edgePodCIDRs }}
-A FABEDGE-FORWARD -s {{ $cidr }} -j ACCEPT
-A FABEDGE-FORWARD -d {{ $cidr }} -j ACCEPT
{{- end }}
COMMIT

*nat
:FABEDGE-POSTROUTING - [0:0]
{{- range $cidr := .edgePodCIDRs }}
-A FABEDGE-POSTROUTING -s {{ $cidr }} -m set --match-set {{ $.ipsetName }} dst -j RETURN
-A FABEDGE-POSTROUTING -s {{ $cidr }} -d {{ $cidr }} -j RETURN
-A FABEDGE-POSTROUTING -s {{ $cidr }} -j MASQUERADE
{{- end }}
COMMIT
`))

var jumpChains = []iptables.JumpChain{
	{Table: iptables.TableFilter, SrcChain: iptables.ChainForward, DstChain: iptables.ChainFabEdgeForward, Position: iptables.Append},
	{Table: iptables.TableNat, SrcChain: iptables.ChainPostRouting, DstChain: iptables.ChainFabEdgePostRouting, Position: iptables.Prepend},
}

func buildRuleData(ipsetName string, edgePodCIDRs []string) []byte {
	buf := bytes.NewBuffer(nil)

	_ = tmpl.Execute(buf, map[string]interface{}{
		"ipsetName":    ipsetName,
		"edgePodCIDRs": edgePodCIDRs,
	})

	return buf.Bytes()
}

func (m *Manager) ensureIPTablesRules() error {
	current := m.getCurrentEndpoint()

	peerIPSet4, peerIPSet6 := m.getAllPeerCIDRs()
	subnetsIP4, subnetsIP6 := classifySubnets(current.Subnets)

	if !areSubnetsEqual(current.Subnets, m.lastSubnets) {
		m.ipt = iptables.NewApplierCleaner(iptables.ProtocolIPv4, jumpChains, buildRuleData(ipset.RemoteCIDR, subnetsIP4))
		m.ipt6 = iptables.NewApplierCleaner(iptables.ProtocolIPv6, jumpChains, buildRuleData(ipset.RemoteCIDR6, subnetsIP6))
		m.lastSubnets = current.Subnets
	}

	configs := []struct {
		name       string
		hashFamily string
		peerIPSet  sets.String
		ipt        iptables.ApplierCleaner
	}{
		{ipset.RemoteCIDR, ipset.ProtocolFamilyIPV4, peerIPSet4, m.ipt},
		{ipset.RemoteCIDR6, ipset.ProtocolFamilyIPV6, peerIPSet6, m.ipt6},
	}

	var errors []error
	for _, c := range configs {
		ipSet := &ipset.IPSet{
			Name:       c.name,
			HashFamily: c.hashFamily,
			SetType:    ipset.HashNet,
		}

		if err := m.ipset.EnsureIPSet(ipSet, c.peerIPSet); err != nil {
			m.log.Error(err, "failed to sync ipset", "ipsetName", c.name)
			errors = append(errors, err)
		}

		if err := c.ipt.Apply(); err != nil {
			m.log.Error(err, "failed to sync iptables rules")
			errors = append(errors, err)
		} else {
			m.log.V(5).Info("iptables rules is synced")
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (m *Manager) getAllPeerCIDRs() (cidrSet4, cidrSet6 sets.String) {
	cidrSet4, cidrSet6 = sets.NewString(), sets.NewString()

	for _, peer := range m.getPeerEndpoints() {
		for _, nodeSubnet := range peer.NodeSubnets {
			if isIPv6(nodeSubnet) {
				cidrSet6.Insert(nodeSubnet)
			} else {
				cidrSet4.Insert(nodeSubnet)
			}
		}

		for _, subnet := range peer.Subnets {
			if isIPv6(subnet) {
				cidrSet6.Insert(subnet)
			} else {
				cidrSet4.Insert(subnet)
			}
		}
	}

	return cidrSet4, cidrSet6
}

func classifySubnets(subnets []string) (ipv4, ipv6 []string) {
	for _, subnet := range subnets {
		if isIPv6(subnet) {
			ipv6 = append(ipv6, subnet)
		} else {
			ipv4 = append(ipv4, subnet)
		}
	}

	return ipv4, ipv6
}

func isIPv6(addr string) bool {
	return strings.Index(addr, ":") != -1
}

func areSubnetsEqual(sa1, sa2 []string) bool {
	if len(sa1) != len(sa2) {
		return false
	}

	for i := range sa1 {
		if sa1[i] != sa2[i] {
			return false
		}
	}

	return true
}
