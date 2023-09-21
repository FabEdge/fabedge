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
	"testing"
)

func TestGenerateCloudAgentRules(t *testing.T) {
	ipt := NewIPTablesHelper()

	// Sync forward
	ipsetName := "FABEDGE-REMOTE-POD-CIDR"
	ipt.ClearAllRules()
	ipt.CreateFabEdgeForwardChain()
	ipt.MaintainForwardRulesForIPSet([]string{ipsetName})

	// Sync PostRouting
	ipt.PreparePostRoutingChain()
	ipt.AddPostRoutingRuleForKubernetes()
	ipt.AddPostRoutingRulesForIPSet(ipsetName)

	str := ipt.GenerateInputFromRuleSet()
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}

func TestGenerateConnectorRules(t *testing.T) {
	ipt := NewIPTablesHelper()

	ipt.ClearAllRules()

	const (
		IPSetEdgePodCIDR   = "FABEDGE-EDGE-POD-CIDR"
		IPSetEdgeNodeCIDR  = "FABEDGE-EDGE-NODE-CIDR"
		IPSetCloudPodCIDR  = "FABEDGE-CLOUD-POD-CIDR"
		IPSetCloudNodeCIDR = "FABEDGE-CLOUD-NODE-CIDR"
	)

	// ensureForwardIPTablesRules
	// ensure rules exist
	ipt.MaintainForwardRulesForIPSet([]string{IPSetCloudPodCIDR, IPSetCloudNodeCIDR})

	// ensureNatIPTablesRules
	ipt.PreparePostRoutingChain()

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	ipt.AllowPostRoutingForIPSet(IPSetCloudPodCIDR, IPSetEdgePodCIDR)

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	ipt.AllowPostRoutingForIPSet(IPSetEdgePodCIDR, IPSetCloudPodCIDR)

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	ipt.AllowPostRoutingForIPSet(IPSetCloudPodCIDR, IPSetEdgeNodeCIDR)

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	ipt.MasqueradePostRoutingForIPSet(IPSetEdgePodCIDR, IPSetCloudNodeCIDR)

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	ipt.MasqueradePostRoutingForIPSet(IPSetEdgeNodeCIDR, IPSetCloudPodCIDR)

	// ensureIPSpecInputRules
	ipt.AllowIPSec()

	str := ipt.GenerateInputFromRuleSet()
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}

func TestGenerateAgentRules(t *testing.T) {
	ipt := NewIPTablesHelper()

	MASQOutgoing := true
	clearFabEdgeNatOutgoingChain := true
	subnets := []string{"192.168.1.0/24", "192.168.2.0/24"}
	ipsetName := "IPSET"

	ipt.ClearAllRules()

	// ensureIPForwardRules
	ipt.CreateFabEdgeForwardChain()
	ipt.PrepareForwardChain()

	// subnets won't change most of the time, and is append-only, so for now we don't need
	// to handle removing old subnet
	ipt.MaintainForwardRulesForSubnets(subnets)

	if MASQOutgoing {
		// configureOutboundRules
		if clearFabEdgeNatOutgoingChain {
			ipt.CreateFabEdgeNatOutgoingChain()
		} else {
			ipt.CreateFabEdgeNatOutgoingChain()
		}
		ipt.MaintainNatOutgoingRulesForSubnets(subnets, ipsetName)
	}

	str := ipt.GenerateInputFromRuleSet()
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}

func TestGenerateAndClearRules(t *testing.T) {
	ipt := NewIPTablesHelper()

	// Sync forward
	ipsetName := "FABEDGE-REMOTE-POD-CIDR"
	ipt.CreateFabEdgeForwardChain()
	ipt.MaintainForwardRulesForIPSet([]string{ipsetName})

	// Sync PostRouting
	ipt.PreparePostRoutingChain()
	ipt.AddPostRoutingRuleForKubernetes()
	ipt.AddPostRoutingRulesForIPSet(ipsetName)

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
