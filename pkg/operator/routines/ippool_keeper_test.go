package routines

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

var _ = Describe("IPPoolKeeper", func() {
	var (
		localClusterName  = "beijing"
		localClusterCIDRs = []string{"192.168.0.0/16"}
		clusterShanghai   = "shanghai"
		cidrsShanghai     = []string{"10.40.0.0/16"}
		clusterSuZhou     = "suzhou"
		cidrsSuZhou       = []string{"172.10.0.0/16", "fd96:ee88:2::/48"}

		cidrsByCluster = map[string][]string{
			localClusterName: localClusterCIDRs,
			clusterShanghai:  cidrsShanghai,
			clusterSuZhou:    cidrsSuZhou,
		}
		keepCIDRs   func(ctx context.Context)
		getCIDRInfo = func() (map[string][]string, error) {
			return cidrsByCluster, nil
		}
	)

	BeforeEach(func() {
		keepCIDRs = newIPPoolKeeperFunc(localClusterName, k8sClient, getCIDRInfo)
	})

	AfterEach(func() {
		var pools calicoapi.IPPoolList
		Expect(k8sClient.List(context.Background(), &pools)).To(Succeed())

		for _, pool := range pools.Items {
			Expect(k8sClient.Delete(context.Background(), &pool)).To(Succeed())
		}
	})

	It("will synchronize calico ippool for external cluster CIDRs", func() {
		By("check local cluster")
		keepCIDRs(context.Background())

		// local cluster will have no ip pool
		pools, err := listIPPools(localClusterName)
		Expect(err).To(BeNil())
		Expect(len(pools.Items)).To(Equal(0))

		// external cluster will have ip pools
		expectPoolsFromClusterCIDRs(clusterShanghai, cidrsShanghai)
		expectPoolsFromClusterCIDRs(clusterSuZhou, cidrsSuZhou)

		By("change external cluster cidrs")
		cidrsShanghai = []string{"2.2.0.0/16"}
		cidrsByCluster[clusterShanghai] = cidrsShanghai
		delete(cidrsByCluster, clusterSuZhou)
		keepCIDRs(context.Background())

		expectPoolsFromClusterCIDRs(clusterShanghai, cidrsShanghai)
		expectPoolsFromClusterCIDRs(clusterSuZhou, nil)
	})
})

func listIPPools(name string) (pools calicoapi.IPPoolList, err error) {
	err = k8sClient.List(context.Background(), &pools, client.MatchingLabels{constants.KeyCluster: name})
	return pools, err
}

func expectPoolsFromClusterCIDRs(clusterName string, cidrs []string) {
	pools, err := listIPPools(clusterName)
	Expect(err).To(BeNil())
	Expect(len(pools.Items)).To(Equal(len(cidrs)))

	cidrSet := sets.NewString()
	for _, pool := range pools.Items {
		cidrSet.Insert(pool.Spec.CIDR)
		ExpectWithOffset(1, pool.Name).To(Equal(normalizeCIDRToKubeName(clusterName, pool.Spec.CIDR)))
		ExpectWithOffset(1, pool.Spec.Disabled).To(BeTrue())
		ExpectWithOffset(1, pool.Spec.BlockSize).To(Equal(26))
		ExpectWithOffset(1, string(pool.Spec.IPIPMode)).To(Equal(string(calicoapi.IPIPModeNever)))
		ExpectWithOffset(1, string(pool.Spec.VXLANMode)).To(Equal(string(calicoapi.VXLANModeNever)))
	}

	expectedCIDRSet := sets.NewString(cidrs...)
	ExpectWithOffset(1, cidrSet).To(Equal(expectedCIDRSet))
}
