package route

import (
	"net"
	"strings"

	"github.com/vishvananda/netlink"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/common/constants"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
)

type CheckRouteFunc func(route netlink.Route) bool

// NewDstWhitelist return a CheckRouteFunc which
// return true if specified route's Dst is not in set
func NewDstWhitelist(set sets.String) CheckRouteFunc {
	return func(route netlink.Route) bool {
		return !set.Has(route.Dst.String())
	}
}

// NewDstBlacklist return a CheckRouteFunc which
// return true if specified route's Dst is in set
func NewDstBlacklist(set sets.String) CheckRouteFunc {
	return func(route netlink.Route) bool {
		return set.Has(route.Dst.String())
	}
}

func FileExistsError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "file exists")
}

func NoSuchProcessError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no such process")
}

func GetDefaultGateway() (net.IP, error) {
	defaultRoute, err := netlink.RouteGet(net.ParseIP("8.8.8.8"))
	if len(defaultRoute) != 1 || err != nil {
		return nil, err
	}
	return defaultRoute[0].Gw, nil
}

func GetDefaultGateway6() (net.IP, error) {
	defaultRoute, err := netlink.RouteGet(net.ParseIP("2001:4860:4860::8888"))
	if len(defaultRoute) != 1 || err != nil {
		return nil, err
	}
	return defaultRoute[0].Gw, nil
}

// PurgeStrongSwanRoutes will delete any route in strongswan table which satisfy checkRoute
func PurgeStrongSwanRoutes(checkRoute CheckRouteFunc) error {
	var routeFilter = &netlink.Route{
		Table: 220,
	}

	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, routeFilter, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}

	var errors []error
	for _, route := range routes {
		if !checkRoute(route) {
			continue
		}

		if err = netlink.RouteDel(&route); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func EnsureStrongswanRoutes(prefixes []string, gw net.IP) error {
	var errors []error

	for _, prefix := range prefixes {
		dst, err := netlink.ParseIPNet(prefix)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		if !netutil.IsCompatible(dst, gw) {
			continue
		}

		err = netlink.RouteReplace(&netlink.Route{Dst: dst, Gw: gw, Table: constants.TableStrongswan})
		if err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}
