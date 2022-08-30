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

package community

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("Controller", func() {
	var (
		requests      chan reconcile.Request
		communityChan chan event.GenericEvent
		store         storepkg.Interface
		ctx           context.Context
		cancel        context.CancelFunc
	)

	BeforeEach(func() {
		store = storepkg.NewStore()
		communityChan = make(chan event.GenericEvent, 100)
		ctx, cancel = context.WithCancel(context.Background())

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ShouldNot(HaveOccurred())

		reconciler := reconcile.Reconciler(&communityController{
			client:        mgr.GetClient(),
			communityChan: communityChan,
			store:         store,
			log:           mgr.GetLogger().WithName(controllerName),
		})
		reconciler, requests = testutil.WrapReconcile(reconciler)
		c, err := controller.New(
			controllerName,
			mgr,
			controller.Options{
				Reconciler: reconciler,
			},
		)
		Expect(err).ShouldNot(HaveOccurred())

		err = c.Watch(
			&source.Kind{Type: &apis.Community{}},
			&handler.EnqueueRequestForObject{},
		)
		Expect(err).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()

		store.SaveEndpoint(apis.Endpoint{
			ID:              "1",
			Name:            "edge1",
			PublicAddresses: []string{"192.168.1.1"},
			Subnets:         []string{"2.2.2.1/26"},
		})
		store.SaveEndpoint(apis.Endpoint{
			ID:              "2",
			Name:            "edge2",
			PublicAddresses: []string{"192.168.1.2"},
			Subnets:         []string{"2.2.2.65/26"},
		})
		store.SaveEndpoint(apis.Endpoint{
			ID:              "4",
			Name:            "edge4",
			PublicAddresses: []string{"192.168.1.4"},
			Subnets:         []string{"2.2.2.130/26"},
		})
	})

	AfterEach(func() {
		cancel()

		var communities apis.CommunityList
		err := k8sClient.List(context.Background(), &communities)
		Expect(err).ShouldNot(HaveOccurred())

		for _, cmm := range communities.Items {
			Expect(k8sClient.Delete(context.Background(), &cmm))
		}
	})

	It("should save community and update communities of related endpoints in store", func() {
		var community apis.Community
		community = apis.Community{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: apis.CommunitySpec{
				Members: []string{
					"edge1",
					"edge2",
					"edge3",
				},
			},
		}

		err := k8sClient.Create(context.Background(), &community)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(requests, 5*time.Second).Should(Receive(Equal(reconcile.Request{
			NamespacedName: ObjectKey{Name: community.Name},
		})))

		cmm, ok := store.GetCommunity(community.Name)
		Expect(ok).Should(BeTrue())

		for _, member := range community.Spec.Members {
			Expect(cmm.Members.List()).Should(ContainElement(member))
		}
	})

	It("should clear community in store", func() {
		var community apis.Community
		community = apis.Community{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: apis.CommunitySpec{
				Members: []string{
					"edge1",
					"edge2",
					"edge3",
				},
			},
		}

		err := k8sClient.Create(context.Background(), &community)
		Expect(err).ShouldNot(HaveOccurred())

		testutil.DrainChan(requests, 5*time.Second)

		Expect(k8sClient.Delete(context.Background(), &community)).ShouldNot(HaveOccurred())
		Eventually(requests, 5*time.Second).Should(Receive(Equal(reconcile.Request{
			NamespacedName: ObjectKey{Name: community.Name},
		})))

		_, ok := store.GetCommunity(community.Name)
		Expect(ok).Should(BeFalse())
	})

	It("should send community event no matter whether what happened to a community", func() {
		var community apis.Community
		community = apis.Community{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: apis.CommunitySpec{
				Members: []string{
					"edge1",
					"edge2",
					"edge3",
				},
			},
		}

		By("create community")
		Expect(k8sClient.Create(context.Background(), &community)).To(Succeed())
		expectCommunityEvent(community, communityChan, 5*time.Second)

		By("delete community")
		Expect(k8sClient.Delete(context.Background(), &community)).To(Succeed())
		Eventually(requests, 5*time.Second).Should(Receive(Equal(reconcile.Request{
			NamespacedName: ObjectKey{Name: community.Name},
		})))
		expectCommunityEvent(community, communityChan, 5*time.Second)
	})
})

func drainCommunityChan(ch chan event.GenericEvent, timeout time.Duration) *event.GenericEvent {
	for {
		select {
		case evt := <-ch:
			return &evt
		case <-time.After(timeout):
			return nil
		}
	}
}

func expectCommunityEvent(expectedCommunity apis.Community, ch chan event.GenericEvent, timeout time.Duration) {
	evt := drainCommunityChan(ch, timeout)
	ExpectWithOffset(1, evt).NotTo(BeNil())

	community, ok := evt.Object.(*apis.Community)
	ExpectWithOffset(1, ok).Should(BeTrue())
	ExpectWithOffset(1, community.Name).To(Equal(expectedCommunity.Name))
	ExpectWithOffset(1, community.Spec.Members).To(Equal(expectedCommunity.Spec.Members))
}
