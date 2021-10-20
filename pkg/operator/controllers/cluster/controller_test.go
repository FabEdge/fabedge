package cluster

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/fabedge/fabedge/pkg/operator/apis/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	. "github.com/fabedge/fabedge/pkg/util/ginkgoext"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("Controller", func() {
	var (
		requests chan reconcile.Request
		ctx      context.Context
		cancel   context.CancelFunc
		cluster  apis.Cluster
		ctrl     *controller
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())

		ctrl = &controller{
			Config: Config{
				Cluster: "test",
				Store:   storepkg.NewStore(),
			},
			clusterCache: make(map[string]EndpointNameSet),
			client:       mgr.GetClient(),
			log:          mgr.GetLogger().WithName(controllerName),
		}
		reconciler := reconcile.Reconciler(ctrl)
		reconciler, requests = testutil.WrapReconcile(reconciler)
		c, err := ctrlpkg.New(
			controllerName,
			mgr,
			ctrlpkg.Options{
				Reconciler: reconciler,
			},
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = c.Watch(
			&source.Kind{Type: &apis.Cluster{}},
			&handler.EnqueueRequestForObject{},
		)
		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()

		cluster = apis.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "root",
			},
			Spec: apis.ClusterSpec{
				Token: "test",
				EndPoints: []apis.TunnelEndpoint{
					{
						Name: "root.connector",
						PublicAddresses: []string{
							"10.10.10.10",
							"test.example",
						},
						Subnets: []string{
							"2.2.0.0/16",
						},
						NodeSubnets: []string{
							"192.168.1.1",
						},
					},
					{
						Name: "root.edge1",
						PublicAddresses: []string{
							"10.10.10.1",
						},
						Subnets: []string{
							"2.3.1.0/24",
						},
						NodeSubnets: []string{
							"192.168.1.2",
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(context.Background(), &cluster)).To(Succeed())
		Eventually(requests, 5*time.Second).Should(ReceiveKey(client.ObjectKey{
			Name: cluster.Name,
		}))
	})

	AfterEach(func() {
		cancel()
		Expect(k8sClient.Delete(context.Background(), &cluster))
	})

	It("should save endpoints of cluster to store when a new cluster is created", func() {
		nameSet, ok := ctrl.clusterCache[cluster.Name]
		Expect(ok).Should(BeTrue())

		for _, ep := range cluster.Spec.EndPoints {
			Expect(nameSet.Has(ep.Name)).Should(BeTrue())

			ep2, ok := ctrl.Store.GetEndpoint(ep.Name)
			Expect(ok).Should(BeTrue())
			Expect(ep2).Should(Equal(convertTunnelEndpoint(ep)))
		}
	})

	It("should update endpoints of cluster to store when cluster is updated", func() {
		cluster.Spec.EndPoints = []apis.TunnelEndpoint{
			{
				Name: "root.connector",
				PublicAddresses: []string{
					"10.10.10.10",
				},
				Subnets: []string{
					"2.2.0.0/16",
				},
				NodeSubnets: []string{
					"192.168.1.1",
				},
			},
		}

		Expect(k8sClient.Update(context.Background(), &cluster)).Should(Succeed())
		Eventually(requests, 5*time.Second).Should(ReceiveKey(client.ObjectKey{
			Name: cluster.Name,
		}))

		nameSet, _ := ctrl.clusterCache[cluster.Name]
		Expect(nameSet.Has("root.connector")).Should(BeTrue())
		Expect(nameSet.Has("root.edge1")).Should(BeFalse())

		_, ok := ctrl.Store.GetEndpoint("root.edge1")
		Expect(ok).Should(BeFalse())

		ep := cluster.Spec.EndPoints[0]
		ep2, _ := ctrl.Store.GetEndpoint("root.connector")
		Expect(ep2).Should(Equal(convertTunnelEndpoint(ep)))
	})

	It("should delete endpoints from store when cluster is deleted", func() {
		Expect(k8sClient.Delete(context.Background(), &cluster)).Should(Succeed())
		Eventually(requests, 5*time.Second).Should(ReceiveKey(client.ObjectKey{
			Name: cluster.Name,
		}))

		_, ok := ctrl.clusterCache[cluster.Name]
		Expect(ok).Should(BeFalse())

		for _, ep := range cluster.Spec.EndPoints {
			_, ok := ctrl.Store.GetEndpoint(ep.Name)
			Expect(ok).Should(BeFalse())
		}
	})

	It("will skip cluster with name specified in controller", func() {
		cluster = apis.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: ctrl.Cluster,
			},
			Spec: apis.ClusterSpec{
				Token: "test",
				EndPoints: []apis.TunnelEndpoint{
					{
						Name: "test.connector",
						PublicAddresses: []string{
							"10.10.10.10",
							"test.example",
						},
						Subnets: []string{
							"2.2.0.0/16",
						},
						NodeSubnets: []string{
							"192.168.1.1",
						},
					},
				},
			},
		}

		Expect(k8sClient.Create(context.Background(), &cluster)).Should(Succeed())
		Eventually(requests, 5*time.Second).Should(ReceiveKey(client.ObjectKey{
			Name: cluster.Name,
		}))

		_, ok := ctrl.clusterCache[cluster.Name]
		Expect(ok).Should(BeFalse())

		_, ok = ctrl.Store.GetEndpoint(cluster.Spec.EndPoints[0].Name)
		Expect(ok).Should(BeFalse())
	})
})
