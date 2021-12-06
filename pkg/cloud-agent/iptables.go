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
	"github.com/coreos/go-iptables/iptables"
	"github.com/fabedge/fabedge/pkg/connector/routing"
	ipsetUtil "github.com/fabedge/fabedge/pkg/util/ipset"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
)

const (
	TableFilter             = "filter"
	TableNat                = "nat"
	ChainForward            = "FORWARD"
	ChainPostRouting        = "POSTROUTING"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting = "FABEDGE-POSTROUTING"
	IPSetRemotePodCIDR      = "FABEDGE-REMOTE-POD-CIDR"
)

var (
	ipt   *iptables.IPTables
	ipset = ipsetUtil.New()
)

func init() {
	var err error
	ipt, err = iptables.New()
	if err != nil {
		klog.Exit("failed to get iptables client:%s", err)
	}
}

func syncForwardRules() (err error) {
	if err = ipt.ClearChain(TableFilter, ChainFabEdgeForward); err != nil {
		return err
	}
	exists, err := ipt.Exists(TableFilter, ChainForward, "-j", ChainFabEdgeForward)
	if err != nil {
		return err
	}

	if !exists {
		if err = ipt.Insert(TableFilter, ChainForward, 1, "-j", ChainFabEdgeForward); err != nil {
			return err
		}
	}

	if err = ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", IPSetRemotePodCIDR, "src", "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = ipt.AppendUnique(TableFilter, ChainFabEdgeForward, "-m", "set", "--match-set", IPSetRemotePodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}

func syncPostRoutingRules() (err error) {
	if err = ipt.ClearChain(TableNat, ChainFabEdgePostRouting); err != nil {
		return err
	}
	exists, err := ipt.Exists(TableNat, ChainPostRouting, "-j", ChainFabEdgePostRouting)
	if err != nil {
		return err
	}

	if !exists {
		if err = ipt.Insert(TableNat, ChainPostRouting, 1, "-j", ChainFabEdgePostRouting); err != nil {
			return err
		}
	}

	if err = ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", IPSetRemotePodCIDR, "dst", "-j", "ACCEPT"); err != nil {
		return err
	}

	return ipt.AppendUnique(TableNat, ChainFabEdgePostRouting, "-m", "set", "--match-set", IPSetRemotePodCIDR, "src", "-j", "ACCEPT")

}

func syncRemotePodCIDRSet(cp routing.ConnectorPrefixes) error {
	ipsetObj, err := ipset.EnsureIPSet(IPSetRemotePodCIDR, ipsetUtil.HashNet)
	if err != nil {
		return err
	}

	allRemotePodCIDRs := getAllRemotePodCIDRs(cp)

	oldRemotePodCIDRs, err := getOldRemotePodCIDRs()
	if err != nil {
		return err
	}

	return ipset.SyncIPSetEntries(ipsetObj, allRemotePodCIDRs, oldRemotePodCIDRs, ipsetUtil.HashNet)
}

func getAllRemotePodCIDRs(cp routing.ConnectorPrefixes) sets.String {
	return sets.NewString(cp.RemotePrefixes...)
}

func getOldRemotePodCIDRs() (sets.String, error) {
	return ipset.ListEntries(IPSetRemotePodCIDR, ipsetUtil.HashNet)
}
