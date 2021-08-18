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

package connector

import (
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"net"
	"strings"
)

const (
	TypeBlackHole   = 6
	TableStrongswan = 220
)

func (m *Manager) syncRoutes() {
	active, err := m.tm.IsActive()
	if err != nil {
		klog.Error(err)
	}

	switch active {
	case true:
		err = m.delRoutesNotInConnections()
		if err != nil {
			klog.Error(err)
		}
		_ = m.addAllRoutes()
	case false:
		m.RouteCleanup()
	}

	return
}

func (m *Manager) delRoutesNotInConnections() error {
	var routeFilter = &netlink.Route{
		Table: TableStrongswan,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}

	for _, r := range routes {
		if !m.inConnections(r.Dst) {
			err = m.delStrongswanRoute(r.Dst)
		}
	}

	return err
}

func (m *Manager) inConnections(dst *net.IPNet) bool {
	for _, con := range m.connections {
		for _, subnet := range con.RemoteSubnets {
			// in case subnet is 1.1.1.1, it will be converted to: 1.1.1.1/32
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				klog.Error(err)
			}
			if s.String() == dst.String() {
				return true
			}
		}
	}
	return false
}

func (m *Manager) addAllRoutes() error {
	err := m.addBlackholeRoute()
	if err != nil {
		return err
	}

	return m.addAllStrongswanRoutes()
}

func (m *Manager) addAllStrongswanRoutes() error {
	gw, err := getDefaultGateway()
	if err != nil {
		return err
	}

	for _, conn := range m.connections {
		for _, subnet := range conn.RemoteSubnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}
			route := netlink.Route{Dst: s, Gw: gw, Table: TableStrongswan}
			if err = netlink.RouteAdd(&route); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) delStrongswanRoute(subnet *net.IPNet) error {
	gw, err := getDefaultGateway()
	if err != nil {
		return err
	}
	route := netlink.Route{Dst: subnet, Gw: gw, Table: TableStrongswan}
	return netlink.RouteDel(&route)
}

func (m *Manager) delAllStrongswanRoutes() {
	for _, conn := range m.connections {
		for _, subnet := range conn.RemoteSubnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				klog.Error(err)
			}
			err = m.delStrongswanRoute(s)
			if err != nil && !noSuchProcessError(err) {
				klog.Error(err)
			}
		}
	}
}

func getDefaultGateway() (net.IP, error) {
	defaultRoute, err := netlink.RouteGet(net.ParseIP("8.8.8.8"))
	if len(defaultRoute) != 1 || err != nil {
		return nil, err
	}
	return defaultRoute[0].Gw, nil
}

func (m *Manager) delBlackholeRoute() error {
	dst, _ := netlink.ParseIPNet(m.config.edgePodCIDR)
	route := netlink.Route{Type: TypeBlackHole, Dst: dst}
	return netlink.RouteDel(&route)
}

func (m *Manager) addBlackholeRoute() error {
	dst, _ := netlink.ParseIPNet(m.config.edgePodCIDR)
	route := netlink.Route{Type: TypeBlackHole, Dst: dst}
	if err := netlink.RouteAdd(&route); err != nil {
		if !fileExistsError(err) {
			return err
		}
	}
	return nil
}

func (m *Manager) RouteCleanup() {
	err := m.delBlackholeRoute()
	if err != nil && !noSuchProcessError(err) {
		klog.Error(err)
	}

	m.delAllStrongswanRoutes()
}

func fileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}

// occur when the route does not exist
func noSuchProcessError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such process")
}
