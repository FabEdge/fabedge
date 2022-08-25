package routines

import (
	"context"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

// LocalClusterReporter create or update cluster data in the cluster where
// controller is running
type LocalClusterReporter struct {
	Cluster      string
	ClusterCIDRs []string
	GetConnector types.EndpointGetter
	SyncInterval time.Duration
	Client       client.Client
	Log          logr.Logger
}

func (ctl *LocalClusterReporter) Start(ctx context.Context) error {
	tick := time.NewTicker(ctl.SyncInterval)

	ctl.report(ctx)
	for {
		select {
		case <-tick.C:
			ctl.report(ctx)
		case <-ctx.Done():
			return nil
		}
	}
}

func (ctl *LocalClusterReporter) report(ctx context.Context) {
	connector := ctl.GetConnector()

	cluster := apis.Cluster{}
	err := ctl.Client.Get(ctx, client.ObjectKey{Name: ctl.Cluster}, &cluster)
	if err != nil {
		if !errors.IsNotFound(err) {
			ctl.Log.Error(err, "failed to get cluster")
			return
		}

		cluster = apis.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: ctl.Cluster,
			},
			Spec: apis.ClusterSpec{
				Token: "",
				CIDRs: ctl.ClusterCIDRs,
				EndPoints: []apis.Endpoint{
					connector,
				},
			},
		}

		if err = ctl.Client.Create(ctx, &cluster); err != nil {
			ctl.Log.Error(err, "failed to create cluster")
		}
		return
	}

	endpoints := []apis.Endpoint{
		connector,
	}

	if reflect.DeepEqual(endpoints, cluster.Spec.EndPoints) && reflect.DeepEqual(ctl.ClusterCIDRs, cluster.Spec.CIDRs) {
		return
	}

	cluster.Spec.EndPoints = endpoints
	cluster.Spec.CIDRs = ctl.ClusterCIDRs
	if err = ctl.Client.Update(ctx, &cluster); err != nil {
		ctl.Log.Error(err, "failed to update cluster")
	}
}
