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
	"sync"

	corev1 "k8s.io/api/core/v1"

	"github.com/fabedge/fabedge/pkg/common/constants"
)

var once sync.Once
var edgeNodeLabels map[string]string

func SetEdgeNodeLabels(labels map[string]string) {
	once.Do(func() {
		edgeNodeLabels = labels
	})
}

func GetEdgeNodeLabels() map[string]string {
	return edgeNodeLabels
}

func GetInternalIPs(node corev1.Node) []string {
	var ips []string
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ips = append(ips, addr.Address)
		}
	}

	return ips
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
	if len(edgeNodeLabels) == 0 {
		return false
	}

	labels := node.GetLabels()
	if len(labels) == 0 {
		return false
	}

	for key, value := range edgeNodeLabels {
		if v, exist := labels[key]; !exist || v != value {
			return false
		}
	}

	return true
}
