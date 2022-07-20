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
