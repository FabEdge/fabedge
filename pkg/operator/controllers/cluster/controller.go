package cluster

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
)

const (
	controllerName = "cluster-controller"
)

type EndpointNameSet = sets.String

type controller struct {
	Config
	client client.Client
	log    logr.Logger

	mux          sync.Mutex
	clusterCache map[string]EndpointNameSet
}

type Config struct {
	Cluster string
	Store   storepkg.Interface
	Manager manager.Manager
}

func AddToManager(config Config) error {
	mgr := config.Manager
	ctl, err := ctrlpkg.New(
		controllerName,
		mgr,
		ctrlpkg.Options{
			Reconciler: &controller{
				Config:       config,
				client:       mgr.GetClient(),
				log:          mgr.GetLogger().WithName(controllerName),
				clusterCache: make(map[string]EndpointNameSet),
			},
		},
	)
	if err != nil {
		return err
	}

	return ctl.Watch(
		&source.Kind{Type: &apis.Cluster{}},
		&handler.EnqueueRequestForObject{},
	)
}

func (ctl *controller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if request.Name == ctl.Cluster {
		return reconcile.Result{}, nil
	}

	var cluster apis.Cluster
	if err := ctl.client.Get(ctx, request.NamespacedName, &cluster); err != nil {
		if errors.IsNotFound(err) {
			ctl.log.Info("cluster is deleted, clearing its endpoints from store", "req", request)
			ctl.pruneEndpoints(request.Name)
			return reconcile.Result{}, nil
		}

		ctl.log.Error(err, "failed to get cluster", "req", request)
		return reconcile.Result{}, err
	}

	if cluster.DeletionTimestamp != nil {
		ctl.pruneEndpoints(request.Name)
		return reconcile.Result{}, nil
	}

	// for now, endpoints will contain only connector of every cluster
	nameSet := sets.NewString()
	for _, endpoint := range cluster.Spec.EndPoints {
		if len(endpoint.PublicAddresses) == 0 || len(endpoint.Subnets) == 0 || len(endpoint.NodeSubnets) == 0 {
			continue
		}

		ctl.Store.SaveEndpoint(endpoint)
		nameSet.Insert(endpoint.Name)
	}

	ctl.mux.Lock()
	oldNameSet, ok := ctl.clusterCache[cluster.Name]
	ctl.clusterCache[cluster.Name] = nameSet
	ctl.mux.Unlock()

	if !ok {
		return reconcile.Result{}, nil
	}

	for name := range oldNameSet.Difference(nameSet) {
		ctl.Store.DeleteEndpoint(name)
	}

	return reconcile.Result{}, nil
}

func (ctl *controller) pruneEndpoints(clusterName string) {
	ctl.mux.Lock()
	defer ctl.mux.Unlock()

	nameSet, ok := ctl.clusterCache[clusterName]
	if !ok {
		return
	}

	for epName := range nameSet {
		ctl.Store.DeleteEndpoint(epName)
	}
	delete(ctl.clusterCache, clusterName)
}
