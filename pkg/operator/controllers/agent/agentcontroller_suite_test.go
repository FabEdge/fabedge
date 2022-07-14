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

package agent

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/fabedge/fabedge/pkg/common/constants"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	testutil "github.com/fabedge/fabedge/pkg/util/test"
)

var cfg *rest.Config
var k8sClient client.Client
var getNodeName = testutil.GenerateGetNameFunc("edge")

// envtest provide a api server which has some differences from real environments,
// read https://book.kubebuilder.io/reference/envtest.html#testing-considerations
var testEnv *envtest.Environment

func TestAgentController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AgentController Suite")
}

var _ = BeforeSuite(func(done Done) {
	testutil.SetupLogger()
	nodeutil.SetEdgeNodeLabels(map[string]string{
		"edge": "",
	})

	By("starting test environment")
	var err error
	testEnv, cfg, k8sClient, err = testutil.StartTestEnv()
	Expect(err).NotTo(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func newNodePodCIDRsInAnnotations(name, ips, subnets string) corev1.Node {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: nodeutil.GetEdgeNodeLabels(),
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  "2.2.2.2/26",
			PodCIDRs: []string{"2.2.2.2/26"},
		},
		Status: corev1.NodeStatus{
			Addresses: newInternalAddresses(ips),
		},
	}

	if subnets != "" {
		node.Annotations = map[string]string{
			constants.KeyPodSubnets: subnets,
		}
	}

	return node
}

func newNodeUsingRawPodCIDRs(name, ips, subnets string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: nodeutil.GetEdgeNodeLabels(),
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  subnets,
			PodCIDRs: strings.Split(subnets, ","),
		},
		Status: corev1.NodeStatus{
			Addresses: newInternalAddresses(ips),
		},
	}
}

func expectOwnerReference(obj client.Object, node corev1.Node) {
	ownerReferences := obj.GetOwnerReferences()
	t := true

	Expect(len(ownerReferences)).To(Equal(1))
	Expect(ownerReferences[0]).To(Equal(metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Node",
		Name:               node.Name,
		UID:                node.UID,
		Controller:         &t,
		BlockOwnerDeletion: &t,
	}))
}

func newInternalAddresses(ips string) []corev1.NodeAddress {
	var addresses []corev1.NodeAddress
	for _, ip := range strings.Split(ips, ",") {
		if ip == "" {
			continue
		}
		addresses = append(addresses, corev1.NodeAddress{
			Type:    corev1.NodeInternalIP,
			Address: ip,
		})
	}

	return addresses
}
