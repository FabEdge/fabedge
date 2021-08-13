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
	"strings"
)

const TypeBlackHole = 6

// syncRoutes propagates one aggregated blackhole route
func (m *Manager) syncRoutes(cidr string) error {
	active, err := m.tm.IsActive()
	if err != nil {
		return err
	}

	// I am inactive, nothing to do
	if !active {
		return nil
	}

	dst, err := netlink.ParseIPNet(cidr)
	route := netlink.Route{Type: TypeBlackHole, Dst: dst}
	if err = netlink.RouteAdd(&route); err != nil {
		if !fileExistsError(err) {
			return err
		}
	}

	return nil
}

func delRoutes(cidr string) error {
	dst, _ := netlink.ParseIPNet(cidr)
	route := netlink.Route{Type: TypeBlackHole, Dst: dst}
	return netlink.RouteDel(&route)
}

func fileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}
