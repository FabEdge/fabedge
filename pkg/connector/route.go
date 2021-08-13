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

func (m *Manager) syncRoutes() error {
	active, err := m.tm.IsActive()
	if err != nil {
		return err
	}

	switch active {
	case true:
		return m.addAllRoutes()
	case false:
		return m.RouteCleanup()
	}

	return nil
}

func (m *Manager) addAllRoutes() error {
	err := m.addBlackholeRoute()
	if err != nil {
		return err
	}

	return m.addRoutesFromConnections()
}

func (m *Manager) addRoutesFromConnections() error {
	gw, err := getGateway()
	if err != nil {
		return err
	}

	for _, conn := range m.connections {
		for _, subnet := range conn.RemoteSubnets {
			dst, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}
			route := netlink.Route{Dst: dst, Gw: gw, Table: TableStrongswan}
			if err = netlink.RouteAdd(&route); err != nil {
				return err
			}
		}
	}

	return nil
}

func (m *Manager) delRoutesFromConnections() error {
	gw, err := getGateway()
	if err != nil {
		return err
	}

	for _, conn := range m.connections {
		for _, subnet := range conn.RemoteSubnets {
			dst, err := netlink.ParseIPNet(subnet)
			if err != nil {
				klog.Error(err)
				continue
			}
			route := netlink.Route{Dst: dst, Gw: gw, Table: TableStrongswan}
			if err = netlink.RouteDel(&route); err != nil {
				klog.Error(err)
				continue
			}
		}
	}

	return nil
}

func getGateway() (net.IP, error) {
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

func (m *Manager) RouteCleanup() error {
	err := m.delBlackholeRoute()
	if err != nil {
		return err
	}

	return m.delRoutesFromConnections()
}

func fileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}
