// Copyright 2021 BoCloud
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
	"strings"
	"time"

	debpkg "github.com/bep/debounce"
	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
	"github.com/jjeffery/stringset"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/exec"
	utilnet "k8s.io/utils/net"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
	"github.com/fabedge/fabedge/third_party/ipvs"
)

const (
	ChainFabEdge     = "FABEDGE"
	ChainForward     = "FORWARD"
	ChainPostRouting = "POSTROUTING"
	ChainMasquerade  = "MASQUERADE"
	ChainNatOutgoing = "fabedge-nat-outgoing"
	TableFilter      = "filter"
	TableNat         = "nat"
)

type Manager struct {
	localCerts       []string
	syncPeriod       time.Duration
	tunnelsConfPath  string
	servicesConfPath string

	tm  tunnel.Manager
	ipt *iptables.IPTables
	log logr.Logger

	events   chan struct{}
	debounce func(func())

	netLink           ipvs.NetLinkHandle
	supportXfrm       bool
	ipvs              ipvs.Interface
	masqOutgoing      bool
	xfrmInterfaceName string
	xfrmInterfaceID   uint
	edgePodCIDR       string
}

func newManager() (*Manager, error) {
	kernelHandler := ipvs.NewLinuxKernelHandler()
	if _, err := ipvs.CanUseIPVSProxier(kernelHandler); err != nil {
		return nil, err
	}

	supportXfrm, err := ipvs.SupportXfrmInterface(kernelHandler)
	if err != nil {
		return nil, err
	}

	var opts strongswan.Options
	if supportXfrm {
		opts = append(opts, strongswan.InterfaceID(&xfrmInterfaceIFID))
	}
	tm, err := strongswan.New(opts...)
	if err != nil {
		return nil, err
	}

	ipt, err := iptables.New()
	if err != nil {
		return nil, err
	}

	m := &Manager{
		localCerts:       []string{localCert},
		syncPeriod:       time.Duration(syncPeriod) * time.Second,
		tunnelsConfPath:  tunnelsConfPath,
		servicesConfPath: servicesConfPath,

		tm:  tm,
		ipt: ipt,
		log: klogr.New().WithName("manager"),

		events:   make(chan struct{}),
		debounce: debpkg.New(time.Duration(debounceDuration) * time.Second),

		netLink:           ipvs.NewNetLinkHandle(false),
		supportXfrm:       supportXfrm,
		ipvs:              ipvs.New(exec.New()),
		masqOutgoing:      masqOutgoing,
		xfrmInterfaceName: xfrmInterfaceName,
		xfrmInterfaceID:   xfrmInterfaceIFID,
		edgePodCIDR:       edgePodCIDR,
	}

	return m, nil
}

func (m *Manager) start() {
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

		go retryForever(ctx, m.syncLoadBalanceRules, func(n uint, err error) {
			m.log.Error(err, "failed to sync load balance rules", "retryNum", n)
		})
	}
}

func (m *Manager) notify() {
	m.debounce(func() {
		m.events <- struct{}{}
	})
}

func (m *Manager) mainNetwork() error {
	conf, err := netconf.LoadNetworkConf(m.tunnelsConfPath)
	if err != nil {
		return err
	}
	m.log.V(3).Info("network conf loaded", "netconf", conf)

	if err = m.ensureConnections(conf); err != nil {
		return err
	}
	m.log.V(3).Info("tunnels are configured")

	if err = m.generateCNIConfig(conf); err != nil {
		return err
	}
	m.log.V(3).Info("cni config is written")

	m.log.V(3).Info("keep iptables rules")
	return m.ensureIPTablesRules(conf)
}

func (m *Manager) ensureConnections(conf netconf.NetworkConf) error {
	newNames := stringset.New()
	subnets := m.addIPToSubnets(conf.IP, conf.Subnets)

	for _, peer := range conf.Peers {
		newNames.Add(peer.Name)
		conn := tunnel.ConnConfig{
			Name: peer.Name,

			LocalID:      conf.ID,
			LocalAddress: []string{conf.IP},
			LocalSubnets: subnets,
			LocalCerts:   m.localCerts,

			RemoteID:      peer.ID,
			RemoteAddress: []string{peer.IP},
		}

		// connector does not need to add its IP address to tunnel.remoteSubnets
		if peer.Name == constants.ConnectorEndpointName {
			conn.RemoteSubnets = peer.Subnets
		} else {
			peerSubnets := m.addIPToSubnets(peer.IP, peer.Subnets)
			conn.RemoteSubnets = peerSubnets
		}

		m.log.V(5).Info("try to add tunnel", "name", peer.Name, "peer", peer)
		if err := m.tm.LoadConn(conn); err != nil {
			return err
		}
	}

	oldNames, err := m.tm.ListConnNames()
	if err != nil {
		return err
	}

	m.log.V(5).Info("clean useless tunnels")
	for _, name := range oldNames {
		if newNames.Contains(name) {
			continue
		}

		if err := m.tm.UnloadConn(name); err != nil {
			m.log.Error(err, "failed to unload tunnel", "name", name)
			return err
		}
	}

	return nil
}

func (m *Manager) addIPToSubnets(ip string, subnets []string) []string {
	if utilnet.IsIPv4(net.ParseIP(ip)) {
		subnets = append(subnets, strings.Join([]string{ip, "32"}, "/"))
	} else {
		subnets = append(subnets, strings.Join([]string{ip, "128"}, "/"))
	}
	return subnets
}

func (m *Manager) ensureIPTablesRules(conf netconf.NetworkConf) error {
	if err := m.ensureChain(TableFilter, ChainFabEdge); err != nil {
		return err
	}

	ensureRule := m.ipt.AppendUnique
	if err := ensureRule(TableFilter, ChainForward, "-j", ChainFabEdge); err != nil {
		return err
	}

	// subnets won't change most of time, and is append-only, so for now we don't need
	// to handle removing old subnet
	for _, subnet := range conf.Subnets {
		if err := ensureRule(TableFilter, ChainFabEdge, "-s", subnet, "-j", "ACCEPT"); err != nil {
			return err
		}

		if err := ensureRule(TableFilter, ChainFabEdge, "-d", subnet, "-j", "ACCEPT"); err != nil {
			return err
		}

		if err := m.configureOutboundRules(subnet); err != nil {
			return err
		}
	}

	return nil
}

// outbound NAT from pods to outside of the cluster
func (m *Manager) configureOutboundRules(subnet string) error {
	if err := m.ensureChain(TableNat, ChainNatOutgoing); err != nil {
		return err
	}

	if m.masqOutgoing {
		m.log.V(3).Info("configure outgoing NAT iptables rules", "masqOutgoing", m.masqOutgoing)
		iFace, err := m.netLink.GetDefaultIFace()
		if err != nil {
			return err
		}

		ensureRule := m.ipt.AppendUnique
		if err := ensureRule(TableNat, ChainNatOutgoing, "-s", subnet, "-d", m.edgePodCIDR, "-j", "RETURN"); err != nil {
			return err
		}

		connectorSubnets, err := m.getConnectorSubnets()
		if err != nil {
			return err
		}

		for _, connSubnet := range connectorSubnets {
			if err := ensureRule(TableNat, ChainNatOutgoing, "-s", subnet, "-d", connSubnet, "-j", "RETURN"); err != nil {
				return err
			}
		}

		if err := ensureRule(TableNat, ChainNatOutgoing, "-s", subnet, "-o", iFace, "-j", ChainMasquerade); err != nil {
			return err
		}

		if err := ensureRule(TableNat, ChainPostRouting, "-j", ChainNatOutgoing); err != nil {
			return err
		}
	} else {
		m.log.V(3).Info("remove NAT outgoing iptables rules if it exists", "masqOutgoing", m.masqOutgoing)
		iFace, err := m.netLink.GetDefaultIFace()
		if err != nil && strings.Contains(err.Error(), "not found") {
			m.log.V(3).Info("iFace was not found in the default route. The NAT outgoing iptables rules does not exist, skip to delete the NAT outgoing iptables rules")
			return nil
		}
		if err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}

		deleteRule := m.ipt.DeleteIfExists

		if err := deleteRule(TableNat, ChainPostRouting, "-j", ChainNatOutgoing); err != nil {
			return err
		}

		if err := deleteRule(TableNat, ChainNatOutgoing, "-s", subnet, "-o", iFace, "-j", ChainMasquerade); err != nil {
			return err
		}

		if err := deleteRule(TableNat, ChainNatOutgoing, "-s", subnet, "-d", m.edgePodCIDR, "-j", "RETURN"); err != nil {
			return err
		}

		connectorSubnets, err := m.getConnectorSubnets()
		if err != nil {
			return err
		}

		for _, connSubnet := range connectorSubnets {
			if err := deleteRule(TableNat, ChainNatOutgoing, "-s", subnet, "-d", connSubnet, "-j", "RETURN"); err != nil {
				return err
			}
		}

	}
	return nil
}

func (m *Manager) ensureChain(table, chain string) error {
	exists, err := m.ipt.ChainExists(table, chain)
	if err != nil {
		return err
	}

	if !exists {
		return m.ipt.NewChain(table, chain)
	}

	return nil
}

func (m *Manager) generateCNIConfig(conf netconf.NetworkConf) error {
	var ranges []RangeSet
	for _, subnet := range conf.Subnets {
		ranges = append(ranges, RangeSet{
			{
				Subnet: subnet,
			},
		})
	}

	cni := CNINetConf{
		CNIVersion: cniVersion,
		Name:       cniNetworkName,
		Type:       "bridge",

		Bridge:           cniBridgeName,
		IsDefaultGateway: true,
		ForceAddress:     true,

		IPAM: IPAMConfig{
			Type:   "host-local",
			Ranges: ranges,
		},
	}

	filename := filepath.Join(cniConfDir, fmt.Sprintf("%s.conf", cniNetworkName))
	data, err := json.MarshalIndent(cni, "", "  ")
	if err != nil {
		return err
	}

	m.log.V(5).Info("generate cni configuration", "cni", cni)
	return ioutil.WriteFile(filename, data, 0644)
}

func (m *Manager) sync() {
	tick := time.NewTicker(m.syncPeriod)
	for {
		m.notify()
		<-tick.C
	}
}

func (m *Manager) syncLoadBalanceRules() error {
	if _, err := m.netLink.EnsureDummyDevice(dummyInterfaceName); err != nil {
		return err
	}
	m.log.V(3).Info("ensure that the dummy interface exists", "dummyInterface", dummyInterfaceName)

	// the kernel has supported xfrm interface since version 4.19+
	if m.supportXfrm {
		if err := m.netLink.EnsureXfrmInterface(m.xfrmInterfaceName, uint32(m.xfrmInterfaceID)); err != nil {
			m.log.Error(err, "failed to create xfrm interface", "xfrmInterface", m.xfrmInterfaceName, "if_id", m.xfrmInterfaceID)
			return err
		}

		m.log.V(3).Info("ensure that the xfrm interface exists", "xfrmInterface", m.xfrmInterfaceName, "if_id", m.xfrmInterfaceID)

		// add a route to another edge node
		if err := m.netLink.EnsureRouteAdd(edgePodCIDR, m.xfrmInterfaceName); err != nil {
			return err
		}
		m.log.V(3).Info("add a route to another edge node", "edgePodCIDR", edgePodCIDR, "xfrmInterface", m.xfrmInterfaceName)

		connectorSubnets, err := m.getConnectorSubnets()
		if err != nil {
			return err
		}
		if connectorSubnets == nil {
			return fmt.Errorf("connector subnets not found")
		}

		// add routes to the cloud
		for _, subnet := range connectorSubnets {
			if err := m.netLink.EnsureRouteAdd(subnet, m.xfrmInterfaceName); err != nil {
				return err
			}
		}
		m.log.V(3).Info("add routes to the cloud", "connectorSubnets", connectorSubnets, "xfrmInterface", m.xfrmInterfaceName)
	}

	// sync service clusterIP bound to kube-ipvs0
	// sync ipvs
	return m.syncService()
}

func (m *Manager) getConnectorSubnets() ([]string, error) {
	conf, err := netconf.LoadNetworkConf(m.tunnelsConfPath)
	if err != nil {
		return nil, err
	}

	for _, p := range conf.Peers {
		if p.Name == constants.ConnectorEndpointName {
			return p.Subnets, nil
		}
	}
	return nil, nil
}

func (m *Manager) syncService() error {
	conf, err := loadServiceConf(m.servicesConfPath)
	if err != nil {
		return err
	}

	servers := toServers(conf)
	if err := m.syncServiceClusterIPBind(servers); err != nil {
		return err
	}

	return m.syncVirtualServer(servers)
}

func (m *Manager) syncServiceClusterIPBind(servers []server) error {
	bindAddrs, err := m.netLink.ListBindAddress(dummyInterfaceName)
	if err != nil {
		return err
	}
	bindAddrSet := sets.NewString(bindAddrs...)
	m.log.V(3).Info("list all IP addresses which are bound in Dummy interface", "dummyInterface", dummyInterfaceName, "ipAddrs", bindAddrs)

	allServiceAddrs := sets.NewString()
	for _, s := range servers {
		allServiceAddrs.Insert(s.virtualServer.Address.String())
	}
	m.log.V(3).Info("extract clusterIP of all services", "clusterIPs", allServiceAddrs)

	addAddrs := allServiceAddrs.Difference(bindAddrSet)
	for addr := range addAddrs {
		if _, err := m.netLink.EnsureAddressBind(addr, dummyInterfaceName); err != nil {
			return err
		}
	}
	m.log.V(3).Info("get IP addresses to be bound to dummy interface", "IPAddrs", addAddrs)

	deleteAddrs := bindAddrSet.Difference(allServiceAddrs)
	for addr := range deleteAddrs {
		if err := m.netLink.UnbindAddress(addr, dummyInterfaceName); err != nil {
			return err
		}
	}
	m.log.V(3).Info("get IP addresses to be unbound to dummy interface", "IPAddrs", deleteAddrs)
	return nil
}

func (m *Manager) syncVirtualServer(servers []server) error {
	oldVirtualServers, err := m.ipvs.GetVirtualServers()
	if err != nil {
		return err
	}
	oldVirtualServerSet := sets.NewString()
	oldVirtualServerMap := make(map[string]*ipvs.VirtualServer)
	for _, vs := range oldVirtualServers {
		oldVirtualServerSet.Insert(vs.String())
		oldVirtualServerMap[vs.String()] = vs
	}
	m.log.V(3).Info("get old virtual servers", "virtualServers", oldVirtualServerSet)

	allVirtualServerSet := sets.NewString()
	allVirtualServerMap := make(map[string]*ipvs.VirtualServer)
	allVirtualServers := make(map[string]server)
	for _, s := range servers {
		allVirtualServerSet.Insert(s.virtualServer.String())
		allVirtualServerMap[s.virtualServer.String()] = s.virtualServer
		allVirtualServers[s.virtualServer.String()] = s
	}
	m.log.V(3).Info("get all virtual servers", "virtualServers", allVirtualServerSet)

	virtualServersToAdd := allVirtualServerSet.Difference(oldVirtualServerSet)
	for vs := range virtualServersToAdd {
		if err := m.ipvs.AddVirtualServer(allVirtualServerMap[vs]); err != nil {
			return err
		}

		virtualServer := allVirtualServerMap[vs]
		realServers := allVirtualServers[vs].realServers
		if err := m.updateRealServers(virtualServer, realServers); err != nil {
			return err
		}
	}
	m.log.V(3).Info("added virtual servers", "virtualServers", virtualServersToAdd)

	virtualServersToDel := oldVirtualServerSet.Difference(allVirtualServerSet)
	for vs := range virtualServersToDel {
		if err := m.ipvs.DeleteVirtualServer(oldVirtualServerMap[vs]); err != nil {
			return err
		}
	}
	m.log.V(3).Info("deleted virtual servers", "virtualServers", virtualServersToDel)

	virtualServersToUpdate := allVirtualServerSet.Intersection(oldVirtualServerSet)
	for vs := range virtualServersToUpdate {
		virtualServer := allVirtualServerMap[vs]
		realServers := allVirtualServers[vs].realServers
		if err := m.updateRealServers(virtualServer, realServers); err != nil {
			return err
		}
	}
	m.log.V(3).Info("updated virtual servers", "virtualServers", virtualServersToUpdate)
	return nil
}

func (m *Manager) updateRealServers(virtualServer *ipvs.VirtualServer, realServers []*ipvs.RealServer) error {
	oldRealServers, err := m.ipvs.GetRealServers(virtualServer)
	if err != nil {
		return err
	}
	oldRealServerSet := sets.NewString()
	oldRealServerMap := make(map[string]*ipvs.RealServer)
	for _, rs := range oldRealServers {
		oldRealServerSet.Insert(rs.String())
		oldRealServerMap[rs.String()] = rs
	}
	m.log.V(3).Info("get old real servers of the virtual server", "virtualServer", virtualServer, "realServers", oldRealServerSet)

	allRealServerSet := sets.NewString()
	allRealServerMap := make(map[string]*ipvs.RealServer)
	for _, rs := range realServers {
		allRealServerSet.Insert(rs.String())
		allRealServerMap[rs.String()] = rs
	}
	m.log.V(3).Info("get all real servers of the virtual server", "virtualServer", virtualServer, "realServers", allRealServerSet)

	realServersToAdd := allRealServerSet.Difference(oldRealServerSet)
	for rs := range realServersToAdd {
		if err := m.ipvs.AddRealServer(virtualServer, allRealServerMap[rs]); err != nil {
			return err
		}
	}
	m.log.V(3).Info("added new real servers of the virtual server", "virtualServer", virtualServer, "realServers", realServersToAdd)

	realServersToDel := oldRealServerSet.Difference(allRealServerSet)
	for rs := range realServersToDel {
		if err := m.ipvs.DeleteRealServer(virtualServer, oldRealServerMap[rs]); err != nil {
			return err
		}
	}
	m.log.V(3).Info("deleted inactive real servers of the virtual server", "virtualServer", virtualServer, "realServers", realServersToDel)
	return nil
}
