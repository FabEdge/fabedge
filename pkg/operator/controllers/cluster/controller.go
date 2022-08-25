package cluster

import (
	"context"
	"crypto/rsa"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/golang-jwt/jwt/v4"
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
	"github.com/fabedge/fabedge/pkg/operator/types"
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
	Cluster       string
	TokenDuration time.Duration
	PrivateKey    *rsa.PrivateKey
	Store         storepkg.Interface
	Manager       manager.Manager
	CIDRMap       *types.ClusterCIDRsMap
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
	log := ctl.log.WithValues("request", request)

	var cluster apis.Cluster
	if err := ctl.client.Get(ctx, request.NamespacedName, &cluster); err != nil {
		if errors.IsNotFound(err) {
			log.Info("cluster is deleted, clearing its endpoints from store")
			ctl.pruneEndpoints(request.Name)
			ctl.CIDRMap.Delete(request.Name)
			return reconcile.Result{}, nil
		}

		log.Error(err, "failed to get cluster")
		return reconcile.Result{}, err
	}
	ctl.syncClusterCIDRs(cluster)

	if cluster.Name == ctl.Cluster {
		log.V(5).Info("This cluster is local cluster, skip it")
		return reconcile.Result{}, nil
	}

	if cluster.DeletionTimestamp != nil {
		ctl.pruneEndpoints(request.Name)
		ctl.CIDRMap.Delete(request.Name)
		return reconcile.Result{}, nil
	}

	if err := ctl.generateTokenIfNeeded(ctx, cluster); err != nil {
		ctl.log.Error(err, "failed to assign token for cluster", "cluster", cluster.Name)
		return reconcile.Result{}, err
	}

	// for now, endpoints will contain only connector of every cluster
	ctl.syncEndpoints(cluster)

	return reconcile.Result{}, nil
}

func (ctl *controller) generateTokenIfNeeded(ctx context.Context, cluster apis.Cluster) error {
	if len(cluster.Spec.Token) != 0 {
		return nil
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.StandardClaims{
		Subject:   cluster.Name,
		ExpiresAt: time.Now().Add(ctl.TokenDuration).Unix(),
	})

	tokenString, err := token.SignedString(ctl.PrivateKey)
	if err != nil {
		return err
	}

	cluster.Spec.Token = tokenString
	return ctl.client.Update(ctx, &cluster)
}

func (ctl *controller) syncEndpoints(cluster apis.Cluster) {
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
		return
	}

	for name := range oldNameSet.Difference(nameSet) {
		ctl.Store.DeleteEndpoint(name)
	}
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

func (ctl *controller) syncClusterCIDRs(cluster apis.Cluster) {
	if len(cluster.Spec.CIDRs) > 0 {
		ctl.CIDRMap.Set(cluster.Name, cluster.Spec.CIDRs)
	} else {
		ctl.CIDRMap.Delete(cluster.Name)
	}
}
