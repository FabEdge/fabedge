package routines

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/operator/apiserver"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

type UpdateCluster func(cluster apis.Cluster) error
type UpdateEndpointsFunc func(endpoints []apis.Endpoint) error
type GetEndpointsAndCommunitiesFunc func() (apiserver.EndpointsAndCommunity, error)

// ExportCluster is used to export cluster CIDRs and endpoints to host cluster
func ExportCluster(interval time.Duration, clusterName string, clusterCIDRs []string, getConnector types.EndpointGetter, updateCluster UpdateCluster) manager.Runnable {
	log := klogr.New().WithName("exportEndpoints")

	fn := func(ctx context.Context) {
		cluster := apis.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
			Spec: apis.ClusterSpec{
				CIDRs: clusterCIDRs,
				EndPoints: []apis.Endpoint{
					getConnector(),
				},
			},
		}

		if err := updateCluster(cluster); err != nil {
			log.Error(err, "failed to export cluster to host cluster")
		}
	}

	return Periodic(interval, fn)
}

func ExportEndpoints(interval time.Duration, getConnector types.EndpointGetter, updateEndpoints UpdateEndpointsFunc) manager.Runnable {
	log := klogr.New().WithName("exportEndpoints")

	fn := func(ctx context.Context) {
		err := updateEndpoints([]apis.Endpoint{
			getConnector(),
		})

		if err != nil {
			log.Error(err, "failed to export endpoints to host cluster")
		}
	}

	return Periodic(interval, fn)
}

func LoadEndpointsAndCommunities(interval time.Duration, store storepkg.Interface, getEndpointsAndCommunities GetEndpointsAndCommunitiesFunc) manager.Runnable {
	log := klogr.New().WithName("loadEndpointsAndCommunities")

	communitySet := sets.NewString()
	endpointSet := sets.NewString()

	fn := func(ctx context.Context) {
		ec, err := getEndpointsAndCommunities()
		if err != nil {
			log.Error(err, "failed to load endpoints and communities")
			return
		}

		currentCommunitySet, currentEndpointSet := sets.NewString(), sets.NewString()
		for name, members := range ec.Communities {
			currentCommunitySet.Insert(name)
			store.SaveCommunity(types.Community{
				Name:    name,
				Members: sets.NewString(members...),
			})
		}

		for _, endpoint := range ec.Endpoints {
			currentEndpointSet.Insert(endpoint.Name)
			store.SaveEndpoint(endpoint)
		}

		for name := range communitySet.Difference(currentCommunitySet) {
			store.DeleteCommunity(name)
		}

		for name := range endpointSet.Difference(currentEndpointSet) {
			store.DeleteEndpoint(name)
		}

		communitySet = currentCommunitySet
		endpointSet = currentEndpointSet
	}

	return Periodic(interval, fn)
}
