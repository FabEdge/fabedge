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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/constants"
)

type GetIDFunc func(nodeName string) string
type GetNameFunc func(nodeName string) string
type NewEndpointFunc func(node corev1.Node) apis.Endpoint
type PodCIDRsGetter func(node corev1.Node) []string
type EndpointGetter func() apis.Endpoint
type GetClusterCIDRInfo func() (map[string][]string, error)

func NewEndpointFuncs(namePrefix, idFormat string, getPodCIDRs PodCIDRsGetter) (GetNameFunc, GetIDFunc, NewEndpointFunc) {
	getName := func(name string) string {
		return fmt.Sprintf("%s.%s", namePrefix, name)
	}

	getID := func(name string) string {
		return strings.ReplaceAll(idFormat, "{node}", getName(name))
	}

	newEndpoint := func(node corev1.Node) apis.Endpoint {
		var nodeSubnets []string
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				nodeSubnets = append(nodeSubnets, addr.Address)
			}
		}

		publicAddresses := getPublicAddressesFromAnnotations(node)
		if len(publicAddresses) == 0 {
			publicAddresses = nodeSubnets
		}

		if node.Name == "" {
			return apis.Endpoint{}
		}

		return apis.Endpoint{
			ID:              getID(node.Name),
			Name:            getName(node.Name),
			PublicAddresses: publicAddresses,
			Subnets:         getPodCIDRs(node),
			NodeSubnets:     nodeSubnets,
			Type:            apis.EdgeNode,
		}
	}

	return getName, getID, newEndpoint
}

func getPublicAddressesFromAnnotations(node corev1.Node) []string {
	if len(node.Annotations) == 0 {
		return nil
	}

	publicAddresses := node.Annotations[constants.KeyNodePublicAddresses]
	if len(publicAddresses) == 0 {
		return nil
	}

	return strings.Split(publicAddresses, ",")
}
