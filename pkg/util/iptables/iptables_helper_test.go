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
	"github.com/fabedge/fabedge/pkg/common/constants"
	"strings"
	"testing"
)

func TestCreateChain(t *testing.T) {
	ipt := NewIPTablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", constants.ChainFabEdgePostRouting, " - [0:0]\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateInternalChain(t *testing.T) {
	ipt := NewIPTablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(constants.TableNat, constants.ChainPostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", constants.ChainPostRouting, " ACCEPT [0:0]\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateDuplicatedChains(t *testing.T) {
	ipt := NewIPTablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	ipt.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	ipt.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", constants.ChainFabEdgePostRouting, " - [0:0]\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateChainAndAppendRule(t *testing.T) {
	ipt := NewIPTablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	ipt.CreateChain(constants.TableNat, constants.ChainPostRouting)
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainPostRouting, "-j", constants.ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", constants.ChainFabEdgePostRouting, " - [0:0]\n", ":", constants.ChainPostRouting, " ACCEPT [0:0]\n",
		"-A ", constants.ChainPostRouting, " -j ", constants.ChainFabEdgePostRouting, "\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}

func TestCreateChainAndAppendDuplicatedRules(t *testing.T) {
	ipt := NewIPTablesHelper()
	ipt.ClearAllRules()
	ipt.CreateChain(constants.TableNat, constants.ChainFabEdgePostRouting)
	ipt.CreateChain(constants.TableNat, constants.ChainPostRouting)
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainPostRouting, "-j", constants.ChainFabEdgePostRouting)
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainPostRouting, "-j", constants.ChainFabEdgePostRouting)
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainPostRouting, "-j", constants.ChainFabEdgePostRouting)
	actual := ipt.GenerateInputFromRuleSet()
	expect := strings.Join([]string{"*nat\n",
		":", constants.ChainFabEdgePostRouting, " - [0:0]\n", ":", constants.ChainPostRouting, " ACCEPT [0:0]\n",
		"-A ", constants.ChainPostRouting, " -j ", constants.ChainFabEdgePostRouting, "\n", "COMMIT\n"}, "")
	if actual != expect {
		t.Fatalf("expect: %s, actual: %s", expect, actual)
	}
}
