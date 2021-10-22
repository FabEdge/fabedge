package ipamblockmonitor

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	testutil "github.com/fabedge/fabedge/pkg/util/test"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestIpamblockmonitor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ipamblockmonitor Suite")
}

var _ = BeforeSuite(func(done Done) {
	testutil.SetupLogger()

	By("starting test environment")
	var err error
	testEnv, cfg, k8sClient, err = testutil.StartTestEnvWithCRD(
		[]string{filepath.Join("..", "..", "..", "..", "third_party", "calicoapi", "crd")},
	)
	Expect(err).NotTo(HaveOccurred())

	_ = calicoapi.AddToScheme(scheme.Scheme)

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).ShouldNot(HaveOccurred())
})
