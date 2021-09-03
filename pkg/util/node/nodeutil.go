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

package node

import (
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/fabedge/fabedge/pkg/common/constants"
)

func GetIP(node corev1.Node) string {
	var ip string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ip = addr.Address
		}
	}

	return ip
}

func GetPodCIDRs(node corev1.Node) []string {
	switch {
	case len(node.Spec.PodCIDRs) > 0:
		return node.Spec.PodCIDRs
	case len(node.Spec.PodCIDR) > 0:
		return []string{node.Spec.PodCIDR}
	default:
		return nil
	}
}

func GetPodCIDRsFromAnnotation(node corev1.Node) []string {
	annotations := node.Annotations
	if annotations == nil {
		return nil
	}

	return strings.Split(annotations[constants.KeyPodSubnets], ",")
}

func IsEdgeNode(node corev1.Node) bool {
	labels := node.GetLabels()
	if labels == nil {
		return false
	}

	_, ok := labels["node-role.kubernetes.io/edge"]
	return ok
}
