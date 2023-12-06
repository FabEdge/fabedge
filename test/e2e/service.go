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
	"sigs.k8s.io/controller-runtime/pkg/client"

	netutil "github.com/fabedge/fabedge/pkg/util/net"
	"github.com/fabedge/fabedge/test/e2e/framework"
)

var _ = Describe("FabEdge", func() {
	// testcases for multi-clusters communication
	if framework.TestContext.IsMultiClusterTest() {
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
		// testcases for single cluster internal communication
		var cluster Cluster
		BeforeEach(func() {
			cluster = clusters[0]
		})

		It("let edge pods can access edge services [p2p][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			for _, serviceName := range cluster.edgeNginxServiceNames() {
				servicePods, _, err := framework.ListCloudAndEdgePods(cluster.client,
					client.InNamespace(namespaceSingle),
					client.MatchingLabels{labelKeyLocation: LocationEdge, labelKeyUseHostNetwork: "false"},
				)
				framework.ExpectNoError(err)

				for _, pod := range edgePods {
					framework.Logf("pod %s visit service %s", pod.Name, serviceName)
					cluster.checkServiceAvailability(pod, serviceName, servicePods)
				}
			}
		})

		It("let edge pods can access headless edge services [p2p][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			for _, serviceName := range cluster.edgeEdgeMySQLServiceNames() {
				replicas := len(edgePods)
				for _, pod := range edgePods {
					framework.Logf("pod %s visit service %s", pod.Name, serviceName)

					for i := 0; i < replicas; i++ {
						endpointName := fmt.Sprintf("%s-%d.%s:%d", serviceName, i, serviceName, defaultHttpPort)
						framework.Logf("pod %s visit endpoint %s", pod.Name, endpointName)
						_, _, e := cluster.execCurl(pod, endpointName)
						framework.ExpectNoError(e)
					}
				}
			}
		})

		It("let edge pods can access edge host service [p2n][e2e]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			servicePods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyLocation: LocationEdge, labelKeyUseHostNetwork: "true"},
			)
			framework.ExpectNoError(err)

			// for now, only test IPv4 service
			serviceName := cluster.serviceHostEdgeNginx
			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)
				cluster.checkServiceAvailability(pod, serviceName, servicePods)
			}
		})

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

		It("let edge pods can access cloud services with host network endpoints [p2n][e2c][host-service]", func() {
			_, edgePods, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			serviceName := cluster.serviceHostCloudNginx
			servicePods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyLocation: LocationCloud, labelKeyUseHostNetwork: "true"},
			)

			for _, pod := range edgePods {
				framework.Logf("pod %s visit service %s", pod.Name, serviceName)

				cluster.checkServiceAvailability(pod, serviceName, servicePods)
			}
		})

		It("let cloud pods can access edge services [p2n][c22]", func() {
			cloudPods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceNetTool},
			)
			framework.ExpectNoError(err)

			hostCloudPods, _, err := framework.ListCloudAndEdgePods(cluster.client,
				client.InNamespace(namespaceSingle),
				client.MatchingLabels{labelKeyInstance: instanceHostNetTool},
			)
			framework.ExpectNoError(err)
			cloudPods = append(cloudPods, hostCloudPods...)

			for _, serviceName := range cluster.edgeNginxServiceNames() {
				servicePods, _, err := framework.ListCloudAndEdgePods(cluster.client,
					client.InNamespace(namespaceSingle),
					client.MatchingLabels{labelKeyLocation: LocationEdge, labelKeyUseHostNetwork: "false"},
				)
				framework.ExpectNoError(err)

				for _, pod := range cloudPods {
					framework.Logf("pod %s visit service %s", pod.Name, serviceName)
					cluster.checkServiceAvailability(pod, serviceName, servicePods)
				}
			}
		})
	}
})
