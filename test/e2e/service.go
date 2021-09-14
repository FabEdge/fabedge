package e2e

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

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fabedge/fabedge/test/e2e/framework"
)

var _ = Describe("FabEdge", func() {
	var k8sclient client.Client
	var err error

	BeforeEach(func() {
		k8sclient, err = framework.CreateClient()
		framework.ExpectNoError(err)
	})

	// 测试边缘pod访问本地服务端点的情况
	It("let edge pods can access local service endpoint when it exists [p2p][local]", func() {
		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		_, servicePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: serviceEdgeNginx},
		)
		framework.ExpectNoError(err)

		hostToPod := make(map[string]string)
		for _, pod := range servicePods {
			hostToPod[pod.Spec.NodeName] = pod.Name
		}

		serviceName := serviceEdgeNginx
		for _, pod := range edgePods {
			expectedPodName, ok := hostToPod[pod.Spec.NodeName]
			Expect(ok).To(BeTrue())

			framework.Logf("pod %s visit service %s", pod.Name, serviceName)
			expectCurlResultContains(pod, serviceName, expectedPodName)
		}
	})

	// 测试边缘pod访问本地服务端点的情况
	It("let edge pods can access local host service endpoint when it exists [p2n][host-service]", func() {
		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		_, hostEdgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
		)
		framework.ExpectNoError(err)

		serviceName := serviceHostEdgeNginx
		edgePods = append(edgePods, hostEdgePods...)
		for _, pod := range edgePods {
			framework.Logf("pod %s visit service %s", pod.Name, serviceName)
			expectCurlResultContains(pod, serviceName, pod.Spec.NodeName)
		}
	})

	// 测试边缘pod访问云端服务端点的情况
	It("let edge pods can access cloud services [p2p][c2e]", func() {
		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		serviceName := serviceCloudNginx
		for _, pod := range edgePods {
			framework.Logf("pod %s visit service %s", pod.Name, serviceName)

			_, _, err := execCurl(pod, serviceName)
			framework.ExpectNoError(err)
		}
	})

	// 测试边缘主机网络pod访问云端服务端点的情况
	It("let edge pods using hostNetwork can access cloud services [n2p][c2e]", func() {
		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
		)
		framework.ExpectNoError(err)

		serviceName := serviceCloudNginx
		for _, pod := range edgePods {
			framework.Logf("pod %s visit service %s", pod.Name, serviceName)

			_, stderr, err := execCurl(pod, serviceName)
			Expect(err).ShouldNot(HaveOccurred(), stderr)
		}
	})

	// 测试边缘pod访问云端服务端点的情况
	It("let edge pods can access cloud services with host network endpoints [p2n][c2e][host-service]", func() {
		// todo: use more flexible control flags
		Skip("feature not supported now, because higher linux kernel is needed")

		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		_, hostEdgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
		)
		framework.ExpectNoError(err)

		serviceName := serviceHostCloudNginx
		edgePods = append(edgePods, hostEdgePods...)
		for _, pod := range edgePods {
			framework.Logf("pod %s visit service %s", pod.Name, serviceName)

			_, stderr, err := execCurl(pod, serviceName)
			Expect(err).ShouldNot(HaveOccurred(), stderr)
		}
	})
})
