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
	"testing"
)

func TestGenerateCloudAgentRules(t *testing.T) {
	ipt := NewIPTablesHelper()

	// Sync forward
	ipsetName := "FABEDGE-REMOTE-POD-CIDR"
	ipt.ClearAllRules()
	ipt.CreateFabEdgeForwardChain()
	ipt.PrepareForwardChain()
	ipt.MaintainForwardRulesForIPSet([]string{ipsetName})

	// Sync PostRouting
	ipt.PreparePostRoutingChain()

	// ipt.AddPostRoutingRuleForKubernetes()
	// If packets have 0x4000/0x4000 mark, then traffic should be handled by KUBE-POSTROUTING chain,
	// otherwise traffic to nodePort service, sometimes load balancer service, won't be masqueraded,
	// and this would cause response packets are dropped
	ipt.CreateChain(constants.TableNat, "KUBE-POSTROUTING")
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "mark", "--mark", "0x4000/0x4000", "-j", "KUBE-POSTROUTING")

	// AddPostRoutingRulesForIPSet(ipsetName)
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT")
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")

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
	ipt.PrepareForwardChain()
	ipt.MaintainForwardRulesForIPSet([]string{IPSetCloudPodCIDR, IPSetCloudNodeCIDR})

	// ensureNatIPTablesRules
	ipt.PreparePostRoutingChain()

	// for cloud-pod to edge-pod, not masquerade, in order to avoid flannel issue
	allowPostRoutingForIPSet(ipt, IPSetCloudPodCIDR, IPSetEdgePodCIDR)

	// for edge-pod to cloud-pod, not masquerade, in order to avoid flannel issue
	allowPostRoutingForIPSet(ipt, IPSetEdgePodCIDR, IPSetCloudPodCIDR)

	// for cloud-pod to edge-node, not masquerade, in order to avoid flannel issue
	allowPostRoutingForIPSet(ipt, IPSetCloudPodCIDR, IPSetEdgeNodeCIDR)

	// for edge-pod to cloud-node, to masquerade it, in order to avoid rp_filter issue
	masqueradePostRoutingForIPSet(ipt, IPSetEdgePodCIDR, IPSetCloudNodeCIDR)

	// for edge-node to cloud-pod, to masquerade it, or the return traffic will not come back to connector node.
	masqueradePostRoutingForIPSet(ipt, IPSetEdgeNodeCIDR, IPSetCloudPodCIDR)

	// ensureIPSpecInputRules
	ipt.AppendUniqueRule(constants.TableFilter, constants.ChainInput, "-j", constants.ChainFabEdgeInput)
	ipt.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "500", "-j", "ACCEPT")
	ipt.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeInput, "-p", "udp", "-m", "udp", "--dport", "4500", "-j", "ACCEPT")
	ipt.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeInput, "-p", "esp", "-j", "ACCEPT")
	ipt.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeInput, "-p", "ah", "-j", "ACCEPT")

	str := ipt.GenerateInputFromRuleSet()
	println(str)

	//err = ipt.ReplaceRules()
	//if err != nil {
	//	t.Error(err)
	//}
}

func allowPostRoutingForIPSet(helper *IPTablesHelper, src, dst string) {
	helper.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "ACCEPT")
}

func masqueradePostRoutingForIPSet(helper *IPTablesHelper, src, dst string) {
	helper.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", src, "src", "-m", "set", "--match-set", dst, "dst", "-j", "MASQUERADE")
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

	// ipt.MaintainForwardRulesForSubnets(subnets)
	for _, subnet := range subnets {
		ipt.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeForward, "-s", subnet, "-j", "ACCEPT")
		ipt.AppendUniqueRule(constants.TableFilter, constants.ChainFabEdgeForward, "-d", subnet, "-j", "ACCEPT")
	}

	if MASQOutgoing {
		// configureOutboundRules
		if clearFabEdgeNatOutgoingChain {
			ipt.CreateChain(constants.TableNat, constants.ChainFabEdgeNatOutgoing)
		} else {
			ipt.CreateChain(constants.TableNat, constants.ChainFabEdgeNatOutgoing)
		}

		// ipt.MaintainNatOutgoingRulesForSubnets(subnets, ipsetName)
		for _, subnet := range subnets {
			ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgeNatOutgoing, "-s", subnet, "-m", "set", "--match-set", ipsetName, "dst", "-j", "RETURN")
			ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgeNatOutgoing, "-s", subnet, "-d", subnet, "-j", "RETURN")
			ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgeNatOutgoing, "-s", subnet, "-j", constants.ChainMasquerade)
			ipt.AppendUniqueRule(constants.TableNat, constants.ChainPostRouting, "-j", constants.ChainFabEdgeNatOutgoing)
		}
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
	ipt.PrepareForwardChain()
	ipt.MaintainForwardRulesForIPSet([]string{ipsetName})

	// Sync PostRouting
	ipt.PreparePostRoutingChain()

	// ipt.AddPostRoutingRuleForKubernetes()
	// If packets have 0x4000/0x4000 mark, then traffic should be handled by KUBE-POSTROUTING chain,
	// otherwise traffic to nodePort service, sometimes load balancer service, won't be masqueraded,
	// and this would cause response packets are dropped
	ipt.CreateChain(constants.TableNat, "KUBE-POSTROUTING")
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "mark", "--mark", "0x4000/0x4000", "-j", "KUBE-POSTROUTING")

	// AddPostRoutingRulesForIPSet(ipsetName)
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "dst", "-j", "ACCEPT")
	ipt.AppendUniqueRule(constants.TableNat, constants.ChainFabEdgePostRouting, "-m", "set", "--match-set", ipsetName, "src", "-j", "ACCEPT")

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
