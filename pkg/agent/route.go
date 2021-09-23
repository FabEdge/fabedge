package agent

import (
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/vishvananda/netlink"
	"net"
	"strings"
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

func addRoutesToAllPeers(conf netconf.NetworkConf) error {
	gw, err := getDefaultGateway()
	if err != nil {
		return err
	}

	for _, peer := range conf.Peers {
		for _, subnet := range peer.Subnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}
			route := netlink.Route{Dst: s, Gw: gw, Table: TableStrongswan}
			err = netlink.RouteAdd(&route)
			if err != nil && !fileExistsError(err) {
				return err
			}
		}
	}

	return nil
}

func delStaleRoutes(conf netconf.NetworkConf) error {
	var routeFilter = &netlink.Route{
		Table: TableStrongswan,
	}
	routes, err := netlink.RouteListFiltered(netlink.FAMILY_V4, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}

	for _, r := range routes {
		if yes, err := IsActive(r.Dst, conf); err == nil && !yes {
			err = delRoute(r.Dst)
		}
	}

	return err
}

func IsActive(dst *net.IPNet, conf netconf.NetworkConf) (bool, error) {
	for _, peer := range conf.Peers {
		for _, subnet := range peer.Subnets {
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

func delRoute(subnet *net.IPNet) error {
	gw, err := getDefaultGateway()
	if err != nil {
		return err
	}
	route := netlink.Route{Dst: subnet, Gw: gw, Table: TableStrongswan}
	return netlink.RouteDel(&route)
}

func fileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}
