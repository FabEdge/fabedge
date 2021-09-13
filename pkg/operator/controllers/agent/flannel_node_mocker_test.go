package agent

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"

	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("FlannelNodeMocker", func() {
	var (
		handler *flannelNodeMocker
		newNode = newNodeUsingRawPodCIDRs
	)

	BeforeEach(func() {
		handler = &flannelNodeMocker{
			client: k8sClient,
			log:    klogr.New().WithName("podCIDRsHandler"),
		}
	})

	AfterEach(func() {
		Expect(testutil.PurgeAllNodes(k8sClient)).Should(Succeed())
	})

	It("let a edge node mock a connector node by copying connector node's flannel annotations", func() {
		edgeNode := newNode(getNodeName(), "10.40.20.181", "2.2.2.2/26")
		connectorNode := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "connector",
				Labels: map[string]string{
					KeyNodeRoleConnector: "",
				},
				Annotations: map[string]string{
					"flannel.alpha.coreos.com/backend-data":        "{\"VtepMAC\":\"ea:c3:d2:22:d3:23\"}",
					"flannel.alpha.coreos.com/backend-type":        "vxlan",
					"flannel.alpha.coreos.com/kube-subnet-manager": "true",
					"flannel.alpha.coreos.com/public-ip":           "10.10.10.10",
				},
			},
		}

		ctx := context.Background()
		Expect(k8sClient.Create(ctx, &edgeNode)).Should(Succeed())
		Expect(k8sClient.Create(ctx, &connectorNode)).Should(Succeed())

		Expect(handler.Do(ctx, edgeNode)).Should(Succeed())

		Expect(k8sClient.Get(ctx, ObjectKey{Name: edgeNode.Name}, &edgeNode)).Should(Succeed())
		for key, value := range connectorNode.Annotations {
			Expect(edgeNode.Annotations).Should(HaveKeyWithValue(key, value))
		}
	})
})
