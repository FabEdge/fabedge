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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/pkg/util/iptables"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
	routeutil "github.com/fabedge/fabedge/pkg/util/route"
	"github.com/fabedge/fabedge/third_party/ipvs"
)

type Manager struct {
	Config

	netLink ipvs.NetLinkHandle
	ipvs    ipvs.Interface
	ipset   ipset.Interface
	ipt     iptables.ApplierCleaner
	ipt6    iptables.ApplierCleaner

	tm  tunnel.Manager
	log logr.Logger

	currentEndpoint  Endpoint
	mediatorEndpoint *Endpoint
	peerEndpoints    map[string]Endpoint
	// endpointLock is used to protect currentEndpoint and peerEndpoints
	endpointLock sync.RWMutex

	// lastSubnets is used to determine if current node's pod CIDR are changed
	lastSubnets []string

	events   chan struct{}
	debounce func(func())
}

func (m *Manager) start() {
	m.ensureSysctlParameters()

	if m.EnableAutoNetworking {
		m.loadLocalEndpoints()
		go m.broadcastEndpoint()
		go m.receiveEndpoint()
		go m.backupLoop()
	}

	if m.DNS.Enabled {
		_ = m.ensureDummyDevice()
		go m.runCoreDNS()
	}

	if m.Proxy.Enabled {
		go m.runKubeProxy()
	}

	var lastCancel context.CancelFunc = func() {}
	defer func() {
		lastCancel()
	}()

	m.log.V(3).Info("Waiting for strongswan to start")
	for {
		if m.tm.IsRunning() {
			break
		}

		time.Sleep(time.Second)
	}

	m.log.V(3).Info("start network synchronization")
	go m.sync()

	for range m.events {
		// cancel last maintenance if it is still in process
		m.log.V(3).Info("new event come, cancel last maintenance if it exists")
		lastCancel()

		ctx, cancel := context.WithCancel(context.Background())
		// this make `go vet` shut up
		lastCancel = cancel

		go retryForever(ctx, m.maintainNetwork, func(n uint, err error) {
			m.log.Error(err, "failed to configure network", "retryNum", n)
		})

		if m.DNS.Enabled {
			go retryForever(ctx, m.ensureDummyDevice, func(n uint, err error) {
				m.log.Error(err, "failed to maintain dummy interface", "retryNum", n)
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

func (m *Manager) ensureSysctlParameters() {
	if err := ensureSysctl("net/ipv4/ip_forward", 1); err != nil {
		m.log.Error(err, "failed to set net/ipv4/ip_forward to 1")
	}
}

func (m *Manager) maintainNetwork() error {
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
	current, peers := m.getCurrentEndpoint(), m.getPeerEndpoints()

	gw, err := routeutil.GetDefaultGateway()
	if err != nil {
		m.log.Error(err, "failed to get IPv4 default gateway")
	}

	var gw6 net.IP
	if netutil.HasIPv6CIDRString(current.Subnets) {
		gw6, err = routeutil.GetDefaultGateway6()
		if err != nil {
			m.log.Error(err, "failed to get IPv6 default gateway")
		}
	}

	newNames := sets.NewString()

	mediator := m.getMediatorEndpoint()
	if mediator != nil {
		m.log.V(5).Info("Mediator found, try to create tunnel", "mediator", *mediator)
		newNames.Insert(mediator.Name)
		m.ensureMediatorConnection(current, *mediator)
	}

	for _, peer := range peers {
		if peer.IsLocal {
			if err := addRoutesToPeer(peer); err != nil {
				m.log.Error(err, "failed to add routes to peer", "peer", peer)
			}
		} else {
			newNames.Insert(peer.Name)
			m.ensureConnection(current, peer, mediator, gw, gw6)
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

		m.log.V(5).Info("try to unload tunnel", "name", name)
		if err := m.tm.UnloadConn(name); err != nil {
			m.log.Error(err, "failed to unload tunnel", "name", name)
		}
	}

	return delStaleRoutes(peers)
}

func (m *Manager) ensureMediatorConnection(current, peer Endpoint) {
	conn := tunnel.ConnConfig{
		Name: peer.Name,

		// We need a different name to distinguish mediator connection from connector connection
		// when use pubkey auth method, only DN or SAN is valid, endpoint name is used here because
		// we have use it as domain name when generating cert
		LocalID:    current.Name,
		LocalCerts: m.LocalCerts,
		LocalType:  current.Type,

		RemoteID:      peer.ID,
		RemoteAddress: peer.PublicAddresses,
		RemoteType:    peer.Type,
		RemotePort:    peer.Port,

		Mediation: true,
	}

	m.log.V(5).Info("try to add mediation tunnel", "name", peer.Name, "peer", peer, "tunnel", conn)
	if err := m.tm.LoadConn(conn); err != nil {
		m.log.Error(err, "failed to load tunnel", "tunnel", conn)
		return
	}

	m.log.V(5).Info("try to initiate tunnel", "name", peer.Name)
	// this may lead to duplicate child sa in strongswan since sometimes two agents try to initiate
	// the same connection on each side at the same time
	if err := m.tm.InitiateConn(peer.Name); err != nil {
		m.log.Error(err, "failed to initiate tunnel", "tunnel", conn)
		return
	}
}

func (m *Manager) ensureConnection(current, peer Endpoint, mediator *Endpoint, gw, gw6 net.IP) {
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
		RemotePort:        peer.Port,
	}
	if mediator != nil && peer.Type == apis.EdgeNode {
		conn.NeedMediation = true
		conn.MediatedBy = mediator.Name
		conn.MediationPeer = peer.Name
	}

	m.log.V(5).Info("try to add tunnel", "name", peer.Name, "peer", peer, "tunnel", conn)
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
	for _, ip := range []net.IP{gw, gw6} {
		if ip == nil {
			continue
		}

		if err := addRoutesToPeerViaGateway(ip, peer); err != nil {
			m.log.Error(err, "failed to add routes to peer", "peer", peer, "gateway", ip)
		}
	}
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
