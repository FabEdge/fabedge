package iptables

import (
	"testing"
)

func TestGenerateCloudAgentRules(t *testing.T) {
	ipt, err := NewIPTablesHelper()
	if err != nil {
		t.Error(err)
	}

	// Sync forward
	ipsetName := "FABEDGE-REMOTE-POD-CIDR"
	ipt.CreateFabEdgeForwardChain()
	ipt.NewMaintainForwardRulesForIPSet([]string{ipsetName})

	// Sync PostRouting
	ipt.NewPreparePostRoutingChain()
	ipt.NewAddPostRoutingRuleForKubernetes()
	ipt.NewAddPostRoutingRulesForIPSet(ipsetName)

	str := ipt.GenerateInputFromRuleSet()
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}
