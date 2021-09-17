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

package e2e

import (
	. "github.com/onsi/ginkgo"
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

	// 测试非主机网络pod与pod边边通信
	It("let edge pods communicate with each other [p2p][e2e]", func() {
		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		By("pods communicate with each other")
		for _, p1 := range edgePods {
			for _, p2 := range edgePods {
				if p1.Name == p2.Name {
					continue
				}

				framework.Logf("ping between %s and %s", p1.Name, p2.Name)
				framework.ExpectNoError(pingBetween(p1, p2), "pods should be able to communicate with each other")
			}
		}
	})

	// 测试非主机网络pod与pod云边通信
	It("let edge pods communicate with cloud pods [p2p][e2c]", func() {
		cloudPods, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		By("pods communicate with each other")
		// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
		for _, p1 := range edgePods {
			for _, p2 := range cloudPods {
				if p1.Name == p2.Name {
					continue
				}

				framework.Logf("ping between %s and %s", p1.Name, p2.Name)
				framework.ExpectNoError(pingBetween(p1, p2), "pods should be able to communicate with each other")
			}
		}
	})

	// 测试非主机网络Pods与主机网络Pod的边边通信
	It("let edge pods communicate with edge pods using host network [p2n][e2e]", func() {
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

		By("pods communicate with each other")
		// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
		for _, p1 := range edgePods {
			for _, p2 := range hostEdgePods {
				framework.Logf("ping between %s and %s", p1.Name, p2.Name)
				framework.ExpectNoError(pingBetween(p1, p2), "pods should be able to communicate with each other")
			}
		}
	})

	// 测试非主机网络Pods与主机网络Pod的互通
	It("let edge pods communicate with cloud pods using host network[p2n][e2c]", func() {
		_, edgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		hostCloudPods, _, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
		)
		framework.ExpectNoError(err)

		By("pods communicate with each other")
		// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
		for _, p1 := range edgePods {
			for _, p2 := range hostCloudPods {
				framework.Logf("ping between %s and %s", p1.Name, p2.Name)
				framework.ExpectNoError(pingBetween(p1, p2), "pods should be able to communicate with each other")
			}
		}
	})

	// 测试主机网络Pods与非主机网络Pod的互通
	It("let edge pods using host network communicate with cloud pods[n2p][e2c]", func() {
		_, hostEdgePods, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
		)
		framework.ExpectNoError(err)

		cloudPods, _, err := framework.ListCloudAndEdgePods(k8sclient,
			client.InNamespace(testNamespace),
			client.MatchingLabels{labelKeyInstance: instanceNetTool},
		)
		framework.ExpectNoError(err)

		By("pods communicate with each other")
		// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
		for _, p1 := range hostEdgePods {
			for _, p2 := range cloudPods {
				framework.Logf("ping between %s and %s", p1.Name, p2.Name)
				framework.ExpectNoError(pingBetween(p1, p2), "pods should be able to communicate with each other")
			}
		}
	})
})
