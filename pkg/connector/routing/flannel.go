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
	"github.com/vishvananda/netlink"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/tunnel"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
	routeUtil "github.com/fabedge/fabedge/pkg/util/route"
)

type FlannelRouter struct {
}

func NewFlannelRouter() *FlannelRouter {
	return &FlannelRouter{}
}

func (r *FlannelRouter) SyncRoutes(connections []tunnel.ConnConfig) error {
	if err := delRoutesNotInConnections(connections, constants.TableStrongswan); err != nil {
		return err
	}
	return addAllEdgeRoutes(connections, constants.TableStrongswan)
}

func (r *FlannelRouter) CleanRoutes(conns []tunnel.ConnConfig) error {
	return delAllEdgeRoutes(conns)
}

func (r *FlannelRouter) GetLocalPrefixes(protocolVersion netutil.ProtocolVersion) ([]string, error) {
	family := netlink.FAMILY_V4

	if protocolVersion == netutil.IPV6 {
		family = netlink.FAMILY_V6
	}
	cni0, err := netlink.LinkByName("cni0")
	if err != nil {
		return nil, err
	}

	routes, err := netlink.RouteList(cni0, family)
	if err != nil {
		return nil, err
	}

	var prefixes []string
	for _, r := range routes {
		prefixes = append(prefixes, r.Dst.String())
	}

	return prefixes, nil
}

func (r *FlannelRouter) GetConnectorPrefixes() (*ConnectorPrefixes, error) {
	cp := new(ConnectorPrefixes)
	local, err := r.GetLocalPrefixes(netutil.IPV4)
	if err != nil {
		return nil, err
	}
	cp.LocalPrefixes = local

	localIPv6, err := r.GetLocalPrefixes(netutil.IPV6)
	if err != nil {
		return nil, err
	}
	cp.LocalPrefixesIPv6 = localIPv6

	remote, err := GetRemotePrefixes(netutil.IPV4)
	if err != nil {
		return nil, err
	}
	cp.RemotePrefixes = remote

	remoteIPv6, err := GetRemotePrefixes(netutil.IPV6)
	if err != nil {
		return nil, err
	}
	cp.RemotePrefixesIPv6 = remoteIPv6

	cp.NodeName = routeUtil.GetNodeName()

	return cp, nil
}
