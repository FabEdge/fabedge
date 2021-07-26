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

package route

import (
	"github.com/vishvananda/netlink"
	"strings"
)

const TypeBlackHole = 6
const StrongswanTable = 220

func edgeRoutesExist() (bool, error) {
	routeFilter := &netlink.Route{Table: StrongswanTable}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return false, err
	}
	//since table 220 is dedicated for strongswan, we don't do the preciously comparison.
	if len(routes) > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

// SyncRoutes sync the aggregated blackhole route with the specific ones.
func SyncRoutes(cidr string) error {
	exist, err := edgeRoutesExist()
	if err != nil {
		return err
	}

	dst, err := netlink.ParseIPNet(cidr)
	if err != nil {
		return err
	}
	route := netlink.Route{Type: TypeBlackHole, Dst: dst}

	// strongswan tunnels are disconnected, to remove the aggregated route
	if !exist {
		return netlink.RouteDel(&route)
	}

	// strongswan tunnels are connected, to add the aggregated route
	if err = netlink.RouteAdd(&route); err != nil {
		if !fileExistsError(err) {
			return err
		}
	}

	return nil
}

func fileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}
