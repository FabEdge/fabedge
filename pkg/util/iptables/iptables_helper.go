// Copyright 2023 FabEdge Team
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

package iptables

import (
	"bytes"
	"fmt"
	"github.com/coreos/go-iptables/iptables"
	"io"
	"os/exec"
	"strings"
)

const (
	TableFilter  = "filter"
	TableNat     = "nat"
	ChainInput   = "INPUT"
	ChainForward = "FORWARD"
)

const (
	ChainPostRouting        = "POSTROUTING"
	ChainMasquerade         = "MASQUERADE"
	ChainFabEdgeInput       = "FABEDGE-INPUT"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting = "FABEDGE-POSTROUTING"
	ChainFabEdgeNatOutgoing = "FABEDGE-NAT-OUTGOING"
)

const (
	IPTablesRestoreCommand  = "iptables-restore"
	IP6TablesRestoreCommand = "ip6tables-restore"
)

type IPTablesHelper struct {
	ipt            *iptables.IPTables
	protocol       iptables.Protocol
	restoreCommand string
	ruleSets       []IPTablesRuleSet
}

func NewIPTablesHelper() (*IPTablesHelper, error) {
	return doCreateIPTablesHelper(iptables.ProtocolIPv4)
}

func NewIP6TablesHelper() (*IPTablesHelper, error) {
	return doCreateIPTablesHelper(iptables.ProtocolIPv6)
}

func doCreateIPTablesHelper(proto iptables.Protocol) (*IPTablesHelper, error) {
	t, err := iptables.NewWithProtocol(proto)
	if err != nil {
		return nil, err
	}
	var command string
	switch proto {
	case iptables.ProtocolIPv4:
		command = IPTablesRestoreCommand
	case iptables.ProtocolIPv6:
		command = IP6TablesRestoreCommand
	}
	return &IPTablesHelper{
		ipt:            t,
		protocol:       proto,
		restoreCommand: command,
		ruleSets:       []IPTablesRuleSet{},
	}, err
}

func (h *IPTablesHelper) runRestoreCommand(args []string, stdin io.Reader) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	args = append(args, "--wait")

	cmd := exec.Command(h.restoreCommand, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = stdin

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}

func (h *IPTablesHelper) ReplaceRules() error {
	rules := h.GenerateInputFromRuleSet()

	stdout, stderr, err := h.runRestoreCommand([]string{}, bytes.NewBuffer([]byte(rules)))
	if err != nil {
		return fmt.Errorf("iptables-helper: fail to replace rules. stdout = %s; stderr = %s; error = %w", stdout, stderr, err)
	}
	return nil
}

func (h *IPTablesHelper) isInternalChain(table string, chain string) bool {
	if table == "filter" {
		if chain == "INPUT" || chain == "OUTPUT" || chain == "FORWARD" {
			return true
		}
	}
	if table == "nat" {
		if chain == "PREROUTING" || chain == "POSTROUTING" || chain == "OUTPUT" {
			return true
		}
	}
	if table == "mangle" {
		if chain == "PREROUTING" || chain == "OUTPUT" || chain == "FORWARD" || chain == "INPUT" || chain == "POSTROUTING" {
			return true
		}
	}
	if table == "raw" {
		if chain == "PREROUTING" || chain == "OUTPUT" {
			return true
		}
	}
	return false
}

func (h *IPTablesHelper) GenerateInputFromRuleSet() string {
	ret := ""
	for _, ruleSet := range h.ruleSets {
		ret += "*" + ruleSet.table + "\n"
		for _, chain := range ruleSet.chains {
			var policy string
			// For custom chains, we do not set default policy
			if h.isInternalChain(ruleSet.table, chain) {
				policy = "ACCEPT"
			} else {
				policy = "-"
			}
			ret += strings.Join([]string{":", chain, " " + policy + " [0:0]\n"}, "")
		}

		for _, ruleEntry := range ruleSet.rules {
			line := strings.Join(append([]string{"-A", ruleEntry.chain}, ruleEntry.rule...), " ")
			ret += line
			ret += "\n"
		}

		ret += "COMMIT\n"
	}
	return ret
}

func (h *IPTablesHelper) findTable(table string) int {
	for i, ruleSet := range h.ruleSets {
		if ruleSet.table == table {
			return i
		}
	}
	return -1
}

func (h *IPTablesHelper) findChain(tableIndex int, chain string) int {
	for i, elem := range h.ruleSets[tableIndex].chains {
		if chain == elem {
			return i
		}
	}
	return -1
}

func (h *IPTablesHelper) CreateChain(table string, chain string) {
	tableIndex := h.findTable(table)
	if tableIndex == -1 {
		h.ruleSets = append(h.ruleSets, IPTablesRuleSet{table: table, chains: []string{}, rules: []IPTablesRule{}})
		tableIndex = len(h.ruleSets) - 1
	}
	chainIndex := h.findChain(tableIndex, chain)
	if chainIndex == -1 {
		h.ruleSets[tableIndex].chains = append(h.ruleSets[tableIndex].chains, chain)
	}
}

func (h *IPTablesHelper) AppendUniqueRule(table string, chain string, rule ...string) {
	// Prepare chain and table if not exist
	tableIndex := h.findTable(table)
	if tableIndex == -1 {
		h.CreateChain(table, chain)
		tableIndex = h.findTable(table)
	}
	chainIndex := h.findChain(tableIndex, chain)
	if chainIndex == -1 {
		h.CreateChain(table, chain)
		chainIndex = h.findChain(tableIndex, chain)
	}

	for _, elem := range h.ruleSets[tableIndex].rules {
		if elem.chain == chain && h.rulesEqual(elem.rule, rule) {
			// Already Exist
			return
		}
	}
	h.ruleSets[tableIndex].rules = append(h.ruleSets[tableIndex].rules, IPTablesRule{chain: chain, rule: rule})
}

func (h *IPTablesHelper) rulesEqual(one, other []string) bool {
	if len(one) != len(other) {
		return false
	}
	for i, elem := range one {
		if elem != other[i] {
			return false
		}
	}
	return true
}

func (h *IPTablesHelper) ClearAllRules() {
	h.ruleSets = []IPTablesRuleSet{}
}

func (h *IPTablesHelper) CreateFabEdgePostRoutingChain() {
	h.CreateChain(TableNat, ChainFabEdgePostRouting)
}

func (h *IPTablesHelper) CreateFabEdgeInputChain() {
	h.CreateChain(TableFilter, ChainFabEdgeInput)
}

func (h *IPTablesHelper) CreateFabEdgeForwardChain() {
	h.CreateChain(TableFilter, ChainFabEdgeForward)
}

func (h *IPTablesHelper) ClearOrCreateFabEdgeNatOutgoingChain() (err error) {
	return h.ipt.ClearChain(TableNat, ChainFabEdgeNatOutgoing)
}

func (h *IPTablesHelper) checkOrCreateChain(table, chain string) error {
	exists, err := h.ipt.ChainExists(table, chain)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return h.ipt.NewChain(table, chain)
}

func (h *IPTablesHelper) CheckOrCreateFabEdgeForwardChain() (err error) {
	return h.checkOrCreateChain(TableFilter, ChainFabEdgeForward)
}

func (h *IPTablesHelper) CheckOrCreateFabEdgeNatOutgoingChain() (err error) {
	return h.checkOrCreateChain(TableNat, ChainFabEdgeNatOutgoing)
}

func (h *IPTablesHelper) NewPreparePostRoutingChain() {
	h.CreateChain(TableNat, ChainFabEdgePostRouting)
	h.AppendUniqueRule(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
}

// To remove
func (h *IPTablesHelper) PrepareForwardChain() (err error) {
	exists, err := h.ipt.Exists(TableFilter, ChainForward, "-j", ChainFabEdgeForward)
	if err != nil {
		return err
	}

	if !exists {
		if err = h.ipt.Insert(TableFilter, ChainForward, 1, "-j", ChainFabEdgeForward); err != nil {
			return err
		}
	}
	return nil
}

func (h *IPTablesHelper) NewMaintainForwardRulesForIPSet(ipsetNames []string) {
	// Prepare
	h.AppendUniqueRule(TableFilter, ChainForward, "-j", ChainFabEdgeForward)
	// Add connection track rule
	h.AppendUniqueRule(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	// Accept forward packets for ipset
	for _, ipsetName := range ipsetNames {
		h.AppendUniqueRule(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")
		h.AppendUniqueRule(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT")
	}
}

// To remove
func (h *IPTablesHelper) acceptForward(ipsetName string) (err error) {
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

// To remove
func (h *IPTablesHelper) addConnectionTrackRule() (err error) {
	if err = h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

// To remove
func (h *IPTablesHelper) MaintainForwardRulesForIPSet(ipsetNames []string) (err error) {
	if err = h.PrepareForwardChain(); err != nil {
		return err
	}

	if err = h.addConnectionTrackRule(); err != nil {
		return err
	}

	for _, ipsetName := range ipsetNames {
		if err = h.acceptForward(ipsetName); err != nil {
			return err
		}
	}
	return nil
}

func (h *IPTablesHelper) MaintainForwardRulesForSubnets(subnets []string) (err error, errRule string) {
	for _, subnet := range subnets {
		if err := h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-s", subnet, "-j", "ACCEPT"); err != nil {
			return err, fmt.Sprintf("-s %s -j ACCEPT", subnet)
		}

		if err := h.ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-d", subnet, "-j", "ACCEPT"); err != nil {
			return err, fmt.Sprintf("-d %s -j ACCEPT", subnet)
		}
	}
	return nil, ""
}

func (h *IPTablesHelper) MaintainNatOutgoingRulesForSubnets(subnets []string, ipsetName string) (err error, errRule string) {
	for _, subnet := range subnets {
		if err := h.ipt.AppendUnique(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-m", "set", "--match-set", ipsetName, "dst", "-j", "RETURN"); err != nil {
			return err, fmt.Sprintf("-s %s -m set --match-set %s dst -j RETURN", subnet, ipsetName)
		}

		if err := h.ipt.AppendUnique(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-d", subnet, "-j", "RETURN"); err != nil {
			return err, fmt.Sprintf("-s %s -d %s -j RETURN", subnet, subnet)
		}

		if err := h.ipt.AppendUnique(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-j", ChainMasquerade); err != nil {
			return err, fmt.Sprintf("-s %s -j %s", subnet, ChainMasquerade)
		}

		if err := h.ipt.AppendUnique(TableNat, ChainPostRouting, "-j", ChainFabEdgeNatOutgoing); err != nil {
			return err, fmt.Sprintf("-j %s", ChainFabEdgeNatOutgoing)
		}
	}
	return nil, ""
}

func (h *IPTablesHelper) NewAddPostRoutingRuleForKubernetes() {
	// If packets have 0x4000/0x4000 mark, then traffic should be handled by KUBE-POSTROUTING chain,
	// otherwise traffic to nodePort service, sometimes load balancer service, won't be masqueraded,
	// and this would cause response packets are dropped
	h.CreateChain(TableNat, "KUBE-POSTROUTING")
	h.AppendUniqueRule(TableNat, ChainFabEdgePostRouting, "-m", "mark", "--mark", "0x4000/0x4000", "-j", "KUBE-POSTROUTING")
}

// To remove
func (h *IPTablesHelper) AddPostRoutingRuleForKubernetes() (err error) {
	// If packets have 0x4000/0x4000 mark, then traffic should be handled by KUBE-POSTROUTING chain,
	// otherwise traffic to nodePort service, sometimes load balancer service, won't be masqueraded,
	// and this would cause response packets are dropped
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "mark", "--mark", "0x4000/0x4000", "-j", "KUBE-POSTROUTING"); err != nil {
		return err
	}
	return nil
}

func (h *IPTablesHelper) NewAddPostRoutingRulesForIPSet(ipsetName string) {
	h.AppendUniqueRule(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT")
	h.AppendUniqueRule(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")
}

// To remove
func (h *IPTablesHelper) AddPostRoutingRulesForIPSet(ipsetName string) (err error) {
	if err = h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")
}

func (h *IPTablesHelper) NewAllowIPSec() {
	h.AppendUniqueRule(TableFilter, ChainInput, "-j", ChainFabEdgeInput)
	h.AppendUniqueRule(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "500", "-j", "ACCEPT")
	h.AppendUniqueRule(TableFilter, ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "4500", "-j", "ACCEPT")
	h.AppendUniqueRule(TableFilter, ChainFabEdgeInput, "-p", "esp", "-j", "ACCEPT")
	h.AppendUniqueRule(TableFilter, ChainFabEdgeInput, "-p", "ah", "-j", "ACCEPT")
}

// To remove
func (h *IPTablesHelper) AllowIPSec() (err error) {
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

func (h *IPTablesHelper) NewAllowPostRoutingForIPSet(src, dst string) {
	h.AppendUniqueRule(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "ACCEPT")
}

// To remove
func (h *IPTablesHelper) AllowPostRoutingForIPSet(src, dst string) (err error) {
	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "ACCEPT")
}

func (h *IPTablesHelper) NewMasqueradePostRoutingForIPSet(src, dst string) {
	h.AppendUniqueRule(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "MASQUERADE")
}

// To remove
func (h *IPTablesHelper) MasqueradePostRoutingForIPSet(src, dst string) (err error) {
	return h.ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "MASQUERADE")
}
