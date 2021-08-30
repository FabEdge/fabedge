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

package routing

import (
	"fmt"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/vishvananda/netlink"
)

const (
	TableStrongswan = 220
	dummyInfName    = "fabedge-dummy0"
)

type CalicoRouter struct {
}

func NewCalicoRouter() *CalicoRouter {
	return &CalicoRouter{}
}

func (c *CalicoRouter) SyncRoutes(active bool, connections []tunnel.ConnConfig) error {
	switch active {
	case true:
		if err := EnsureDummyDevice(dummyInfName); err != nil {
			return err
		}

		if err := delRoutesNotInConnections(connections, TableStrongswan); err != nil {
			return err
		}

		if err := addAllEdgeRoutes(connections, TableStrongswan); err != nil {
			return err
		}

		if err := addAllEdgeRoutesIntoMainTable(connections, dummyInfName); err != nil {
			return err
		}

	case false:
		if err := c.CleanRoutes(connections); err != nil {
			return err
		}
	}

	return nil
}

func (c *CalicoRouter) CleanRoutes(conns []tunnel.ConnConfig) error {
	// delete routes in table 220
	if err := delAllEdgeRoutes(conns); err != nil {
		return err
	}

	// delete routes in main table via dummy interface
	return delAllEdgeRoutesFromMainTable(dummyInfName)
}

// add all routes via a dummy interface for them to be propagated into other nodes
func addAllEdgeRoutesIntoMainTable(conns []tunnel.ConnConfig, devName string) error {
	link, err := netlink.LinkByName(devName)
	if err != nil {
		return err
	}
	for _, conn := range conns {
		for _, subnet := range conn.RemoteSubnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}
			route := netlink.Route{Dst: s, LinkIndex: link.Attrs().Index}
			err = netlink.RouteAdd(&route)
			if err != nil && !fileExistsError(err) {
				return err
			}
		}
	}

	return nil
}

func delAllEdgeRoutesFromMainTable(devName string) error {
	link := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{Name: devName},
	}
	// after the interface is deleted, all routes via which are cleaned automatically.
	err := netlink.LinkDel(link)
	if err != nil && invalidArgument(err) {
		return fmt.Errorf("%s does not exist", devName)
	} else {
		return err
	}
}
