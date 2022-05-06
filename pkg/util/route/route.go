package route

import (
	"net"
	"os"
	"strings"

	"github.com/vishvananda/netlink"
)

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

func GetIPv6DefaultGateway() (net.IP, error) {
	defaultRoute, err := netlink.RouteGet(net.ParseIP("2001:4860:4860::8888"))
	if len(defaultRoute) != 1 || err != nil {
		return nil, err
	}
	return defaultRoute[0].Gw, nil
}

func GetNodeName() string {
	n, err := os.Hostname()
	if err != nil {
		return ""
	}

	return n
}
