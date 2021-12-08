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

package routing

import (
	"fmt"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/tunnel"
	routeUtil "github.com/fabedge/fabedge/pkg/util/route"
	"github.com/vishvananda/netlink"
	"net"
	"strings"
)

type ConnectorPrefixes struct {
	NodeName       string   `json:"name"`
	LocalPrefixes  []string `json:"local-prefixes"`
	RemotePrefixes []string `json:"remote-prefixes"`
}

type Routing interface {
	SyncRoutes(connections []tunnel.ConnConfig) error
	CleanRoutes(connections []tunnel.ConnConfig) error
	GetConnectorPrefixes() (*ConnectorPrefixes, error)
}

func GetRouter(cni string) (Routing, error) {
	var router Routing
	var err error

	switch strings.ToUpper(cni) {
	case "CALICO":
		router = NewCalicoRouter()
	case "FLANNEL":
		router = NewFlannelRouter()
	default:
		err = fmt.Errorf("cni:%s is not implemented", cni)
	}

	return router, err
}

func IsInConns(dst *net.IPNet, connections []tunnel.ConnConfig) (bool, error) {
	for _, con := range connections {
		for _, subnet := range con.RemoteSubnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return false, err
			}
			if s.String() == dst.String() {
				return true, nil
			}
		}
	}
	return false, nil
}

func addAllEdgeRoutes(conns []tunnel.ConnConfig, table int) error {
	gw, err := routeUtil.GetDefaultGateway()
	if err != nil {
		return err
	}

	for _, conn := range conns {
		for _, subnet := range conn.RemoteSubnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}
			// add into table 220
			route := netlink.Route{Dst: s, Gw: gw, Table: table}
			err = netlink.RouteAdd(&route)
			if err != nil && !routeUtil.FileExistsError(err) {
				return err
			}
		}
	}

	return nil
}

func delEdgeRoute(subnet *net.IPNet) error {
	gw, err := routeUtil.GetDefaultGateway()
	if err != nil {
		return err
	}
	route := netlink.Route{Dst: subnet, Gw: gw, Table: constants.TableStrongswan}
	return netlink.RouteDel(&route)
}

func delAllEdgeRoutes(conns []tunnel.ConnConfig) error {
	for _, conn := range conns {
		for _, subnet := range conn.RemoteSubnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}
			err = delEdgeRoute(s)
			if err != nil && !routeUtil.NoSuchProcessError(err) {
				return err
			}
		}
	}

	return nil
}

func delRoutesNotInConnections(connections []tunnel.ConnConfig, table int) error {
	var routeFilter = &netlink.Route{
		Table: table,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}

	for _, r := range routes {
		if yes, err := IsInConns(r.Dst, connections); err == nil && !yes {
			err = delEdgeRoute(r.Dst)
		}
	}

	return err
}

func GetRemotePrefixes() ([]string, error) {
	var routeFilter = &netlink.Route{
		Table: constants.TableStrongswan,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, err
	}

	var prefixes []string
	for _, r := range routes {
		prefixes = append(prefixes, r.Dst.String())
	}

	return prefixes, nil
}
