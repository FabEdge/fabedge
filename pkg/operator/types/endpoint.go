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
	"strings"

	corev1 "k8s.io/api/core/v1"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
)

type IDFunc func(nodeName string) string
type NamingFunc func(nodeName string) string
type NewEndpointFunc func(node corev1.Node) apis.Endpoint
type PodCIDRsGetter func(node corev1.Node) []string
type EndpointGetter func() apis.Endpoint

func GenerateNewEndpointFunc(idFormat string, getName NamingFunc, getPodCIDRs PodCIDRsGetter) NewEndpointFunc {
	return func(node corev1.Node) apis.Endpoint {
		var nodeSubnets []string
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				nodeSubnets = append(nodeSubnets, addr.Address)
			}
		}

		if node.Name == "" {
			return apis.Endpoint{}
		}

		name := getName(node.Name)
		return apis.Endpoint{
			ID:   GetID(idFormat, name),
			Name: name,
			// todo: get public address from annotations or labels
			PublicAddresses: nodeSubnets,
			Subnets:         getPodCIDRs(node),
			NodeSubnets:     nodeSubnets,
			Type:            apis.EdgeNode,
		}
	}
}

func GetID(format, nodeName string) string {
	return strings.ReplaceAll(format, "{node}", nodeName)
}
