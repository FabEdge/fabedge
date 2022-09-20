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

package agent

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/types"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var _ = Describe("AgentController", func() {
	const (
		testNamespace = "fabedge"
	)

	var (
		newNode    = newNodePodCIDRsInAnnotations
		handlers   []Handler
		controller *agentController
	)

	JustBeforeEach(func() {
		log := klogr.New().WithName(controllerName)
		controller = &agentController{
			handlers:    handlers,
			edgeNameSet: types.NewSafeStringSet(),
			client:      k8sClient,
			log:         log,
		}
	})

	JustAfterEach(func() {
		Expect(testutil.PurgeAllSecrets(k8sClient, client.InNamespace(testNamespace))).Should(Succeed())
		Expect(testutil.PurgeAllConfigMaps(k8sClient, client.InNamespace(testNamespace))).Should(Succeed())
		Expect(testutil.PurgeAllPods(k8sClient, client.InNamespace(testNamespace))).Should(Succeed())
		Expect(testutil.PurgeAllNodes(k8sClient, client.InNamespace(testNamespace))).Should(Succeed())
	})

	Describe("Reconcile", func() {
		Context("node has neither edge labels nor ips", func() {
			var handler *FuncHandler
			BeforeEach(func() {
				handler = &FuncHandler{}
				handlers = []Handler{handler}
			})

			It("skip executing handlers if a node has no ip", func() {
				nodeName := getNodeName()
				node := newNode(nodeName, "", "")

				err := k8sClient.Create(context.Background(), &node)
				Expect(err).ShouldNot(HaveOccurred())

				_, err = controller.Reconcile(context.Background(), reconcile.Request{
					NamespacedName: ObjectKey{
						Name: nodeName,
					},
				})
				Expect(err).Should(BeNil())

				Expect(controller.edgeNameSet.Has(nodeName)).Should(BeFalse())
				Expect(handler.DoContext).Should(BeNil())
				Expect(handler.UndoContext).Should(BeNil())
			})

			It("skip executing handlers if a node has no edge labels", func() {
				nodeName := getNodeName()
				node := newNode(nodeName, "10.40.20.181", "")
				node.Labels = nil

				err := k8sClient.Create(context.Background(), &node)
				Expect(err).ShouldNot(HaveOccurred())

				_, err = controller.Reconcile(context.Background(), reconcile.Request{
					NamespacedName: ObjectKey{
						Name: nodeName,
					},
				})
				Expect(err).Should(BeNil())

				Expect(controller.edgeNameSet.Has(nodeName)).Should(BeFalse())
				Expect(handler.DoContext).Should(BeNil())
				Expect(handler.UndoContext).Should(BeNil())
			})
		})

		Context("node has edge labels and has ips", func() {
			var (
				nodeName     string
				node         corev1.Node
				firstHandler *FuncHandler
				lastHandler  *FuncHandler
			)

			BeforeEach(func() {
				firstHandler = &FuncHandler{
					ErrorForDo: errRestartAgent,
				}
				lastHandler = &FuncHandler{}
				handlers = []Handler{firstHandler, lastHandler}
			})

			JustBeforeEach(func() {
				nodeName = getNodeName()
				node = newNode(nodeName, "10.40.20.181", "")

				err := k8sClient.Create(context.Background(), &node)
				Expect(err).ShouldNot(HaveOccurred())

				_, err = controller.Reconcile(context.Background(), reconcile.Request{
					NamespacedName: ObjectKey{
						Name: nodeName,
					},
				})
				Expect(err).Should(BeNil())
			})

			It("should record node name in edgeNameSet", func() {
				Expect(controller.edgeNameSet.Has(nodeName)).Should(BeTrue())
			})

			It("execute Do method of each handlers in order", func() {
				Expect(firstHandler.DoContext).NotTo(BeNil())
				Expect(lastHandler.DoContext).NotTo(BeNil())

				Expect(firstHandler.UndoContext).To(BeNil())
				Expect(lastHandler.UndoContext).To(BeNil())
			})

			It("pass errRestartAgent in context to next handlers if a handler return errRestartAgent", func() {
				Expect(lastHandler.DoContext.Value(keyRestartAgent)).To(Equal(errRestartAgent))
			})

			It("return error if Do method of any handler return a error but errRestartAgent", func() {
				firstHandler = &FuncHandler{ErrorForDo: fmt.Errorf("some error")}
				lastHandler = &FuncHandler{}
				controller.edgeNameSet = types.NewSafeStringSet()
				controller.handlers = []Handler{firstHandler, lastHandler}

				_, err := controller.Reconcile(context.Background(), reconcile.Request{
					NamespacedName: ObjectKey{
						Name: nodeName,
					},
				})
				Expect(err).To(Equal(firstHandler.ErrorForDo))

				Expect(lastHandler.DoContext).To(BeNil())
				Expect(controller.edgeNameSet.Has(nodeName)).To(BeTrue())
			})

			It("return a requeue reconcile.Result if any handler return errRequeueRequest", func() {
				firstHandler = &FuncHandler{ErrorForDo: errRequeueRequest}
				lastHandler = &FuncHandler{}
				controller.edgeNameSet = types.NewSafeStringSet()
				controller.handlers = []Handler{firstHandler, lastHandler}

				req, err := controller.Reconcile(context.Background(), reconcile.Request{
					NamespacedName: ObjectKey{
						Name: nodeName,
					},
				})
				Expect(err).To(BeNil())
				Expect(req.Requeue).To(BeTrue())
				Expect(req.RequeueAfter).To(Equal(5 * time.Second))

				Expect(lastHandler.DoContext).To(BeNil())
				Expect(controller.edgeNameSet.Has(nodeName)).To(BeTrue())
			})

			When("node is deleted or lose edge labels", func() {
				DescribeTable("execute Undo method of each handlers", func(action func() error) {
					Expect(action()).Should(Succeed())

					_, err := controller.Reconcile(context.Background(), reconcile.Request{
						NamespacedName: ObjectKey{
							Name: nodeName,
						},
					})
					Expect(err).To(BeNil())

					Expect(lastHandler.UndoContext).NotTo(BeNil())
					Expect(firstHandler.UndoContext).NotTo(BeNil())
					Expect(controller.edgeNameSet.Has(nodeName)).To(BeFalse())
				},
					Entry("delete node", func() error {
						return k8sClient.Delete(context.Background(), &node)
					}),

					Entry("remove edge labels", func() error {
						node.Labels = nil
						return k8sClient.Update(context.Background(), &node)
					}),
				)

				It("stop execute Undo if Undo of any handler return error", func() {
					firstHandler = &FuncHandler{}
					lastHandler = &FuncHandler{ErrorForUndo: fmt.Errorf("some error")}
					controller.handlers = []Handler{firstHandler, lastHandler}

					Expect(k8sClient.Delete(context.Background(), &node)).To(Succeed())

					_, err := controller.Reconcile(context.Background(), reconcile.Request{
						NamespacedName: ObjectKey{
							Name: nodeName,
						},
					})
					Expect(err).To(Equal(lastHandler.ErrorForUndo))

					// also check if handlers are executed in reverse order
					Expect(lastHandler.UndoContext).NotTo(BeNil())
					Expect(firstHandler.UndoContext).To(BeNil())
				})
			})
		})
	})
})

var _ = Describe("newCommunityEventHandler", func() {
	var (
		handler handler.EventHandler
		queue   workqueue.RateLimitingInterface
	)

	BeforeEach(func() {
		handler = newCommunityEventHandler("fabedge", klogr.New())
		queue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "test")
	})

	AfterEach(func() {
		queue.ShutDown()
	})

	It("can trigger nodes events from community event", func() {
		community := apis.Community{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: apis.CommunitySpec{
				Members: []string{"fabedge.edge1", "fabedge.edge2"},
			},
		}
		handler.Generic(event.GenericEvent{Object: &community}, queue)

		data, _ := queue.Get()
		req, ok := data.(reconcile.Request)
		Expect(ok).Should(BeTrue())
		Expect(req.Name).To(Equal("edge1"))

		data, _ = queue.Get()
		req, ok = data.(reconcile.Request)
		Expect(ok).Should(BeTrue())
		Expect(req.Name).To(Equal("edge2"))
	})

	It("will ignore community members which doesn't match clusterName argument", func() {
		community := apis.Community{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: apis.CommunitySpec{
				Members: []string{"beijing.edge1", "fabedge.edge2"},
			},
		}
		handler.Generic(event.GenericEvent{Object: &community}, queue)

		data, _ := queue.Get()
		req, ok := data.(reconcile.Request)
		Expect(ok).Should(BeTrue())
		Expect(req.Name).To(Equal("edge2"))
	})

	It("will ignore non-community events", func() {
		handler.Generic(event.GenericEvent{Object: &corev1.Node{}}, queue)
		Expect(queue.Len()).To(Equal(0))
	})
})

var _ Handler = &FuncHandler{}

type FuncHandler struct {
	DoFunc   func(ctx context.Context, node corev1.Node) error
	UndoFunc func(ctx context.Context, nodeName string) error

	ErrorForDo   error
	ErrorForUndo error

	DoContext   context.Context
	UndoContext context.Context

	Node     corev1.Node
	NodeName string
}

func (fh *FuncHandler) Do(ctx context.Context, node corev1.Node) error {
	fh.DoContext = ctx
	fh.Node = node

	if fh.DoFunc == nil {
		return fh.ErrorForDo
	} else {
		return fh.DoFunc(ctx, node)
	}
}

func (fh *FuncHandler) Undo(ctx context.Context, nodeName string) error {
	fh.UndoContext = ctx
	fh.NodeName = nodeName

	if fh.UndoFunc == nil {
		return fh.ErrorForUndo
	} else {
		return fh.UndoFunc(ctx, nodeName)
	}
}
