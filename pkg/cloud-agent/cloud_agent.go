package cloud_agent

import (
	"encoding/json"
	"fmt"
	"github.com/bep/debounce"
	"github.com/fabedge/fabedge/pkg/common/about"
	"github.com/fabedge/fabedge/pkg/connector/routing"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	"github.com/fabedge/fabedge/pkg/util/memberlist"
	routeUtil "github.com/fabedge/fabedge/pkg/util/route"
	flag "github.com/spf13/pflag"
	"github.com/vishvananda/netlink"
	"k8s.io/klog/v2"
	"net"
	"time"
)

var (
	initMembers []string
	debounced   = debounce.New(time.Second * 10)
	addedRoutes = map[string][]netlink.Route{}
)

func init() {
	logutil.AddFlags(flag.CommandLine)
	flag.StringSliceVar(&initMembers, "connector-node-addresses", []string{}, "internal ip address of all connector nodes")
}

func getRouteTmpl(prefix string) (netlink.Route, error) {
	ip, _, err := net.ParseCIDR(prefix)
	if err != nil {
		return netlink.Route{}, err
	}

	routes, err := netlink.RouteGet(ip)
	if err != nil || len(routes) < 1 {
		return netlink.Route{}, err
	}

	r := netlink.Route{}
	r.Flags = int(netlink.FLAG_ONLINK)
	r.Gw = routes[0].Gw
	r.Dst = routes[0].Dst
	r.LinkIndex = routes[0].LinkIndex

	return r, nil
}

func addAndSaveRoutes(cp routing.ConnectorPrefixes) error {
	if len(cp.RemotePrefixes) < 1 {
		return nil
	}

	// get the route to connector's local prefix and save it as a template
	rt, err := getRouteTmpl(cp.LocalPrefixes[0])
	if err != nil {
		return err
	}

	// add all remote routes, which are rendered with the template saved before
	var routes []netlink.Route
	for _, p := range cp.RemotePrefixes {
		_, prefix, err := net.ParseCIDR(p)
		if err != nil {
			return err
		}
		rt.Dst = prefix

		klog.V(5).Infof("add route: %+v", rt)
		if err = netlink.RouteAdd(&rt); err != nil {
			if !routeUtil.FileExistsError(err) {
				return fmt.Errorf("failed to add route:%+v with error:%s", rt, err)
			}
		}

		// save the route, for the sake to remove it once the node left
		routes = append(routes, rt)
	}

	addedRoutes[cp.Name] = routes

	return nil
}

func msgHandler(b []byte) {
	debounced(func() {
		var cp routing.ConnectorPrefixes
		if err := json.Unmarshal(b, &cp); err != nil {
			klog.Errorf("failed to unmarshal message:%s", err)
		}
		klog.V(5).Infof("get connector message:%+v", cp)

		if err := addAndSaveRoutes(cp); err != nil {
			klog.Errorf("failed to add route:%s", err)
		}
	})
}

func delAllSavedRoutesByNode(name string) {
	if _, ok := addedRoutes[name]; ok {
		for _, r := range addedRoutes[name] {
			klog.V(5).Infof("delete route: %+v", r)
			if err := netlink.RouteDel(&r); err != nil {
				if !routeUtil.NoSuchProcessError(err) {
					klog.Errorf("failed to delete route:%+v with error:%s", r, err)
				}
			}
		}
		delete(addedRoutes, name)
	}
}

func nodeLeaveHandler(name string) {
	debounced(func() {
		klog.V(5).Infof("node %s leave, to delete all routes via it", name)
		delAllSavedRoutesByNode(name)
	})
}

func Execute() {
	flag.Parse()

	about.DisplayVersion()

	mc, err := memberlist.New(msgHandler, nodeLeaveHandler)
	if err != nil {
		klog.Exit(err)
	}

	if len(initMembers) < 1 {
		klog.Exit("at least one connector node address is needed")
	}

	err = mc.Run(initMembers)
	if err != nil {
		klog.Exit(err)
	}

	for {
		if len(mc.ListMembers()) < 2 {
			klog.Exit("lost connection to connectors, exit")
		}

		for _, member := range mc.ListMembers() {
			klog.V(5).Infof("Member: %s %s\n", member.Name, member.Addr)
		}

		time.Sleep(time.Minute * 5)
	}
}
