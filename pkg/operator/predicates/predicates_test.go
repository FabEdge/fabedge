package predicates

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestEdgeNodePredicate(t *testing.T) {
	nodeutil.SetEdgeNodeLabels(map[string]string{"edge": ""})
	g := NewGomegaWithT(t)

	predicate := EdgeNodePredicate()
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"edge": "",
			},
		},
	}

	g.Expect(predicate.Create(event.CreateEvent{Object: &node})).To(BeTrue())
	g.Expect(predicate.Update(event.UpdateEvent{ObjectNew: &node, ObjectOld: &node})).To(BeTrue())
	g.Expect(predicate.Delete(event.DeleteEvent{Object: &node})).To(BeTrue())
	g.Expect(predicate.Generic(event.GenericEvent{Object: &node})).To(BeTrue())
}

func TestNonEdgeNodePredicate(t *testing.T) {
	nodeutil.SetEdgeNodeLabels(map[string]string{"connector": ""})
	g := NewGomegaWithT(t)

	predicate := NonEdgeNodePredicate()
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"edge": "",
			},
		},
	}
	g.Expect(predicate.Create(event.CreateEvent{Object: &node})).To(BeFalse())
	g.Expect(predicate.Update(event.UpdateEvent{ObjectNew: &node, ObjectOld: &node})).To(BeFalse())
	g.Expect(predicate.Delete(event.DeleteEvent{Object: &node})).To(BeFalse())
	g.Expect(predicate.Generic(event.GenericEvent{Object: &node})).To(BeFalse())
}
