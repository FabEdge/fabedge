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
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/tunnel"
	routeUtil "github.com/fabedge/fabedge/pkg/util/route"
	"github.com/vishvananda/netlink"
)

type CalicoRouter struct {
}

func NewCalicoRouter() *CalicoRouter {
	return &CalicoRouter{}
}

func (r *CalicoRouter) SyncRoutes(connections []tunnel.ConnConfig) error {
	if err := delRoutesNotInConnections(connections, constants.TableStrongswan); err != nil {
		return err
	}
	return addAllEdgeRoutes(connections, constants.TableStrongswan)
}

func (r *CalicoRouter) CleanRoutes(conns []tunnel.ConnConfig) error {
	return delAllEdgeRoutes(conns)
}

func (r *CalicoRouter) GetLocalPrefixes() ([]string, error) {
	tunl, err := netlink.LinkByName("tunl0")
	if err != nil {
		return nil, err
	}
	addrs, err := netlink.AddrList(tunl, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}

	var prefixes []string
	for _, a := range addrs {
		prefixes = append(prefixes, a.IPNet.String())
	}

	return prefixes, nil
}

func (r *CalicoRouter) GetConnectorPrefixes() (*ConnectorPrefixes, error) {
	cp := new(ConnectorPrefixes)
	local, err := r.GetLocalPrefixes()
	if err != nil {
		return nil, err
	}
	cp.LocalPrefixes = local

	remote, err := GetRemotePrefixes()
	if err != nil {
		return nil, err
	}
	cp.RemotePrefixes = remote

	cp.NodeName = routeUtil.GetNodeName()

	return cp, nil
}
