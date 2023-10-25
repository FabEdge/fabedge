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
			for _, c1 := range clusters {
				cloudPodsI, _, err := framework.ListCloudAndEdgePods(c1.client,
					client.InNamespace(namespaceMulti),
					client.MatchingLabels{labelKeyInstance: instanceNetTool},
				)
				framework.ExpectNoError(err)

				for _, c2 := range clusters {
					if c1.name == c2.name {
						continue
					}

					cloudPodsJ, _, err := framework.ListCloudAndEdgePods(c2.client,
						client.InNamespace(namespaceMulti),
						client.MatchingLabels{labelKeyInstance: instanceNetTool},
					)
					framework.ExpectNoError(err)

					for _, p1 := range cloudPodsI {
						for _, p2 := range cloudPodsJ {
							framework.Logf("ping between %s/%s and %s/%s", c1.name, p1.Name, c2.name, p2.Name)
							for _, podIP := range p2.Status.PodIPs {
								framework.ExpectNoError(c1.ping(p1, podIP.IP))
							}

							for _, podIP := range p1.Status.PodIPs {
								framework.ExpectNoError(c2.ping(p2, podIP.IP))
							}
						}
					}
				}
			}
		})

	} else {
		// 单集群测试
		var cluster Cluster

		BeforeEach(func() {
			cluster = clusters[0]
		})

		// 测试非主机网络pod与pod边边通信
		It("let edge pods communicate with each other [p2p][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			for _, p1 := range edgePods {
				for _, p2 := range edgePods {
					cluster.expectPodsCanCommunicate(p1, p2, bothIPv4AndIPv6)
				}
			}
		})

		// 测试非主机网络pod与pod云边通信
		It("let edge pods communicate with cloud pods [p2p][e2c]", func() {
			cloudPods, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			for _, p1 := range cloudPods {
				for _, p2 := range edgePods {
					cluster.expectPodsCanCommunicate(p1, p2, bothIPv4AndIPv6)
				}
			}
		})

		// 测试非主机网络Pods与主机网络Pod的边边通信
		It("let edge pods communicate with edge pods using host network [p2n][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			_, hostEdgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			By("pods communicate with each other")
			for _, p1 := range edgePods {
				for _, p2 := range hostEdgePods {
					// for now, not all edge framework support dual stack, so only IPv4 is tested
					cluster.expectPodsCanCommunicate(p1, p2, onlyIPv4)
				}
			}
		})

		// 测试非主机网络Pods与主机网络Pod的云边互通
		It("let edge pods communicate with cloud pods using host network[p2n][e2c]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			hostCloudPods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			for _, p1 := range hostCloudPods {
				for _, p2 := range edgePods {
					// for now, not all edge framework support dual stack, so only IPv4 is tested
					cluster.expectPodsCanCommunicate(p1, p2, onlyIPv4)
				}
			}
		})

		// 测试主机网络边缘Pods与非主机网络Pod的互通
		// Communication betwwen cloud pods and edge nodes don't work well, so this spec will be pending
		PIt("let edge pods using host network communicate with cloud pods[n2p][e2c]", func() {
			_, hostEdgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			cloudPods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			By("pods communicate with each other")
			for _, p1 := range cloudPods {
				for _, p2 := range hostEdgePods {
					// for now, not all edge framework support dual stack, so only IPv4 is tested
					cluster.expectPodsCanCommunicate(p1, p2, onlyIPv4)
				}
			}
		})
	}
})
