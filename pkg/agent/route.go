package agent

import (
	"net"

	"github.com/vishvananda/netlink"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	routeutil "github.com/fabedge/fabedge/pkg/util/route"
)

func addRoutesToAllPeers(conf netconf.NetworkConf) error {
	gw, err := routeutil.GetDefaultGateway()
	if err != nil {
		return err
	}

	for _, peer := range conf.Peers {
		for _, subnet := range peer.Subnets {
			s, err := netlink.ParseIPNet(subnet)
			if err != nil {
				return err
			}

			// for debug
			// if netutil.IPVersion(s.IP) == netutil.IPV6 {
			// 	continue
			// }

			route := netlink.Route{Dst: s, Gw: gw, Table: constants.TableStrongswan}
			err = netlink.RouteAdd(&route)
			if err != nil && !routeutil.FileExistsError(err) {
				return err
			}
		}
	}

	return nil
}

func delStaleRoutes(conf netconf.NetworkConf) error {
	var routeFilter = &netlink.Route{
		Table: constants.TableStrongswan,
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
	gw, err := routeutil.GetDefaultGateway()
	if err != nil {
		return err
	}
	route := netlink.Route{Dst: subnet, Gw: gw, Table: constants.TableStrongswan}
	return netlink.RouteDel(&route)
}
