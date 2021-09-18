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

package connector

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var getNodeName = testutil.GenerateGetNameFunc("node")
var getEdgeName = testutil.GenerateGetNameFunc("edge-node")
var edgeLabels = map[string]string{
	"edge": "",
}

func TestConnector(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Endpoint Suite")
}

var _ = BeforeSuite(func(done Done) {
	testutil.SetupLogger()
	nodeutil.SetEdgeNodeLabels(edgeLabels)

	By("starting test environment")
	var err error
	testEnv, cfg, k8sClient, err = testutil.StartTestEnv()
	Expect(err).NotTo(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(testEnv.Stop()).ShouldNot(HaveOccurred())
})
