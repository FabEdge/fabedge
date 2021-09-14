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

package types

import (
	"net"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/fabedge/fabedge/pkg/common/netconf"
)

type NewEndpointFunc func(node corev1.Node) Endpoint
type PodCIDRsGetter func(node corev1.Node) []string
type EndpointGetter func() Endpoint

type Endpoint struct {
	ID              string
	Name            string
	PublicAddresses []string
	Subnets         []string
	NodeSubnets     []string
}

func (e Endpoint) Equal(o Endpoint) bool {
	return reflect.DeepEqual(e, o)
}

func (e Endpoint) IsValid() bool {
	if len(e.PublicAddresses) == 0 || len(e.NodeSubnets) == 0 || len(e.Subnets) == 0 {
		return false
	}

	for _, subnet := range e.Subnets {
		_, _, err := net.ParseCIDR(subnet)
		if err != nil {
			return false
		}
	}

	for _, subnet := range e.NodeSubnets {
		_, _, err := net.ParseCIDR(subnet)
		if err != nil {
			return false
		}
	}

	return true
}

func (e Endpoint) ConvertToTunnelEndpoint() netconf.TunnelEndpoint {
	return netconf.TunnelEndpoint{
		ID:              e.ID,
		PublicAddresses: e.PublicAddresses,
		Name:            e.Name,
		Subnets:         e.Subnets,
		NodeSubnets:     e.NodeSubnets,
	}
}

func GenerateNewEndpointFunc(idFormat string, getPodCIDRs PodCIDRsGetter) NewEndpointFunc {
	return func(node corev1.Node) Endpoint {
		var nodeSubnets []string
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				nodeSubnets = append(nodeSubnets, addr.Address)
			}
		}

		var id = ""
		if node.Name != "" {
			id = GetID(idFormat, node.Name)
		}

		return Endpoint{
			ID:   id,
			Name: node.Name,
			// todo: get public address from annotations or labels
			PublicAddresses: nodeSubnets,
			Subnets:         getPodCIDRs(node),
			NodeSubnets:     nodeSubnets,
		}
	}
}

func GetID(format, nodeName string) string {
	return strings.ReplaceAll(format, "{node}", nodeName)
}
