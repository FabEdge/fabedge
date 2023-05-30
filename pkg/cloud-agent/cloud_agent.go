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

package cloud_agent

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/bep/debounce"
	"github.com/coreos/go-iptables/iptables"
	flag "github.com/spf13/pflag"
	"github.com/vishvananda/netlink"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/connector/routing"
	logutil "github.com/fabedge/fabedge/pkg/util/log"
	"github.com/fabedge/fabedge/pkg/util/memberlist"
	routeutil "github.com/fabedge/fabedge/pkg/util/route"
)

var (
	logger                 = klogr.New().WithName("cloudAgent")
	errAtLeaseOneConnector = fmt.Errorf("at least one connector node address is needed")
)

type cloudAgent struct {
	debounce func(f func())
	iph      *IptablesHandler
	iph6     *IptablesHandler

	routesLock   sync.RWMutex
	routesByHost map[string][]netlink.Route
}

func Execute() {
	defer klog.Flush()

	var initMembers []string
	flag.StringSliceVar(&initMembers, "connector-node-addresses", []string{}, "internal ip address of all connector nodes")
	logutil.AddFlags(flag.CommandLine)
	about.AddFlags(flag.CommandLine)

	flag.Parse()

	about.DisplayAndExitIfRequested()

	if len(initMembers) < 1 {
		logger.V(0).Error(errAtLeaseOneConnector, "not enough initial members for memberlist")
		os.Exit(1)
	}

	agent, err := newCloudAgent()
	if err != nil {
		logger.Error(err, "failed to create cloud agent")
		os.Exit(1)
	}

	mc, err := memberlist.New(initMembers, agent.handleMessage, agent.handleNodeLeave)
	if err != nil {
		logger.Error(err, "failed to create memberlist")
		os.Exit(1)
	}

	for {
		if len(mc.ListMembers()) < 2 {
			logger.Error(errAtLeaseOneConnector, "lost connection to connectors, exit")
			os.Exit(1)
		}

		for _, member := range mc.ListMembers() {
			logger.V(5).Info("Got Member", "name", member.Name, "addr", member.Addr)
		}
		time.Sleep(time.Minute * 5)
	}
}

func newCloudAgent() (*cloudAgent, error) {
	iph, err := newIptableHandler(iptables.ProtocolIPv4)
	if err != nil {
		return nil, err
	}

	iph6, err := newIptableHandler(iptables.ProtocolIPv6)
	if err != nil {
		return nil, err
	}

	if iph == nil && iph6 == nil {
		return nil, fmt.Errorf("at lease one iptablesHandler is required")
	}

	return &cloudAgent{
		iph:          iph,
		iph6:         iph6,
		debounce:     debounce.New(10 * time.Second),
		routesByHost: make(map[string][]netlink.Route),
	}, nil
}

func (a *cloudAgent) addAndSaveRoutes(cp routing.ConnectorPrefixes) {
	if a.iph != nil {
		go a.iph.maintainRules(cp.RemotePrefixes)
	}

	if a.iph6 != nil {
		go a.iph6.maintainRules(cp.RemotePrefixes6)
	}

	if err := addRouteRuleForStrongswan(); err != nil {
		logger.Error(err, "failed to add route rule for strongswan")
		return
	} else {
		logger.V(5).Info("ip rule is synced")
	}

	routes := a.syncRoutes(cp.LocalPrefixes, cp.RemotePrefixes)
	routes = append(routes, a.syncRoutes(cp.LocalPrefixes6, cp.RemotePrefixes6)...)

	whitelist := sets.NewString(cp.RemotePrefixes...)
	whitelist.Insert(cp.RemotePrefixes6...)
	if err := routeutil.PurgeStrongSwanRoutes(routeutil.NewDstWhitelist(whitelist)); err != nil {
		logger.Error(err, "failed to purge stale routes in strongswan table")
	}

	a.routesLock.Lock()
	a.routesByHost[cp.NodeName] = routes
	a.routesLock.Unlock()

	logger.V(5).Info("routes are synced", "routes", routes)
}

func (a *cloudAgent) syncRoutes(localPrefixes []string, remotePrefixes []string) []netlink.Route {
	if len(localPrefixes) == 0 || len(remotePrefixes) == 0 {
		logger.V(5).Info("no localPrefixes or no remotePrefixes, skip synchronizing routes")
		return nil
	}

	lp := localPrefixes[0]
	// get the route to connector's local prefix and save it as a template
	rt, err := getRouteTmpl(lp)
	if err != nil {
		logger.Error(err, "failed to get route for local prefix", "localPrefix", lp)
		return nil
	} else {
		logger.V(5).Info("get an route to connector's local prefix", "localPrefix", lp)
	}

	var routes []netlink.Route
	for _, rp := range remotePrefixes {
		_, dst, err := net.ParseCIDR(rp)
		if err != nil {
			logger.Error(err, "failed to parse a remote prefix", "remotePrefix", rp)
			continue
		}

		rt.Dst = dst
		rt.Table = constants.TableStrongswan

		if err = netlink.RouteReplace(&rt); err != nil {
			logger.Error(err, "failed to replace route", "route", rt)
			continue
		}

		// save the route, for the sake to remove it once the node left
		routes = append(routes, rt)
	}

	logger.V(5).Info("routes are synced")
	return routes
}

func (a *cloudAgent) handleMessage(msgBytes []byte) {
	a.debounce(func() {
		var cp routing.ConnectorPrefixes
		if err := json.Unmarshal(msgBytes, &cp); err != nil {
			logger.Error(err, "failed to unmarshal message")
			return
		} else {
			logger.V(5).Info("get connector message", "connectorPrefixes", cp)
		}

		a.addAndSaveRoutes(cp)
	})
}

func (a *cloudAgent) deleteRoutesByHost(host string) {
	routes := func() []netlink.Route {
		a.routesLock.Lock()
		a.routesLock.Unlock()

		rs := a.routesByHost[host]
		delete(a.routesByHost, host)

		return rs
	}()

	for _, r := range routes {
		if err := netlink.RouteDel(&r); err != nil {
			if !routeutil.NoSuchProcessError(err) {
				logger.Error(err, "failed to delete route", "route", r)
			}
		}
	}
}

func (a *cloudAgent) handleNodeLeave(name string) {
	logger.V(5).Info("A node has left, to delete all routes via it", "node", name)
	go a.deleteRoutesByHost(name)
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

func addRouteRuleForStrongswan() error {
	var errs []error

	for _, family := range []int{netlink.FAMILY_V4, netlink.FAMILY_V6} {
		if err := ensureStrongswanRouteRule(family); err != nil {
			errs = append(errs, err)
		}
	}

	return utilerrors.NewAggregate(errs)
}

func ensureStrongswanRouteRule(family int) error {
	rules, err := netlink.RuleList(family)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		if rule.Table == constants.TableStrongswan && rule.Priority == constants.TableStrongswan {
			return nil
		}
	}

	rule := netlink.NewRule()
	rule.Family = family
	rule.Priority = constants.TableStrongswan
	rule.Table = constants.TableStrongswan

	err = netlink.RuleAdd(rule)
	if err != nil && routeutil.FileExistsError(err) {
		err = nil
	}

	return err
}
