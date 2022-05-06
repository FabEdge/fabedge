package agent

import (
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	TableStrongswan = 220
)

func getDefaultGateway() (net.IP, error) {
	defaultRoute, err := netlink.RouteGet(net.ParseIP("8.8.8.8"))
	if len(defaultRoute) != 1 || err != nil {
		return nil, err
	}
	return defaultRoute[0].Gw, nil
}

func addRoutesToPeerViaGateway(gw net.IP, peer Endpoint) error {
	for _, subnet := range peer.Subnets {
		s, err := netlink.ParseIPNet(subnet)
		if err != nil {
			return err
		}

		if err = ensureUniqueRoute(s, gw, TableStrongswan); err != nil {
			return err
		}
	}

	return nil
}

// ensureUniqueRoute will remove any routes which doesn't have the same
// dst and gw from specified table
func ensureUniqueRoute(dst *net.IPNet, gw net.IP, table int) error {
	exist := false

	var routeFilter = &netlink.Route{
		Dst:   dst,
		Table: TableStrongswan,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
	if err != nil {
		return err
	}

	for _, route := range routes {
		if route.Gw.Equal(gw) {
			exist = true
			continue
		} else {
			if err = netlink.RouteDel(&route); err != nil {
				return err
			}
		}
	}

	if exist {
		return nil
	}

	route := netlink.Route{Dst: dst, Gw: gw, Table: table}
	err = netlink.RouteAdd(&route)
	if err != nil && !fileExistsError(err) {
		return err
	}

	return nil
}

func addRoutesToPeer(peer Endpoint) error {
	// todo: IPv6
	for _, nodeSubnet := range peer.NodeSubnets {
		gw := net.ParseIP(nodeSubnet)
		return addRoutesToPeerViaGateway(gw, peer)
	}

	return nil
}

func delStaleRoutes(peers []Endpoint) error {
	var routeFilter = &netlink.Route{
		Table: TableStrongswan,
	}

	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}

	dstSet := sets.NewString()
	for _, peer := range peers {
		dstSet.Insert(peer.Subnets...)
	}

	for _, route := range routes {
		if dstSet.Has(route.Dst.String()) {
			continue
		}

		// todo: aggregate errors
		err = netlink.RouteDel(&route)
	}

	return err
}

func fileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}
