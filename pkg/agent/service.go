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
	"io/ioutil"
	"net"

	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/third_party/ipvs"
)

type server struct {
	virtualServer *ipvs.VirtualServer
	realServers   []*ipvs.RealServer
}

func (m *Manager) syncLoadBalanceRules() error {
	m.log.V(3).Info("ensure that the dummy interface exists")
	if _, err := m.netLink.EnsureDummyDevice(m.DummyInterfaceName); err != nil {
		m.log.Error(err, "failed to check or create dummy interface", "dummyInterface", m.DummyInterfaceName)
		return err
	}

	// sync service clusterIP bound to kube-ipvs0
	// sync ipvs
	m.log.V(3).Info("load services config file")
	conf, err := loadServiceConf(m.ServicesConfPath)
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

func (m *Manager) syncServiceClusterIPBind(servers []server) error {
	log := m.log.WithValues("dummyInterface", m.DummyInterfaceName)

	addresses, err := m.netLink.ListBindAddress(m.DummyInterfaceName)
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
		if _, err = m.netLink.EnsureAddressBind(addr, m.DummyInterfaceName); err != nil {
			log.Error(err, "failed to bind address", "addr", addr)
			return err
		}
	}

	for addr := range boundedAddresses.Difference(allServiceAddresses) {
		if err = m.netLink.UnbindAddress(addr, m.DummyInterfaceName); err != nil {
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

func loadServiceConf(path string) (netconf.VirtualServers, error) {
	var conf netconf.VirtualServers

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return conf, err
	}

	return conf, yaml.Unmarshal(data, &conf)
}

func toServers(vssConf netconf.VirtualServers) []server {
	servers := []server{}
	for _, vsConf := range vssConf {
		server := server{
			virtualServer: toVirtualServer(vsConf),
		}
		for _, rsConf := range vsConf.RealServers {
			server.realServers = append(server.realServers, toRealServer(rsConf))
		}
		servers = append(servers, server)
	}
	return servers
}

func toVirtualServer(vsConf netconf.VirtualServer) *ipvs.VirtualServer {
	vs := ipvs.VirtualServer{
		Address:  net.ParseIP(vsConf.IP),
		Protocol: string(vsConf.Protocol),
		Port:     uint16(vsConf.Port),
	}

	if len(vsConf.Scheduler) == 0 {
		vsConf.Scheduler = ipvs.DefaultScheduler
	}
	vs.Scheduler = vsConf.Scheduler

	if vsConf.SessionAffinity == v1.ServiceAffinityClientIP {
		vs.Flags |= ipvs.FlagPersistent
		vs.Timeout = uint32(vsConf.StickyMaxAgeSeconds)
	}

	return &vs
}

func toRealServer(rsConf netconf.RealServer) *ipvs.RealServer {
	return &ipvs.RealServer{
		Address: net.ParseIP(rsConf.IP),
		Port:    uint16(rsConf.Port),
		Weight:  1,
	}
}
