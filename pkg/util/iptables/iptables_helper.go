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
	"github.com/fabedge/fabedge/pkg/common/constants"
	"io"
	"os/exec"
	"strings"
)

const (
	IPTablesRestoreCommand  = "iptables-restore"
	IP6TablesRestoreCommand = "ip6tables-restore"
)

const (
	ProtocolIPv4 = "ipv4"
	ProtocolIPv6 = "ipv6"
)

type Protocol string

type IPTablesHelper struct {
	protocol       Protocol
	restoreCommand string
	ruleSets       []IPTablesRuleSet
}

func NewIPTablesHelper() *IPTablesHelper {
	return doCreateIPTablesHelper(ProtocolIPv4)
}

func NewIP6TablesHelper() *IPTablesHelper {
	return doCreateIPTablesHelper(ProtocolIPv6)
}

func doCreateIPTablesHelper(proto Protocol) *IPTablesHelper {
	var command string
	switch proto {
	case ProtocolIPv4:
		command = IPTablesRestoreCommand
	case ProtocolIPv6:
		command = IP6TablesRestoreCommand
	}
	return &IPTablesHelper{
		protocol:       proto,
		restoreCommand: command,
		ruleSets:       []IPTablesRuleSet{},
	}
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
	h.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
}

func (h *IPTablesHelper) CreateFabEdgeForwardChain() {
	h.CreateChain(constants.TableFilter, constants.ChainFabEdgeForward)
}

func (h *IPTablesHelper) PreparePostRoutingChain() {
	h.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	h.AppendUniqueRule(constants.TableNat, constants.ChainPostRouting, "-j", constants.ChainFabEdgePostRouting)
}

func (h *IPTablesHelper) PrepareForwardChain() {
	h.CreateChain(constants.TableFilter, constants.ChainFabEdgeForward)
	h.AppendUniqueRule(constants.TableFilter, constants.ChainForward, "-j", constants.ChainFabEdgeForward)
}

func (h *IPTablesHelper) MaintainForwardRulesForIPSet(ipsetNames []string) {
	// Add connection track rule
	h.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	// Accept forward packets for ipset
	for _, ipsetName := range ipsetNames {
		h.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")
		h.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeForward, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT")
	}
}
