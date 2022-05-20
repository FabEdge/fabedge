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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	netutil "github.com/fabedge/fabedge/pkg/util/net"
	"github.com/fabedge/fabedge/test/e2e/framework"
)

var _ = Describe("FabEdge", func() {
	// 集群间通信测试
	if framework.TestContext.IsMultiClusterTest() {
		// 测试集群内pod访问集群间云端服务端点的情况
		It("let cluster cloud pods can access another cluster cloud services [multi-cluster]", func() {
			for _, c1 := range clusters {
				clientPods, _, err := framework.ListCloudAndEdgePods(c1.client,
					client.InNamespace(namespaceMulti),
					client.MatchingLabels{labelKeyInstance: instanceNetTool},
				)
				framework.ExpectNoError(err)

				if len(clientPods) == 0 {
					continue
				}

				for _, c2 := range clusters {
					if c1.name == c2.name {
						continue
					}

					for _, serviceName := range c2.cloudNginxServiceNames() {
						svcIP, err := c2.getServiceIP(namespaceMulti, serviceName)
						framework.ExpectNoError(err)

						servicePods, _, err := framework.ListCloudAndEdgePods(c2.client,
							client.InNamespace(namespaceMulti),
							client.MatchingLabels{labelKeyLocation: LocationCloud, labelKeyUseHostNetwork: "false"},
						)
						framework.ExpectNoError(err)

						for _, pod := range clientPods {
							framework.Logf("%s/%s visit service %s/%s", c1.name, pod.Name, c2.name, serviceName)
							url := fmt.Sprintf("http://%s", svcIP)
							if netutil.IsIPv6String(svcIP) {
								url = fmt.Sprintf("http://[%s]", svcIP)
							}
							c1.checkServiceAvailability(pod, url, servicePods)
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

		// 测试边缘pod访问本地服务端点的情况
		It("let edge pods can access local service endpoint when it exists [p2p]", func() {
			_, clientPods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			_, servicePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyLocation: LocationEdge, labelKeyUseHostNetwork: "false"},
			)
			framework.ExpectNoError(err)

			hostToPod := make(map[string]string)
			for _, pod := range servicePods {
				hostToPod[pod.Spec.NodeName] = pod.Name
			}
			for _, serviceName := range cluster.edgeNginxServiceNames() {
				for _, pod := range clientPods {
					expectedPodName, ok := hostToPod[pod.Spec.NodeName]
					Expect(ok).To(BeTrue())

					framework.Logf("pod %s visit service %s", pod.Name, serviceName)
					cluster.expectCurlResultContains(pod, serviceName, expectedPodName)
				}
			}
		})

		// 测试边缘pod访问本地服务端点的情况
		It("let edge pods can access local host service endpoint when it exists [p2n]", func() {
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

			serviceName := cluster.serviceHostEdgeNginx
			edgePods = append(edgePods, hostEdgePods...)
			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)
				cluster.expectCurlResultContains(pod, serviceName, pod.Spec.NodeName)
			}
		})

		// 测试边缘pod访问云端服务端点的情况
		It("let edge pods can access cloud services [p2p][c2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			for _, serviceName := range cluster.cloudNginxServiceNames() {
				servicePods, _, err := framework.ListCloudAndEdgePods(cluster.client,
					client.InNamespace(namespaceSingle),
					client.MatchingLabels{labelKeyLocation: LocationCloud, labelKeyUseHostNetwork: "false"},
				)
				framework.ExpectNoError(err)

				for _, pod := range edgePods {
					framework.Logf("pod %s visit service %s", pod.Name, serviceName)
					cluster.checkServiceAvailability(pod, serviceName, servicePods)
				}
			}
		})

		// 测试边缘主机网络pod访问云端服务端点的情况
		It("let edge pods using hostNetwork can access cloud services [n2p][e2c]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			// for now, only test IPv4 service
			serviceName := cluster.serviceCloudNginx
			servicePods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyLocation: LocationCloud, labelKeyUseHostNetwork: "false"},
			)
			framework.ExpectNoError(err)

			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)
				cluster.checkServiceAvailability(pod, serviceName, servicePods)
			}
		})

		// 测试边缘pod访问云端服务端点的情况
		It("let edge pods can access cloud services with host network endpoints [p2n][e2c][host-service]", func() {
			// todo: use more flexible control flags
			Skip("feature not supported now, because higher linux kernel is needed")

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

			serviceName := cluster.serviceHostCloudNginx
			edgePods = append(edgePods, hostEdgePods...)
			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)

				_, stderr, err := cluster.execCurl(pod, serviceName)
				Expect(err).ShouldNot(HaveOccurred(), stderr)
			}
		})
	}
})
