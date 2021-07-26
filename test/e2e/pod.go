// Copyright 2021 BoCloud
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

package e2e

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

	// 测试非主机网络pod与pod间通信
	It("let pods communicate with each other", func() {
		cloudPods, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		By("pods communicate with each other")
		// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
		pods := append(edgePods, cloudPods...)

		for _, p1 := range pods {
			for _, p2 := range pods {
				if p1.Name == p2.Name {
					continue
				}

				framework.Logf("ping from %s to %s", p1.Name, p2.Name)
				framework.ExpectNoError(ping(p1, p2.Status.PodIP), "pods should be able to communicate with each other")
			}
		}
	})

	// 测试边缘pod访问本地服务端点的情况
	It("let edge pods can access local service endpoint when it exists", func() {
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
			stdout, _, err := execCurl(pod, serviceName)
			framework.ExpectNoError(err)
			Expect(stdout).To(ContainSubstring(expectedPodName))
		}
	})

	// 测试边缘pod访问云端服务端点的情况
	It("let edge pods can access cloud services", func() {
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
})
