package routines

import (
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
	"github.com/fabedge/fabedge/third_party/calicoapi"
)

var cfg *rest.Config
var k8sClient client.Client

// envtest provide an api server which has some differences from real environments,
// read https://book.kubebuilder.io/reference/envtest.html#testing-considerations
var testEnv *envtest.Environment

func TestRoutines(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Routines Suite")
}

var _ = BeforeSuite(func(done Done) {
	testutil.SetupLogger()

	By("starting test environment")
	var err error
	testEnv, cfg, k8sClient, err = testutil.StartTestEnvWithCRD(
		[]string{
			filepath.Join("..", "..", "..", "deploy", "crds"),
			filepath.Join("..", "..", "..", "third_party", "calicoapi", "crd"),
		},
	)
	Expect(err).ToNot(HaveOccurred())

	Expect(apis.AddToScheme(scheme.Scheme)).Should(Succeed())
	Expect(calicoapi.AddToScheme(scheme.Scheme)).Should(Succeed())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ShouldNot(HaveOccurred())
})
