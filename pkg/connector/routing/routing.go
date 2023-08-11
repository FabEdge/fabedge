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
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/tunnel"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
	routeutil "github.com/fabedge/fabedge/pkg/util/route"
)

var logger = klogr.New().WithName("router")

type ConnectorPrefixes struct {
	NodeName        string   `json:"name"`
	LocalPrefixes   []string `json:"local-prefixes"`
	LocalPrefixes6  []string `json:"local-prefixes6"`
	RemotePrefixes  []string `json:"remote-prefixes"`
	RemotePrefixes6 []string `json:"remote-prefixes6"`
}

type Routing interface {
	SyncRoutes(connections []tunnel.ConnConfig) error
	CleanRoutes(connections []tunnel.ConnConfig) error
	GetConnectorPrefixes() (*ConnectorPrefixes, error)
}

type GeneralRouter struct {
	getLocalPrefixes func() (lp4, lp6 []string, err error)
}

func GetRouter(cni string) (Routing, error) {
	var router Routing
	switch strings.ToUpper(cni) {
	case "CALICO":
		router = &GeneralRouter{getLocalPrefixes: getCalicoLocalPrefixes}
	case "FLANNEL":
		router = &GeneralRouter{getLocalPrefixes: getFlannelLocalPrefixes}
	default:
		return nil, fmt.Errorf("cni:%s is not implemented", cni)
	}

	return router, nil
}

func (_ GeneralRouter) SyncRoutes(connections []tunnel.ConnConfig) error {
	if err := purgeStaleStrongSwanRoutes(connections); err != nil {
		return err
	}
	return addAllEdgeRoutes(connections)
}

func (_ GeneralRouter) CleanRoutes(conns []tunnel.ConnConfig) error {
	dstSet := getRemoteSubnetSet(conns)
	return routeutil.PurgeStrongSwanRoutes(routeutil.NewDstBlacklist(dstSet))
}

func (r GeneralRouter) GetConnectorPrefixes() (*ConnectorPrefixes, error) {
	cp := new(ConnectorPrefixes)

	lp4, lp6, err := r.getLocalPrefixes()
	if err != nil {
		logger.Error(err, "failed to get local prefixes")
		return nil, err
	}
	cp.LocalPrefixes = lp4
	cp.LocalPrefixes6 = lp6

	remotePrefixes4, remotePrefixes6, err := getRemotePrefixes()
	if err != nil {
		logger.Error(err, "failed to get remote prefixes")
		return nil, err
	}
	cp.RemotePrefixes = remotePrefixes4
	cp.RemotePrefixes6 = remotePrefixes6

	cp.NodeName, _ = os.Hostname()

	return cp, nil
}

func getFlannelLocalPrefixes() (lp4, lp6 []string, err error) {
	cni0, err := netlink.LinkByName("cni0")
	if err != nil {
		return nil, nil, err
	}

	routes, err := netlink.RouteList(cni0, netlink.FAMILY_ALL)
	if err != nil {
		return nil, nil, err
	}

	for _, r := range routes {
		if netutil.IsIPv4CIDR(r.Dst) {
			lp4 = append(lp4, r.Dst.String())
		} else {
			lp6 = append(lp6, r.Dst.String())
		}
	}

	return lp4, lp6, nil
}

func getCalicoLocalPrefixes() (lp4, lp6 []string, err error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, nil, err
	}

	collectIPv4 := func(link netlink.Link) {
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
		for _, addr := range addrs {
			lp4 = append(lp4, addr.IPNet.String())
		}
	}

	collectIPv6 := func(link netlink.Link) {
		addrs, _ := netlink.AddrList(link, netlink.FAMILY_V6)
		for _, addr := range addrs {
			lp6 = append(lp6, addr.IPNet.String())
		}
	}

	for _, link := range links {
		switch link.Attrs().Name {
		case "vxlan.calico":
			collectIPv4(link)
		case "vxlan-v6.calico":
			collectIPv6(link)
		case "tunl0":
			collectIPv4(link)
		}
	}

	logger.Info("Calico IP address: IPv4 = " + strings.Join(lp4, ",") + ", IPv6 = " + strings.Join(lp6, ","))

	return lp4, lp6, nil
}

func addAllEdgeRoutes(conns []tunnel.ConnConfig) error {
	logger.V(5).Info("add routes for edge pod CIDRs in strongswan table")

	var hasIPv6CIDR = false
	for _, conn := range conns {
		if netutil.HasIPv6CIDRString(conn.RemoteSubnets) {
			hasIPv6CIDR = true
			break
		}
	}

	var gatewayIPs []net.IP
	gw4, err := routeutil.GetDefaultGateway()
	if err != nil {
		logger.Error(err, "failed to get IPv4 default gateway")
	} else {
		gatewayIPs = append(gatewayIPs, gw4)
	}

	var gw6 net.IP
	if hasIPv6CIDR {
		gw6, err = routeutil.GetDefaultGateway6()
		if err != nil {
			logger.Error(err, "failed to get IPv6 default gateway")
		} else {
			gatewayIPs = append(gatewayIPs, gw6)
		}
	}

	for _, gw := range gatewayIPs {
		for _, conn := range conns {
			if err = routeutil.EnsureStrongswanRoutes(conn.RemoteSubnets, gw); err != nil {
				logger.Error(err, "failed to maintain routes under strongswan table", "connectionName", conn.Name, "RemoteSubnets", conn.RemoteSubnets, "gw", gw)
			}
		}
	}

	return nil
}

func purgeStaleStrongSwanRoutes(connections []tunnel.ConnConfig) error {
	logger.V(5).Info("purge stale strongswan routes")
	dstSet := getRemoteSubnetSet(connections)
	return routeutil.PurgeStrongSwanRoutes(routeutil.NewDstWhitelist(dstSet))
}

func getRemoteSubnetSet(connections []tunnel.ConnConfig) sets.String {
	set := sets.NewString()
	for _, conn := range connections {
		for _, subnet := range conn.RemoteSubnets {
			set.Insert(subnet)
		}
	}

	return set
}

func getRemotePrefixes() (prefixes4, prefixes6 []string, err error) {
	var routeFilter = &netlink.Route{
		Table: constants.TableStrongswan,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return nil, nil, err
	}

	for _, r := range routes {
		if netutil.IsIPv4CIDR(r.Dst) {
			prefixes4 = append(prefixes4, r.Dst.String())
		} else {
			prefixes6 = append(prefixes6, r.Dst.String())
		}
	}

	return prefixes4, prefixes6, nil
}
