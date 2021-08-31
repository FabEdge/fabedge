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
	"time"

	debpkg "github.com/bep/debounce"
	"github.com/coreos/go-iptables/iptables"
	"github.com/go-logr/logr"
	"github.com/jjeffery/stringset"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/exec"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
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

type CNI struct {
	Version     string
	ConfDir     string
	NetworkName string
	BridgeName  string
}

type Config struct {
	LocalCerts       []string
	SyncPeriod       time.Duration
	DebounceDuration time.Duration
	TunnelsConfPath  string
	ServicesConfPath string
	MasqOutgoing     bool

	DummyInterfaceName string

	UseXfrm           bool
	XfrmInterfaceName string
	XfrmInterfaceID   uint

	CNI CNI

	EnableProxy bool
}

type Manager struct {
	cni CNI

	localCerts       []string
	syncPeriod       time.Duration
	tunnelsConfPath  string
	servicesConfPath string

	netLink            ipvs.NetLinkHandle
	ipvs               ipvs.Interface
	ipset              ipset.Interface
	masqOutgoing       bool
	useXfrm            bool
	xfrmInterfaceName  string
	xfrmInterfaceID    uint
	dummyInterfaceName string
	enableProxy        bool

	tm  tunnel.Manager
	ipt *iptables.IPTables
	log logr.Logger

	events   chan struct{}
	debounce func(func())
}

func newManager(cnf Config) (*Manager, error) {
	kernelHandler := ipvs.NewLinuxKernelHandler()
	canUseProxy, err := ipvs.CanUseIPVSProxier(kernelHandler)
	if err != nil {
		return nil, err
	}
	cnf.EnableProxy = canUseProxy && cnf.EnableProxy

	supportXfrm, err := ipvs.SupportXfrmInterface(kernelHandler)
	if err != nil {
		return nil, err
	}
	cnf.UseXfrm = supportXfrm && cnf.UseXfrm

	var opts strongswan.Options
	if cnf.UseXfrm {
		opts = append(opts, strongswan.InterfaceID(&cnf.XfrmInterfaceID))
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
		cni:              cnf.CNI,
		localCerts:       cnf.LocalCerts,
		syncPeriod:       cnf.SyncPeriod,
		tunnelsConfPath:  cnf.TunnelsConfPath,
		servicesConfPath: cnf.ServicesConfPath,

		useXfrm:            cnf.UseXfrm,
		masqOutgoing:       cnf.MasqOutgoing,
		xfrmInterfaceName:  cnf.XfrmInterfaceName,
		xfrmInterfaceID:    cnf.XfrmInterfaceID,
		dummyInterfaceName: cnf.DummyInterfaceName,
		enableProxy:        cnf.EnableProxy,

		tm:  tm,
		ipt: ipt,
		log: klogr.New().WithName("manager"),

		events:   make(chan struct{}),
		debounce: debpkg.New(cnf.DebounceDuration),

		netLink: ipvs.NewNetLinkHandle(false),
		ipvs:    ipvs.New(exec.New()),
		ipset:   ipset.New(),
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

		if m.masqOutgoing {
			go retryForever(ctx, m.syncIPSetEdgePeerCIDR, func(n uint, err error) {
				m.log.Error(err, "failed to sync ipset EDGE-PEER-CIDR", "retryNum", n)
			})
		}

		go retryForever(ctx, m.mainNetwork, func(n uint, err error) {
			m.log.Error(err, "failed to configure network", "retryNum", n)
		})

		if m.enableProxy {
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

func (m *Manager) mainNetwork() error {
	m.log.V(3).Info("load network config")
	conf, err := netconf.LoadNetworkConf(m.tunnelsConfPath)
	if err != nil {
		return err
	}

	m.log.V(3).Info("synchronize tunnels")
	if err = m.ensureConnections(conf); err != nil {
		return err
	}

	m.log.V(3).Info("generate cni config file")
	if err = m.generateCNIConfig(conf); err != nil {
		return err
	}

	m.log.V(3).Info("keep iptables rules")
	if err = m.ensureIPTablesRules(conf); err != nil {
		return err
	}

	m.log.V(3).Info("maintain dummy/xfrm interface and routes")
	return m.ensureInterfacesAndRoutes()
}

func (m *Manager) ensureConnections(conf netconf.NetworkConf) error {
	newNames := stringset.New()

	netconf.EnsureNodeSubnets(&conf)

	for _, peer := range conf.Peers {
		newNames.Add(peer.Name)

		conn := tunnel.ConnConfig{
			Name: peer.Name,

			LocalID:          conf.ID,
			LocalSubnets:     conf.Subnets,
			LocalNodeSubnets: conf.NodeSubnets,
			LocalCerts:       m.localCerts,

			RemoteID:          peer.ID,
			RemoteAddress:     []string{peer.IP},
			RemoteSubnets:     peer.Subnets,
			RemoteNodeSubnets: peer.NodeSubnets,
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

func (m *Manager) ensureIPTablesRules(conf netconf.NetworkConf) error {
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
	for _, subnet := range conf.Subnets {
		if err := ensureRule(TableFilter, ChainFabEdgeForward, "-s", subnet, "-j", "ACCEPT"); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainFabEdgeForward, "rule", fmt.Sprintf("-s %s -j ACCEPT", subnet))
			return err
		}

		if err := ensureRule(TableFilter, ChainFabEdgeForward, "-d", subnet, "-j", "ACCEPT"); err != nil {
			m.log.Error(err, "failed to check or add rule", "table", TableFilter, "chain", ChainFabEdgeForward, "rule", fmt.Sprintf("-d %s -j ACCEPT", subnet))
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
	if err := m.ensureChain(TableNat, ChainFabEdgeNatOutgoing); err != nil {
		m.log.Error(err, "failed to check or add rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing)
		return err
	}

	if m.masqOutgoing {
		m.log.V(3).Info("configure outgoing NAT iptables rules")
		iFace, err := m.netLink.GetDefaultIFace()
		if err != nil {
			m.log.Error(err, "failed to get default interface")
			return err
		}

		ensureRule := m.ipt.AppendUnique
		if err = ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-m", "set", "--match-set", IPSetFabEdgePeerCIDR, "dst", "-j", "RETURN"); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -m set --match-set %s dst -j RETURN", subnet, IPSetFabEdgePeerCIDR))
			return err
		}

		if err = ensureRule(TableNat, ChainFabEdgeNatOutgoing, "-s", subnet, "-o", iFace, "-j", ChainMasquerade); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainFabEdgeNatOutgoing, "rule", fmt.Sprintf("-s %s -o %s -j %s", subnet, iFace, ChainMasquerade))
			return err
		}

		if err = ensureRule(TableNat, ChainPostRouting, "-j", ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to append rule", "table", TableNat, "chain", ChainPostRouting, "rule", fmt.Sprintf("-j %s", ChainFabEdgeNatOutgoing))
			return err
		}
	} else {
		if err := m.ipt.ClearChain(TableNat, ChainFabEdgeNatOutgoing); err != nil {
			m.log.Error(err, "failed to deletes all rules in the specified table/chain ", "table", TableNat, "chain", ChainPostRouting)
			return err
		}
	}

	return nil
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
		CNIVersion: m.cni.Version,
		Name:       m.cni.NetworkName,
		Type:       "bridge",

		Bridge:           m.cni.BridgeName,
		IsDefaultGateway: true,
		ForceAddress:     true,

		IPAM: IPAMConfig{
			Type:   "host-local",
			Ranges: ranges,
		},
	}

	filename := filepath.Join(m.cni.ConfDir, fmt.Sprintf("%s.conf", m.cni.NetworkName))
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

func (m *Manager) sync() {
	tick := time.NewTicker(m.syncPeriod)
	for {
		m.notify()
		<-tick.C
	}
}

func (m *Manager) ensureInterfacesAndRoutes() error {
	if m.enableProxy {
		m.log.V(3).Info("ensure that the dummy interface exists")
		if _, err := m.netLink.EnsureDummyDevice(m.dummyInterfaceName); err != nil {
			m.log.Error(err, "failed to check or create dummy interface", "dummyInterface", m.dummyInterfaceName)
			return err
		}
	}

	// the kernel has supported xfrm interface since version 4.19+
	if m.useXfrm {
		log := m.log.V(3).WithValues("xfrmInterface", m.xfrmInterfaceName, "if_id", m.xfrmInterfaceID)

		log.Info("ensure that the xfrm interface exists")
		if err := m.netLink.EnsureXfrmInterface(m.xfrmInterfaceName, uint32(m.xfrmInterfaceID)); err != nil {
			log.Error(err, "failed to create xfrm interface")
			return err
		}

		// TODO: add routes to cloud-node, cloud-pod, agent-node and agent-pod
		// The xfrm feature is temporarily unavailable because the routing information is missing

		//log.Info("add a route to edge node", "edgePodCIDR", m.edgePodCIDR)
		//if err := m.netLink.EnsureRouteAdd(m.edgePodCIDR, m.xfrmInterfaceName); err != nil {
		//	m.log.Error(err, "failed to add route", "xfrmInterface", m.xfrmInterfaceName, "podCIDR", m.edgePodCIDR)
		//	return err
		//}

		connectorSubnets, err := m.getConnectorSubnets()
		if err != nil {
			log.Error(err, "failed to get connector subnets")
			return err
		}

		// add routes to the cloud connector
		for _, subnet := range connectorSubnets {
			if err = m.netLink.EnsureRouteAdd(subnet, m.xfrmInterfaceName); err != nil {
				m.log.Error(err, "failed to create route", "subnet", subnet)
				return err
			}
		}
	}
	return nil
}

func (m *Manager) syncLoadBalanceRules() error {
	// sync service clusterIP bound to kube-ipvs0
	// sync ipvs
	m.log.V(3).Info("load services config file")
	conf, err := loadServiceConf(m.servicesConfPath)
	if err != nil {
		m.log.Error(err, "failed to load services config")
		return err
	}

	m.log.V(3).Info("binding cluster ips to dummy interface")
	servers := toServers(conf)
	if err = m.syncServiceClusterIPBind(servers); err != nil {
		return err
	}

	m.log.V(3).Info("synchronize ipvs rules")
	return m.syncVirtualServer(servers)
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

func (m *Manager) syncServiceClusterIPBind(servers []server) error {
	log := m.log.WithValues("dummyInterface", m.dummyInterfaceName)

	addresses, err := m.netLink.ListBindAddress(m.dummyInterfaceName)
	if err != nil {
		log.Error(err, "failed to get addresses from dummyInterface")
		return err
	}

	boundedAddresses := sets.NewString(addresses...)
	allServiceAddresses := sets.NewString()
	for _, s := range servers {
		allServiceAddresses.Insert(s.virtualServer.Address.String())
	}

	for addr := range allServiceAddresses.Difference(boundedAddresses) {
		if _, err = m.netLink.EnsureAddressBind(addr, m.dummyInterfaceName); err != nil {
			log.Error(err, "failed to bind address", "addr", addr)
			return err
		}
	}

	for addr := range boundedAddresses.Difference(allServiceAddresses) {
		if err = m.netLink.UnbindAddress(addr, m.dummyInterfaceName); err != nil {
			log.Error(err, "failed to unbind address", "addr", addr)
			return err
		}
	}

	return nil
}

func (m *Manager) syncVirtualServer(servers []server) error {
	oldVirtualServers, err := m.ipvs.GetVirtualServers()
	if err != nil {
		m.log.Error(err, "failed to get ipvs virtual servers")
		return err
	}
	oldVirtualServerSet := sets.NewString()
	oldVirtualServerMap := make(map[string]*ipvs.VirtualServer, len(oldVirtualServers))
	for _, vs := range oldVirtualServers {
		oldVirtualServerSet.Insert(vs.String())
		oldVirtualServerMap[vs.String()] = vs
	}

	allVirtualServerSet := sets.NewString()
	allVirtualServerMap := make(map[string]*ipvs.VirtualServer, len(servers))
	allVirtualServers := make(map[string]server, len(servers))
	for _, s := range servers {
		allVirtualServerSet.Insert(s.virtualServer.String())
		allVirtualServerMap[s.virtualServer.String()] = s.virtualServer
		allVirtualServers[s.virtualServer.String()] = s
	}

	virtualServersToAdd := allVirtualServerSet.Difference(oldVirtualServerSet)
	for vs := range virtualServersToAdd {
		if err := m.ipvs.AddVirtualServer(allVirtualServerMap[vs]); err != nil {
			m.log.Error(err, "failed to add virtual server", "virtualServer", vs)
			return err
		}

		virtualServer := allVirtualServerMap[vs]
		realServers := allVirtualServers[vs].realServers
		if err := m.updateRealServers(virtualServer, realServers); err != nil {
			return err
		}
	}

	virtualServersToDel := oldVirtualServerSet.Difference(allVirtualServerSet)
	for vs := range virtualServersToDel {
		if err := m.ipvs.DeleteVirtualServer(oldVirtualServerMap[vs]); err != nil {
			m.log.Error(err, "failed to delete virtual server", "virtualServer", vs)
			return err
		}
	}

	virtualServersToUpdate := allVirtualServerSet.Intersection(oldVirtualServerSet)
	for vs := range virtualServersToUpdate {
		virtualServer := allVirtualServerMap[vs]
		realServers := allVirtualServers[vs].realServers
		if err := m.updateRealServers(virtualServer, realServers); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) updateRealServers(virtualServer *ipvs.VirtualServer, realServers []*ipvs.RealServer) error {
	oldRealServers, err := m.ipvs.GetRealServers(virtualServer)
	if err != nil {
		m.log.Error(err, "failed to get real servers")
		return err
	}
	oldRealServerSet := sets.NewString()
	oldRealServerMap := make(map[string]*ipvs.RealServer)
	for _, rs := range oldRealServers {
		oldRealServerSet.Insert(rs.String())
		oldRealServerMap[rs.String()] = rs
	}

	allRealServerSet := sets.NewString()
	allRealServerMap := make(map[string]*ipvs.RealServer)
	for _, rs := range realServers {
		allRealServerSet.Insert(rs.String())
		allRealServerMap[rs.String()] = rs
	}

	realServersToAdd := allRealServerSet.Difference(oldRealServerSet)
	for rs := range realServersToAdd {
		if err := m.ipvs.AddRealServer(virtualServer, allRealServerMap[rs]); err != nil {
			m.log.Error(err, "failed to add real server", "realServer", rs)
			return err
		}
	}

	realServersToDel := oldRealServerSet.Difference(allRealServerSet)
	for rs := range realServersToDel {
		if err := m.ipvs.DeleteRealServer(virtualServer, oldRealServerMap[rs]); err != nil {
			m.log.Error(err, "failed to delete real server", "realServer", rs)
			return err
		}
	}

	return nil
}

func (m *Manager) syncIPSetEdgePeerCIDR() error {
	ipsetObj, err := m.ipset.EnsureIPSet(IPSetFabEdgePeerCIDR, ipset.HashNet)
	if err != nil {
		return err
	}

	allEdgePeerCIDRs, err := m.getAllEdgePeerCIDRs()
	if err != nil {
		return err
	}

	oldEdgePeerCIDRs, err := m.getOldEdgePeerCIDRs()
	if err != nil {
		return err
	}

	return m.ipset.SyncIPSetEntries(ipsetObj, allEdgePeerCIDRs, oldEdgePeerCIDRs, ipset.HashNet)
}

func (m *Manager) getAllEdgePeerCIDRs() (sets.String, error) {
	conf, err := netconf.LoadNetworkConf(m.tunnelsConfPath)
	if err != nil {
		return nil, err
	}

	cidrSet := sets.String{}
	for _, p := range conf.Peers {
		for _, nodeSubnet := range p.NodeSubnets {
			if _, _, err := net.ParseCIDR(nodeSubnet); err != nil {
				s := m.ipset.ConvertIPToCIDR(nodeSubnet)
				cidrSet.Insert(s)
			}
		}

		ip := m.ipset.ConvertIPToCIDR(p.IP)
		cidrSet.Insert(ip)

		cidrSet.Insert(p.Subnets...)
	}

	return cidrSet, nil
}

func (m *Manager) getOldEdgePeerCIDRs() (sets.String, error) {
	return m.ipset.ListEntries(IPSetFabEdgePeerCIDR, ipset.HashNet)
}
