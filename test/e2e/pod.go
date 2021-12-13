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
	// 集群间通信测试
	if framework.TestContext.IsMultiClusterTest() {
		// 测试集群间云端pod与pod通信
		It("let cluster cloud pods communicate with another cluster cloud pods [multi-cluster]", func() {

			for i := 0; i < len(clusterIPs)-1; i++ {
				c1 := clusterByIP[clusterIPs[i]]
				cloudPodsI, _, err := framework.ListCloudAndEdgePods(c1.client,
					client.InNamespace(multiClusterNamespace),
					client.MatchingLabels{labelKeyInstance: instanceNetTool},
				)
				framework.ExpectNoError(err)

				for j := i + 1; j < len(clusterIPs); j++ {
					c2 := clusterByIP[clusterIPs[j]]
					cloudPodsJ, _, err := framework.ListCloudAndEdgePods(c2.client,
						client.InNamespace(multiClusterNamespace),
						client.MatchingLabels{labelKeyInstance: instanceNetTool},
					)
					framework.ExpectNoError(err)
					for _, p1 := range cloudPodsI {
						for _, p2 := range cloudPodsJ {
							framework.Logf("ping between %s of cluster %s and %s of cluster %s", p1.Name, c1.name, p2.Name, c2.name)
							framework.ExpectNoError(c1.ping(p1, p2.Status.PodIP))
							framework.ExpectNoError(c2.ping(p2, p1.Status.PodIP))
						}
					}
				}
			}
		})

	} else {
		// 单集群测试
		var cluster *Cluster
		BeforeEach(func() {
			cluster = clusterByIP[clusterKeySingle]
		})

		// 测试非主机网络pod与pod边边通信
		It("let edge pods communicate with each other [p2p][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
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
					framework.ExpectNoError(cluster.ping(p1, p2.Status.PodIP))
					framework.ExpectNoError(cluster.ping(p2, p1.Status.PodIP))
				}
			}
		})

		// 测试非主机网络pod与pod云边通信
		It("let edge pods communicate with cloud pods [p2p][e2c]", func() {
			cloudPods, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
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
					framework.ExpectNoError(cluster.ping(p1, p2.Status.PodIP))
					framework.ExpectNoError(cluster.ping(p2, p1.Status.PodIP))
				}
			}
		})

		// 测试非主机网络Pods与主机网络Pod的边边通信
		It("let edge pods communicate with edge pods using host network [p2n][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			_, hostEdgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			By("pods communicate with each other")
			// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
			for _, p1 := range edgePods {
				for _, p2 := range hostEdgePods {
					framework.Logf("ping between %s and %s", p1.Name, p2.Name)
					framework.ExpectNoError(cluster.ping(p1, p2.Status.PodIP))
					framework.ExpectNoError(cluster.ping(p2, p1.Status.PodIP))
				}
			}
		})

		// 测试非主机网络Pods与主机网络Pod的互通
		It("let edge pods communicate with cloud pods using host network[p2n][e2c]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			hostCloudPods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			By("pods communicate with each other")
			// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
			for _, p1 := range edgePods {
				for _, p2 := range hostCloudPods {
					framework.Logf("ping between %s and %s", p1.Name, p2.Name)
					framework.ExpectNoError(cluster.ping(p1, p2.Status.PodIP))
					framework.ExpectNoError(cluster.ping(p2, p1.Status.PodIP))
				}
			}
		})

		// 测试主机网络Pods与非主机网络Pod的互通
		It("let edge pods using host network communicate with cloud pods[n2p][e2c]", func() {
			_, hostEdgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			cloudPods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			By("pods communicate with each other")
			// 必须让edgePod在前面，因为云端pod不能主动打通边缘Pod的隧道
			for _, p1 := range hostEdgePods {
				for _, p2 := range cloudPods {
					framework.Logf("ping between %s and %s", p1.Name, p2.Name)
					framework.ExpectNoError(cluster.ping(p1, p2.Status.PodIP))
					framework.ExpectNoError(cluster.ping(p2, p1.Status.PodIP))
				}
			}
		})
	}
})
