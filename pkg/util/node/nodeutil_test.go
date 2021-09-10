package node_test

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
)

func TestIsEdgeNode(t *testing.T) {
	g := NewGomegaWithT(t)

	nodeutil.SetEdgeNodeLabels(map[string]string{
		"edge":    "",
		"managed": "true",
	})

	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"edge":    "",
				"managed": "true",
			},
		},
	}

	g.Expect(nodeutil.IsEdgeNode(node)).To(BeTrue())

	node.Labels["edge"] = "not-blank"
	g.Expect(nodeutil.IsEdgeNode(node)).To(BeFalse())

	node.Labels["edge"] = ""
	node.Labels["managed"] = "false"
	g.Expect(nodeutil.IsEdgeNode(node)).To(BeFalse())
}
