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

package community

import (
	"github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var cfg *rest.Config
var k8sClient client.Client

// envtest provide a api server which has some differences from real environments,
// read https://book.kubebuilder.io/reference/envtest.html#testing-considerations
var testEnv *envtest.Environment

func TestCommunity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Community Suite")
}

var _ = BeforeSuite(func(done Done) {
	testutil.SetupLogger()

	By("starting test environment")
	var err error
	testEnv, cfg, k8sClient, err = testutil.StartTestEnvWithCRD(
		[]string{filepath.Join("..", "..", "..", "..", "deploy", "crds")},
	)
	Expect(err).ToNot(HaveOccurred())

	_ = v1alpha1.AddToScheme(scheme.Scheme)

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ShouldNot(HaveOccurred())
})
