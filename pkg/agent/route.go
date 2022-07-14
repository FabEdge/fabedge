package agent

import (
	"net"

	"k8s.io/apimachinery/pkg/util/sets"

	routeutil "github.com/fabedge/fabedge/pkg/util/route"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

func addRoutesToPeerViaGateway(gw net.IP, peer Endpoint) error {
	return routeutil.EnsureStrongswanRoutes(peer.Subnets, gw)
}

func addRoutesToPeer(peer Endpoint) error {
	var errors []error
	for _, nodeSubnet := range peer.NodeSubnets {
		gw := net.ParseIP(nodeSubnet)
		if gw == nil {
			continue
		}

		if err := addRoutesToPeerViaGateway(gw, peer); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func delStaleRoutes(peers []Endpoint) error {
	dstSet := sets.NewString()
	for _, peer := range peers {
		dstSet.Insert(peer.Subnets...)
	}

	return routeutil.PurgeStrongSwanRoutes(routeutil.NewDstWhitelist(dstSet))
}
