// Copyright 2021 BoCloud
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

package agent

import (
	"context"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	"github.com/fabedge/fabedge/pkg/operator/predicates"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	. "github.com/fabedge/fabedge/pkg/util/ginkgoext"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
	timeutil "github.com/fabedge/fabedge/pkg/util/time"
)

var _ = Describe("AgentController", func() {
	const (
		timeout         = 2 * time.Second
		namespace       = "default"
		agentImage      = "fabedge/agent:latest"
		strongswanImage = "strongswan:5.9.1"
	)

	var (
		requests    chan reconcile.Request
		store       storepkg.Interface
		alloc       allocator.Interface
		ctx         context.Context
		cancel      context.CancelFunc
		certManager certutil.Manager
		newNode     = newNodePodCIDRsInAnnotations

		newEndpoint = types.GenerateNewEndpointFunc("C=CN, O=StrongSwan, CN={node}", nodeutil.GetPodCIDRsFromAnnotation)
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		store = storepkg.NewStore()

		alloc, _ = allocator.New("2.2.0.0/16")

		caCertDER, caKeyDER, _ := certutil.NewSelfSignedCA(certutil.Config{
			CommonName:     certutil.DefaultCAName,
			Organization:   []string{certutil.DefaultOrganization},
			IsCA:           true,
			ValidityPeriod: timeutil.Days(365),
		})
		certManager, _ = certutil.NewManger(caCertDER, caKeyDER)

		mgr, err := manager.New(cfg, manager.Options{
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		cnf := Config{
			Namespace: namespace,

			AllocatePodCIDR: true,

			AgentImage:           agentImage,
			StrongswanImage:      strongswanImage,
			CertManager:          certManager,
			CertOrganization:     certutil.DefaultOrganization,
			CertValidPeriod:      365,
			Allocator:            alloc,
			Store:                store,
			NewEndpoint:          newEndpoint,
			GetConnectorEndpoint: getConnectorEndpoint,
		}

		reconciler := reconcile.Reconciler(&agentController{
			handlers: initHandlers(cnf, k8sClient, mgr.GetLogger()),
			client:   mgr.GetClient(),
			log:      mgr.GetLogger().WithName(controllerName),
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
			&source.Kind{Type: &corev1.Node{}},
			&handler.EnqueueRequestForObject{},
			predicates.EdgeNodePredicate(),
		)
		Expect(err).ShouldNot(HaveOccurred())

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(ctx)).NotTo(HaveOccurred())
		}()
	})

	AfterEach(func() {
		cancel()

		Expect(testutil.PurgeAllSecrets(k8sClient, client.InNamespace(namespace))).Should(Succeed())
		Expect(testutil.PurgeAllConfigMaps(k8sClient, client.InNamespace(namespace))).Should(Succeed())
		Expect(testutil.PurgeAllPods(k8sClient, client.InNamespace(namespace))).Should(Succeed())
		Expect(testutil.PurgeAllNodes(k8sClient, client.InNamespace(namespace))).Should(Succeed())
	})

	It("skip reconciling if this node has no ip", func() {
		nodeName := getNodeName()
		node := newNode(nodeName, "", "")

		err := k8sClient.Create(context.Background(), &node)
		Expect(err).ShouldNot(HaveOccurred())

		// create event
		Eventually(requests, timeout).Should(ReceiveKey(ObjectKey{Name: nodeName}))

		node = corev1.Node{}
		err = k8sClient.Get(context.Background(), ObjectKey{Name: nodeName}, &node)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(node.Annotations[constants.KeyPodSubnets]).Should(BeEmpty())
	})

	When("a node is created", func() {
		var nodeName string
		var node corev1.Node

		BeforeEach(func() {
			nodeName = getNodeName()
			node = newNode(nodeName, "10.40.20.181", "")

			err := k8sClient.Create(context.Background(), &node)
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(requests, timeout).Should(ReceiveKey(ObjectKey{Name: nodeName}))
		})

		It("should ensure a agent pod for each node", func() {
			var pod corev1.Pod
			agentPodName := getAgentPodName(nodeName)
			err := k8sClient.Get(context.Background(), ObjectKey{Namespace: namespace, Name: agentPodName}, &pod)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("should ensure a certificate/private key secret for each node", func() {
			var secret corev1.Secret
			secretName := getCertSecretName(nodeName)
			Expect(k8sClient.Get(ctx, ObjectKey{Namespace: namespace, Name: secretName}, &secret)).Should(Succeed())
		})

		It("should ensure a tunnels configmap for each node", func() {
			var cm corev1.ConfigMap
			configName := getAgentConfigMapName(nodeName)
			Expect(k8sClient.Get(ctx, ObjectKey{Namespace: namespace, Name: configName}, &cm)).Should(Succeed())
		})
	})
})

func newNodePodCIDRsInAnnotations(name, ip, subnets string) corev1.Node {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node-role.kubernetes.io/edge": "",
			},
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  "2.2.2.2/26",
			PodCIDRs: []string{"2.2.2.2/26"},
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: ip,
				},
			},
		},
	}

	if subnets != "" {
		node.Annotations = map[string]string{
			constants.KeyPodSubnets: subnets,
		}
	}

	return node
}

func newNodeUsingRawPodCIDRs(name, ip, subnets string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node-role.kubernetes.io/edge": "",
			},
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  subnets,
			PodCIDRs: strings.Split(subnets, ","),
		},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{
					Type:    corev1.NodeInternalIP,
					Address: ip,
				},
			},
		},
	}
}
