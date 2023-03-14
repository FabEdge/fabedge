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
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/workqueue"
	ctrlpkg "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
)

// errRestartAgent is used to signal controller put restartAgent in context
var errRestartAgent = fmt.Errorf("restart agent")
var errRequeueRequest = fmt.Errorf("requeue this request")

const (
	controllerName            = "agent-controller"
	agentConfigTunnelFileName = "tunnels.yaml"

	keyRestartAgent = "restartAgent"
)

type ObjectKey = client.ObjectKey

type Handler interface {
	Do(ctx context.Context, node corev1.Node) error
	Undo(ctx context.Context, nodeName string) error
}

var _ reconcile.Reconciler = &agentController{}

type agentController struct {
	handlers    []Handler
	client      client.Client
	log         logr.Logger
	edgeNameSet *types.SafeStringSet
}

type Config struct {
	ClusterName string
	Allocators  []allocator.Interface
	Store       storepkg.Interface
	Manager     manager.Manager

	Namespace         string
	AgentImage        string
	StrongswanImage   string
	ImagePullPolicy   string
	AgentPodArguments types.AgentArgumentMap

	GetConnectorEndpoint types.EndpointGetter
	NewEndpoint          types.NewEndpointFunc
	GetEndpointName      types.GetNameFunc
	CommunityChan        <-chan event.GenericEvent

	CertManager      certutil.Manager
	CertOrganization string
}

func AddToManager(cnf Config) error {
	mgr := cnf.Manager

	log := mgr.GetLogger().WithName(controllerName)
	cli := mgr.GetClient()

	reconciler := &agentController{
		log:         log,
		client:      cli,
		edgeNameSet: types.NewSafeStringSet(),
		handlers:    initHandlers(cnf, cli, log),
	}

	return ctrlpkg.NewControllerManagedBy(mgr).
		For(&corev1.Node{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.Pod{}).
		Watches(&source.Channel{Source: cnf.CommunityChan}, newCommunityEventHandler(cnf.ClusterName, mgr.GetLogger())).
		Named(controllerName).
		Complete(reconciler)
}

func initHandlers(cnf Config, cli client.Client, log logr.Logger) []Handler {
	var handlers []Handler
	if len(cnf.Allocators) != 0 {
		handlers = append(handlers, &allocatablePodCIDRsHandler{
			store:           cnf.Store,
			allocators:      cnf.Allocators,
			newEndpoint:     cnf.NewEndpoint,
			getEndpointName: cnf.GetEndpointName,
			client:          cli,
			log:             log.WithName("podCIDRsHandler"),
		})
	} else {
		handlers = append(handlers, &rawPodCIDRsHandler{
			store:           cnf.Store,
			getEndpointName: cnf.GetEndpointName,
			newEndpoint:     cnf.NewEndpoint,
		})
	}

	handlers = append(handlers, &configHandler{
		namespace:            cnf.Namespace,
		client:               cli,
		store:                cnf.Store,
		getEndpointName:      cnf.GetEndpointName,
		getConnectorEndpoint: cnf.GetConnectorEndpoint,
		log:                  log.WithName("configHandler"),
	})

	handlers = append(handlers, &certHandler{
		namespace: cnf.Namespace,
		client:    cli,

		certManager:      cnf.CertManager,
		getEndpointName:  cnf.GetEndpointName,
		certOrganization: cnf.CertOrganization,

		log: log.WithName("certHandler"),
	})

	// agent pod created by agentPodHandler always use tls.crt as local-cert
	cnf.AgentPodArguments.Set("local-cert", "tls.crt")
	handlers = append(handlers, &agentPodHandler{
		namespace: cnf.Namespace,
		client:    cli,
		log:       log.WithName("agentPodHandler"),

		imagePullPolicy: corev1.PullPolicy(cnf.ImagePullPolicy),
		agentImage:      cnf.AgentImage,
		strongswanImage: cnf.StrongswanImage,
		argMap:          cnf.AgentPodArguments,
		args:            cnf.AgentPodArguments.ArgumentArray(),
		agentNameSet:    types.NewSafeStringSet(),
	})

	return handlers
}

func (ctl *agentController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := ctl.log.WithValues("key", request)

	var node corev1.Node
	if err := ctl.client.Get(ctx, request.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, ctl.clearAllocatedResourcesForEdgeNode(ctx, request.Name)
		}

		log.Error(err, "unable to get edge node")
		return reconcile.Result{}, err
	}

	if node.DeletionTimestamp != nil || !nodeutil.IsEdgeNode(node) {
		return reconcile.Result{}, ctl.clearAllocatedResourcesForEdgeNode(ctx, request.Name)
	}

	if ctl.shouldSkip(node) {
		log.V(5).Info("This node has no ip or pod CIDRs, skip reconciling")
		return reconcile.Result{}, nil
	}

	ctl.edgeNameSet.Insert(node.Name)
	for _, handler := range ctl.handlers {
		if err := handler.Do(ctx, node); err != nil {
			if err == errRestartAgent {
				ctx = context.WithValue(ctx, keyRestartAgent, err)
				continue
			} else if err == errRequeueRequest {
				return reconcile.Result{Requeue: true, RequeueAfter: 5 * time.Second}, nil
			}
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (ctl *agentController) shouldSkip(node corev1.Node) bool {
	return len(nodeutil.GetInternalIPs(node)) == 0
}

func (ctl *agentController) clearAllocatedResourcesForEdgeNode(ctx context.Context, nodeName string) error {
	if !ctl.edgeNameSet.Has(nodeName) {
		return nil
	}

	ctl.log.Info("clear resources allocated to this node", "nodeName", nodeName)
	for i := len(ctl.handlers) - 1; i >= 0; i-- {
		if err := ctl.handlers[i].Undo(ctx, nodeName); err != nil {
			return err
		}
	}

	ctl.edgeNameSet.Delete(nodeName)
	return nil
}

func newCommunityEventHandler(clusterName string, log logr.Logger) handler.EventHandler {
	prefix := fmt.Sprintf("%s.", clusterName)
	return handler.Funcs{
		GenericFunc: func(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
			if evt.Object == nil {
				log.Error(nil, "GenericEvent received with no metadata", "event", evt)
				return
			}
			community, ok := evt.Object.(*apis.Community)
			if !ok {
				log.Error(nil, "GenericEvent's object is not community", "event", evt)
				return
			}

			for _, name := range community.Spec.Members {
				if strings.HasPrefix(name, prefix) {
					name = strings.TrimPrefix(name, prefix)
				} else {
					continue
				}

				q.Add(reconcile.Request{NamespacedName: ObjectKey{
					Name: name,
				}})
			}
		},
	}
}
