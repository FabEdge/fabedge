package routines

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

func NewIPPoolKeeper(interval time.Duration, localClusterName string, cli client.Client, getClusterCIDRInfo types.GetClusterCIDRInfo) manager.Runnable {
	return Periodic(interval, newIPPoolKeeperFunc(localClusterName, cli, getClusterCIDRInfo))
}

func newIPPoolKeeperFunc(localClusterName string, cli client.Client, getClusterCIDRInfo types.GetClusterCIDRInfo) func(ctx context.Context) {
	log := klogr.New().WithName("ippool-keeper")

	oldClusterSet := sets.NewString()
	return func(ctx context.Context) {
		cidrsByCluster, err := getClusterCIDRInfo()
		if err != nil {
			log.Error(err, "failed to export endpoints to host cluster")
			return
		}

		newClusterSet := sets.NewString()
		for name, cidrs := range cidrsByCluster {
			if name == localClusterName {
				continue
			}
			newClusterSet.Insert(name)

			keepIPPoolForCluster(ctx, name, cidrs, cli, log)
		}

		noError := true
		for clusterName := range oldClusterSet.Difference(newClusterSet) {
			var pools calicoapi.IPPoolList
			if err = cli.List(ctx, &pools, client.MatchingLabels{constants.KeyCluster: clusterName}); err != nil {
				log.Error(err, "failed to get ippool list", "cluster", clusterName)
				noError = false
			}

			for _, pool := range pools.Items {
				if err = cli.Delete(ctx, &pool); err != nil {
					log.Error(err, "failed to delete ippool")
					noError = false
				}
			}
		}

		if noError {
			oldClusterSet = newClusterSet
		}
	}
}

func keepIPPoolForCluster(ctx context.Context, clusterName string, cidrs []string, cli client.Client, log logr.Logger) {
	var pools calicoapi.IPPoolList
	if err := cli.List(ctx, &pools, client.MatchingLabels{constants.KeyCluster: clusterName}); err != nil {
		log.Error(err, "failed to get ippool list", "cluster", clusterName)
		return
	}

	newCIDRSet, oldCIDRSet := sets.NewString(cidrs...), sets.NewString()
	for _, pool := range pools.Items {
		oldCIDRSet.Insert(pool.Spec.CIDR)
	}

	if oldCIDRSet.Equal(newCIDRSet) {
		return
	}

	for cidr := range newCIDRSet.Difference(oldCIDRSet) {
		pool := NewIPPool(clusterName, cidr)
		if err := cli.Create(ctx, &pool); err != nil {
			log.Error(err, "failed to create ippool", "cidr", cidr, "cluster", clusterName)
		}
	}

	for cidr := range oldCIDRSet.Difference(newCIDRSet) {
		poolName := normalizeCIDRToKubeName(clusterName, cidr)
		pool := calicoapi.IPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: poolName,
			},
		}
		if err := cli.Delete(ctx, &pool); err != nil {
			log.Error(err, "failed to delete ippool", "cidr", cidr, "cluster", clusterName)
		}
	}
}

func NewIPPool(clusterName, cidr string) calicoapi.IPPool {
	return calicoapi.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name: normalizeCIDRToKubeName(clusterName, cidr),
			Labels: map[string]string{
				constants.KeyCluster:   clusterName,
				constants.KeyCreatedBy: constants.AppOperator,
			},
		},
		Spec: calicoapi.IPPoolSpec{
			CIDR:     cidr,
			Disabled: true,
			// for IPv6, it should be 122, but since this ippool id disabled, the value doesn't matter
			BlockSize: 26,
			IPIPMode:  calicoapi.IPIPModeNever,
			VXLANMode: calicoapi.VXLANModeNever,
		},
	}
}

func normalizeCIDRToKubeName(cluster, cidr string) string {
	cidr = strings.ReplaceAll(cidr, ".", "-")
	cidr = strings.ReplaceAll(cidr, ":", "-")
	cidr = strings.ReplaceAll(cidr, "/", "-")
	return fmt.Sprintf("%s-%s", cluster, cidr)
}
