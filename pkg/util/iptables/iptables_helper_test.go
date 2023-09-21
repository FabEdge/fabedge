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
	ipt.ClearAllRules()
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

func TestGenerateConnectorRules(t *testing.T) {
	ipt, err := NewIPTablesHelper()
	if err != nil {
		t.Error(err)
	}

	ipt.ClearAllRules()

	const (
		IPSetEdgePodCIDR   = "FABEDGE-EDGE-POD-CIDR"
		IPSetEdgeNodeCIDR  = "FABEDGE-EDGE-NODE-CIDR"
		IPSetCloudPodCIDR  = "FABEDGE-CLOUD-POD-CIDR"
		IPSetCloudNodeCIDR = "FABEDGE-CLOUD-NODE-CIDR"
	)

	// ensureForwardIPTablesRules
	// ensure rules exist
	ipt.NewMaintainForwardRulesForIPSet([]string{IPSetCloudPodCIDR, IPSetCloudNodeCIDR})

	// ensureNatIPTablesRules
	ipt.NewPreparePostRoutingChain()

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	ipt.NewAllowPostRoutingForIPSet(IPSetCloudPodCIDR, IPSetEdgePodCIDR)

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	ipt.NewAllowPostRoutingForIPSet(IPSetEdgePodCIDR, IPSetCloudPodCIDR)

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	ipt.NewAllowPostRoutingForIPSet(IPSetCloudPodCIDR, IPSetEdgeNodeCIDR)

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	ipt.NewMasqueradePostRoutingForIPSet(IPSetEdgePodCIDR, IPSetCloudNodeCIDR)

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	ipt.NewMasqueradePostRoutingForIPSet(IPSetEdgeNodeCIDR, IPSetCloudPodCIDR)

	// ensureIPSpecInputRules
	ipt.NewAllowIPSec()

	str := ipt.GenerateInputFromRuleSet()
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}

func TestGenerateAndClearRules(t *testing.T) {
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
	println("Old:")
	println(str)

	ipt.ClearAllRules()
	ipt.CreateFabEdgeForwardChain()
	str = ipt.GenerateInputFromRuleSet()
	println("New:")
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}
