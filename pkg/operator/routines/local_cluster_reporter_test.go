package routines

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/operator/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ = Describe("LocalClusterReporter", func() {
	It("should create or update cluster", func() {
		connector := types.Endpoint{
			ID:              "connector",
			Name:            "connector",
			PublicAddresses: []string{"10.10.10.10"},
			Subnets:         []string{"2.2.0.0/2"},
			NodeSubnets:     []string{"192.168.1.1"},
		}

		reporter := &LocalClusterReporter{
			Cluster:      "test",
			Client:       k8sClient,
			SyncInterval: time.Second,
			Log:          klogr.New(),
			GetConnector: func() types.Endpoint {
				return connector
			},
		}

		By("first report")
		reporter.report(context.Background())

		By("check if cluster is created")
		var cluster apis.Cluster
		err := k8sClient.Get(context.Background(), client.ObjectKey{Name: reporter.Cluster}, &cluster)
		Expect(err).Should(BeNil())
		Expect(len(cluster.Spec.EndPoints)).Should(Equal(1))

		By("update connector and report again")
		connector.PublicAddresses = []string{"10.10.1.1"}
		reporter.report(context.Background())

		By("check if cluster is updated")
		err = k8sClient.Get(context.Background(), client.ObjectKey{Name: reporter.Cluster}, &cluster)
		Expect(err).Should(BeNil())

		Expect(cluster.Spec.EndPoints[0].PublicAddresses).Should(Equal(connector.PublicAddresses))
	})
})
