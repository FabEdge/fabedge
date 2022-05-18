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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/third_party/ipvs"
)

const (
	TableFilter             = "filter"
	TableNat                = "nat"
	ChainForward            = "FORWARD"
	ChainPostRouting        = "POSTROUTING"
	ChainMasquerade         = "MASQUERADE"
	ChainFabEdgeForward     = "FABEDGE-FORWARD"
	ChainFabEdgeNatOutgoing = "FABEDGE-NAT-OUTGOING"
	IPSetFabEdgePeerCIDR    = "FABEDGE-PEER-CIDR"
)

type Manager struct {
	Config
	netLink ipvs.NetLinkHandle
	ipvs    ipvs.Interface
	ipset   ipset.Interface

	tm  tunnel.Manager
	ipt *iptables.IPTables
	log logr.Logger

	// lastSubnets is used to determine whether to clear chain FABEDGE-NAT-OUTGOING
	lastSubnets     []string
	currentEndpoint Endpoint
	peerEndpoints   map[string]Endpoint
	lock            sync.RWMutex

	events   chan struct{}
	debounce func(func())
}

func (m *Manager) start() {
	if m.EnableAutoNetworking {
		m.loadLocalEndpoints()
		go m.broadcastEndpoint()
		go m.receiveEndpoint()
		go m.backupLoop()
	}

	var lastCancel context.CancelFunc = func() {}
	defer func() {
		lastCancel()
	}()

	m.log.V(3).Info("start network synchronization")
	go m.sync()

	for range m.events {
		// cancel last maintenance if it is still in process
		m.log.V(3).Info("new event come, cancel last maintenance if it exists")
		lastCancel()

		ctx, cancel := context.WithCancel(context.Background())
		// this make `go vet` shut up
		lastCancel = cancel

		go retryForever(ctx, m.mainNetwork, func(n uint, err error) {
			m.log.Error(err, "failed to configure network", "retryNum", n)
		})

		if m.EnableProxy {
			go retryForever(ctx, m.syncLoadBalanceRules, func(n uint, err error) {
				m.log.Error(err, "failed to sync load balance rules", "retryNum", n)
			})
		}
	}
}

func (m *Manager) notify() {
	m.debounce(func() {
		m.events <- struct{}{}
	})
}

func (m *Manager) sync() {
	tick := time.NewTicker(m.SyncPeriod)
	for {
		m.notify()
		<-tick.C
	}
}

func (m *Manager) mainNetwork() error {
	m.log.V(3).Info("load network config")
	err := m.loadNetworkConf()
	if err != nil {
		m.log.Error(err, "failed to load network configuration")
		return err
	}

	m.log.V(3).Info("clean expired endpoints")
	m.cleanExpiredEndpoints()

	m.log.V(3).Info("synchronize tunnels")
	if err := m.ensureConnections(); err != nil {
		return err
	}

	m.log.V(3).Info("generate cni config file")
	if err := m.generateCNIConfig(); err != nil {
		return err
	}

	m.log.V(3).Info("keep iptables rules")
	return m.ensureIPTablesRules()
}

func (m *Manager) ensureConnections() error {
	gw, err := getDefaultGateway()
	if err != nil {
		m.log.Error(err, "failed to get default gateway IP")
		return err
	}

	current, peers := m.getCurrentEndpoint(), m.getPeerEndpoints()

	newNames := sets.NewString()
	for _, peer := range peers {
		if peer.IsLocal {
			if err := addRoutesToPeer(peer); err != nil {
				m.log.Error(err, "failed to add routes to peer", "peer", peer)
			}
		} else {
			newNames.Insert(peer.Name)
			m.ensureConnection(current, peer, gw)
		}
	}

	oldNames, err := m.tm.ListConnNames()
	if err != nil {
		m.log.Error(err, "failed to list connections")
		return err
	}

	m.log.V(5).Info("clean useless tunnels")
	for _, name := range oldNames {
		if newNames.Has(name) {
			continue
		}

		if err := m.tm.UnloadConn(name); err != nil {
			m.log.Error(err, "failed to unload tunnel", "name", name)
		}
	}

	return delStaleRoutes(peers)
}

func (m *Manager) ensureConnection(current, peer Endpoint, gw net.IP) {
	conn := tunnel.ConnConfig{
		Name: peer.Name,

		LocalID:          current.ID,
		LocalSubnets:     current.Subnets,
		LocalNodeSubnets: current.NodeSubnets,
		LocalCerts:       m.LocalCerts,
		LocalType:        current.Type,

		RemoteID:          peer.ID,
		RemoteAddress:     peer.PublicAddresses,
		RemoteSubnets:     peer.Subnets,
		RemoteNodeSubnets: peer.NodeSubnets,
		RemoteType:        peer.Type,
	}

	m.log.V(5).Info("try to add tunnel", "name", peer.Name, "peer", peer)
	if err := m.tm.LoadConn(conn); err != nil {
		m.log.Error(err, "failed to add tunnel", "tunnel", conn)
		return
	}

	m.log.V(5).Info("try to initiate tunnel", "name", peer.Name)
	// this may lead to duplicate child sa in strongswan since sometimes two agents try to initiate
	// the same connection on each side at the same time
	if err := m.tm.InitiateConn(peer.Name); err != nil {
		m.log.Error(err, "failed to initiate tunnel", "tunnel", conn)
		return
	}

	m.log.V(5).Info("try to add routes to peer", "name", peer.Name)
	if err := addRoutesToPeerViaGateway(gw, peer); err != nil {
		m.log.Error(err, "failed to add routes to peer", "peer", peer)
	}
}

func (m *Manager) ensureIPTablesRules() error {
	current := m.getCurrentEndpoint()

	if err := m.ensureChain(TableFilter, ChainFabEdgeForward); err != nil {
		m.log.Error(err, "failed to check or create iptables chain", "table", TableFilter, "chain", ChainFabEdgeForward)
		return err
	}

	ensureRule := m.ipt.AppendUnique
	if err := ensureRule(TableFilter, ChainForward, "-j", ChainFabEdgeForward); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainForward, "rule", "-j FABEDGE")
		return err
	}

	// subnets won't change most of time, and is append-only, so for now we don't need
	// to handle removing old subnet
	for _, subnet := range current.Subnets {
		if err := ensureRule(TableFilter, ChainFabEdgeForward, "-s", subnet, "-j", "ACCEPT"); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainFabEdgeForward, "rule", fmt.Sprintf("-s %s -j ACCEPT", subnet))
			return err
		}

		if err := ensureRule(TableFilter, ChainFabEdgeForward, "-d", subnet, "-j", "ACCEPT"); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainFabEdgeForward, "rule", fmt.Sprintf("-d %s -j ACCEPT", subnet))
			return err
		}
	}

	if m.MASQOutgoing {
		return m.configureOutboundRules(current.Subnets)
	}

	return nil
}

// outbound NAT from pods to outside the cluster
func (m *Manager) configureOutboundRules(subnets []string) error {
	if !m.areSubnetsEqual(subnets, m.lastSubnets) {
		m.log.V(3).Info("Subnets are changed, clear iptables chain FABEDGE-NAT-OUTGOING")
		if err := m.ipt.ClearChain(TableNat, ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing)
			return err
		}
		m.lastSubnets = subnets
	}

	for _, subnet := range subnets {
		m.log.V(3).Info("configure outgoing NAT iptables rules")

		ensureRule := m.ipt.AppendUnique
		if err := ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-m", "set", "--match-set", IPSetFabEdgePeerCIDR, "dst", "-j", "RETURN"); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -m set --match-set %s dst -j RETURN", subnet, IPSetFabEdgePeerCIDR))
			continue
		}

		if err := ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-d", subnet, "-j", "RETURN"); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -d %s -j RETURN", subnet, subnet))
			continue
		}

		if err := ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-j", ChainMasquerade); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -j %s", subnet, ChainMasquerade))
			continue
		}

		if err := ensureRule(TableNat, ChainPostRouting, "-j", ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainPostRouting, "rule", fmt.Sprintf("-j %s", ChainFabEdgeNatOutgoing))
			continue
		}
	}

	if err := m.syncIPSetPeerCIDR(); err != nil {
		m.log.Error(err, "failed to sync ipset FABEDGE-PEER-CIDR")
		return err
	}

	return nil
}

func (m *Manager) areSubnetsEqual(sa1, sa2 []string) bool {
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

func (m *Manager) ensureChain(table, chain string) error {
	exists, err := m.ipt.ChainExists(table, chain)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	return m.ipt.NewChain(table, chain)
}

func (m *Manager) generateCNIConfig() error {
	current := m.getCurrentEndpoint()

	var ranges []RangeSet
	for _, subnet := range current.Subnets {
		ranges = append(ranges, RangeSet{
			{
				Subnet: subnet,
			},
		})
	}

	cni := CNINetConf{
		CNIVersion: m.CNI.Version,
		Name:       m.CNI.NetworkName,
	}

	bridge := BridgeConfig{
		Type: "bridge",

		Bridge:           m.CNI.BridgeName,
		IsDefaultGateway: true,
		ForceAddress:     true,
		HairpinMode:      m.EnableHairpinMode,
		MTU:              m.NetworkPluginMTU,

		IPAM: IPAMConfig{
			Type:   "host-local",
			Ranges: ranges,
		},
	}

	portmap := CapbilitiesConfig{
		Type:         "portmap",
		Capabilities: map[string]bool{"portMappings": true},
	}

	// bandwidth under control by metadata.annotations within yaml:
	// kubernetes.io/ingress-bandwidth: 1M
	// kubernetes.io/egress-bandwidth: 1M
	// there will be no limit without these 2 items.
	bandwidth := CapbilitiesConfig{
		Type:         "bandwidth",
		Capabilities: map[string]bool{"bandwidth": true},
	}
	cni.Plugins = append(cni.Plugins, bridge, portmap, bandwidth)

	filename := filepath.Join(m.CNI.ConfDir, fmt.Sprintf("%s.conflist", m.CNI.NetworkName))
	data, err := json.MarshalIndent(cni, "", "  ")
	if err != nil {
		m.log.Error(err, "failed to marshal cni config")
		return err
	}

	m.log.V(5).Info("generate cni configuration", "cni", cni)
	err = ioutil.WriteFile(filename, data, 0644)
	if err != nil {
		m.log.Error(err, "failed to write cni config file")
	}

	return err
}

func (m *Manager) syncIPSetPeerCIDR() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetFabEdgePeerCIDR, ipset.HashNet)
	if err != nil {
		return err
	}

	allPeerCIDRs, err := m.getAllPeerCIDRs()
	if err != nil {
		return err
	}

	oldPeerCIDRs, err := m.getOldPeerCIDRs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allPeerCIDRs, oldPeerCIDRs, ipset.HashNet)
}

func (m *Manager) getAllPeerCIDRs() (sets.String, error) {
	cidrSet := sets.String{}

	for _, peer := range m.getPeerEndpoints() {
		for _, nodeSubnet := range peer.NodeSubnets {
			if _, _, err := net.ParseCIDR(nodeSubnet); err != nil {
				s := m.ipset.ConvertIPToCIDR(nodeSubnet)
				cidrSet.Insert(s)
			} else {
				cidrSet.Insert(nodeSubnet)
			}
		}

		cidrSet.Insert(peer.Subnets...)
	}

	return cidrSet, nil
}

func (m *Manager) getOldPeerCIDRs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetFabEdgePeerCIDR, ipset.HashNet)
}
