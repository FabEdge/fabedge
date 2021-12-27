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
	// 集群间通信测试
	if framework.TestContext.IsMultiClusterTest() {
		// 测试集群内pod访问集群间云端服务端点的情况
		It("let cluster cloud pods can access another cluster cloud services [multi-cluster]", func() {
			for i := 0; i < len(clusterIPs); i++ {
				c1 := clusterByIP[clusterIPs[i]]
				cloudPodsI, _, err := framework.ListCloudAndEdgePods(c1.client,
					client.InNamespace(multiClusterNamespace),
					client.MatchingLabels{labelKeyInstance: instanceNetTool},
				)
				framework.ExpectNoError(err)
				if len(cloudPodsI) == 0 {
					continue
				}

				for j := 0; j < len(clusterIPs); j++ {
					if i == j {
						continue
					}
					c2 := clusterByIP[clusterIPs[j]]
					serviceName := c2.serviceCloudNginx
					svcIP, err := c2.getServiceIP(multiClusterNamespace, serviceName)
					framework.ExpectNoError(err)
					for _, pod := range cloudPodsI {
						framework.Logf("pod %s of cluster %s visit service %s of cluster %s", pod.Name, c1.name, serviceName, c2.name)
						_, _, err := c1.execCurl(pod, svcIP)
						framework.ExpectNoError(err)
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

		// 测试边缘pod访问本地服务端点的情况
		It("let edge pods can access local service endpoint when it exists [p2p]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			_, servicePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: cluster.serviceEdgeNginx},
			)
			framework.ExpectNoError(err)

			hostToPod := make(map[string]string)
			for _, pod := range servicePods {
				hostToPod[pod.Spec.NodeName] = pod.Name
			}

			serviceName := cluster.serviceEdgeNginx
			for _, pod := range edgePods {
				expectedPodName, ok := hostToPod[pod.Spec.NodeName]
				Expect(ok).To(BeTrue())

				framework.Logf("pod %s visit service %s", pod.Name, serviceName)
				expectCurlResultContains(cluster, pod, serviceName, expectedPodName)
			}
		})

		// 测试边缘pod访问本地服务端点的情况
		It("let edge pods can access local host service endpoint when it exists [p2n]", func() {
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

			serviceName := cluster.serviceHostEdgeNginx
			edgePods = append(edgePods, hostEdgePods...)
			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)
				expectCurlResultContains(cluster, pod, serviceName, pod.Spec.NodeName)
			}
		})

		// 测试边缘pod访问云端服务端点的情况
		It("let edge pods can access cloud services [p2p][c2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			serviceName := cluster.serviceCloudNginx
			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)

				_, _, err := cluster.execCurl(pod, serviceName)
				framework.ExpectNoError(err)
			}
		})

		// 测试边缘主机网络pod访问云端服务端点的情况
		It("let edge pods using hostNetwork can access cloud services [n2p][e2c]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(testNamespace),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)

			serviceName := cluster.serviceCloudNginx
			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)

				_, stderr, err := cluster.execCurl(pod, serviceName)
				Expect(err).ShouldNot(HaveOccurred(), stderr)
			}
		})

		// 测试边缘pod访问云端服务端点的情况
		It("let edge pods can access cloud services with host network endpoints [p2n][e2c][host-service]", func() {
			// todo: use more flexible control flags
			Skip("feature not supported now, because higher linux kernel is needed")

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
