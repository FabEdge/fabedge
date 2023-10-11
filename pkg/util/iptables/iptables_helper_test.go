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
	"strings"
	"testing"
)

func TestCreateChain(t *testing.T) {
	ipt := NewIP4TablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(TableNat, ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", ChainFabEdgePostRouting, " - [0:0]\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateInternalChain(t *testing.T) {
	ipt := NewIP4TablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(TableNat, ChainPostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", ChainPostRouting, " ACCEPT [0:0]\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateDuplicatedChains(t *testing.T) {
	ipt := NewIP4TablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(TableNat, ChainFabEdgePostRouting)
	ipt.CreateChain(TableNat, ChainFabEdgePostRouting)
	ipt.CreateChain(TableNat, ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", ChainFabEdgePostRouting, " - [0:0]\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateChainAndAppendRule(t *testing.T) {
	ipt := NewIP4TablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(TableNat, ChainFabEdgePostRouting)
	ipt.CreateChain(TableNat, ChainPostRouting)
	ipt.AppendUniqueRule(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", ChainFabEdgePostRouting, " - [0:0]\n", ":", ChainPostRouting, " ACCEPT [0:0]\n",
		"-A ", ChainPostRouting, " -j ", ChainFabEdgePostRouting, "\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateChainAndAppendDuplicatedRules(t *testing.T) {
	ipt := NewIP4TablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(TableNat, ChainFabEdgePostRouting)
	ipt.CreateChain(TableNat, ChainPostRouting)
	ipt.AppendUniqueRule(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	ipt.AppendUniqueRule(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	ipt.AppendUniqueRule(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", ChainFabEdgePostRouting, " - [0:0]\n", ":", ChainPostRouting, " ACCEPT [0:0]\n",
		"-A ", ChainPostRouting, " -j ", ChainFabEdgePostRouting, "\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}
