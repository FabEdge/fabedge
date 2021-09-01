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
