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
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	"github.com/fabedge/fabedge/test/e2e/framework"
)

type Location string

const (
	LocationEdge  = "edge"
	LocationCloud = "cloud"

	namespaceSingle = "fabedge-e2e-test"
	communityEdges  = "e2e-all-edges"

	communityConnectors = "e2e-all-connectors"
	namespaceMulti      = "fabedge-e2e-test-multi"

	appNetTool          = "fabedge-net-tool"
	instanceNetTool     = "net-tool"
	instanceHostNetTool = "host-net-tool"
	instanceMySQL       = "mysql"

	labelKeyApp            = "app"
	labelKeyInstance       = "instance"
	labelKeyService        = "service"
	labelKeyLocation       = "location"
	labelKeyUseHostNetwork = "use-host-network"
	// add a random label, prevent kubeedge to cache it
	labelKeyRand = "random"

	serviceCloudNginx     = "cloud-nginx"
	serviceCloudNginx6    = "cloud-nginx6"
	serviceEdgeNginx      = "edge-nginx"
	serviceEdgeNginx6     = "edge-nginx6"
	serviceEdgeMySQL      = "edge-mysql"
	serviceEdgeMySQL6     = "edge-mysql6"
	serviceHostCloudNginx = "host-cloud-nginx"
	serviceHostEdgeNginx  = "host-edge-nginx"

	defaultHttpPort  = 30080
	defaultHttpsPort = 30443
)

var (
	// 标记是否有失败的spec
	hasFailedSpec = false

	clusters = make([]Cluster, 0)
)

func init() {
	_ = apis.AddToScheme(scheme.Scheme)
	rand.Seed(int64(time.Now().UnixNano()))
}

// RunE2ETests checks configuration parameters (specified through flags) and then runs
// E2E tests using the Ginkgo runner.
func RunE2ETests(t *testing.T) {
	gomega.RegisterFailHandler(func(message string, callerSkip ...int) {
		hasFailedSpec = true
		ginkgo.Fail(message, callerSkip...)
	})

	if framework.TestContext.GenReport {
		reportFile := framework.TestContext.ReportFile
		framework.Logf("test report will be written to file %s", reportFile)
		ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "FabEdge Network Tests", []ginkgo.Reporter{
			framework.NewTableReporter(reportFile),
		})
	} else {
		ginkgo.RunSpecs(t, "FabEdge Network Tests")
	}
}

var _ = ginkgo.SynchronizedBeforeSuite(func() []byte {

	if framework.TestContext.IsMultiClusterTest() {
		multiClusterE2eTestPrepare()
	} else {
		singleClusterE2eTestPrepare()
	}

	return nil
}, func(_ []byte) {
})

var _ = ginkgo.SynchronizedAfterSuite(func() {
	framework.Logf("test suite finished")
	switch framework.PreserveResourcesMode(framework.TestContext.PreserveResources) {
	case framework.PreserveResourcesAlways:
		framework.Logf("resources are preserved, please prune them manually before next time")
	case framework.PreserveResourcesOnFailure:
		if hasFailedSpec {
			framework.Logf("resources are preserved as some specs failed, please prune them manually before next time")
			return
		}
		fallthrough
	case framework.PreserveResourcesNever:
		framework.Logf("pruning resources")
		framework.RunCleanupActions()
	}
}, func() {
})

func singleClusterE2eTestPrepare() {
	framework.Logf("single cluster e2e test")
	cfg, err := framework.LoadConfig()
	if err != nil {
		framework.Failf("Error loading config: %v", err)
	}
	client, err := framework.CreateClient()
	if err != nil {
		framework.Failf("Error creating client: %v", err)
	}
	clientset, err := framework.CreateClientSet()
	if err != nil {
		framework.Failf("Error creating clientset: %v", err)
	}

	cluster := Cluster{
		config:    cfg,
		client:    client,
		clientset: clientset,
	}

	err = cluster.getNameAndRole()
	if err != nil {
		framework.Failf("failed to get cluster name or role: %v", err)
	}

	cluster.makeupServiceNames()
	clusters = append(clusters, cluster)

	if framework.TestContext.CreateEdgeCommunity {
		cluster.addAllEdgesToCommunity()
	}
	prepareClustersNamespace(namespaceSingle)
	preparePodsOnEachClusterNode(namespaceSingle)
	prepareStatefulSets(namespaceSingle)
	cluster.prepareHostNetworkPodsOnEachNode(namespaceSingle)
	prepareServicesOnEachCluster(namespaceSingle)

	WaitForAllClusterPodsReady(namespaceSingle)
}

func multiClusterE2eTestPrepare() {
	framework.Logf("multi cluster e2e test")
	configDir := framework.TestContext.KubeConfigsDir
	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		framework.Failf("Error reading kubeconfig dir: %v", err)
	}

	if len(files) <= 1 {
		framework.Failf("only %d kubeconfig files are found, can not do multi-cluster e2e test", len(files))
	}

	hasHostCluster := false
	for _, f := range files {
		kubeconfigPath := path.Join(configDir, f.Name())
		cluster, err := generateCluster(kubeconfigPath)
		if err != nil {
			framework.Logf("failed to get cluster info with kubeconfig: %s. %s", kubeconfigPath, err)
			continue
		}

		clusters = append(clusters, cluster)
		hasHostCluster = hasHostCluster || cluster.isHost()
	}

	if !hasHostCluster {
		framework.Failf("no host cluster found, can not do e2e test")
	}

	addAllConnectorsToCommunity()
	prepareClustersNamespace(namespaceMulti)
	preparePodsOnEachClusterNode(namespaceMulti)
	prepareServicesOnEachCluster(namespaceMulti)

	WaitForAllClusterPodsReady(namespaceMulti)
}

// 将各集群connector添加到同一个社区，确保所有集群非边缘节点上的pod可以互相通信
func addAllConnectorsToCommunity() {
	framework.Logf("add all connectors to community %s", communityConnectors)

	community := apis.Community{
		ObjectMeta: metav1.ObjectMeta{
			Name: communityConnectors,
		},
		Spec: apis.CommunitySpec{},
	}

	var hostCluster Cluster

	for _, cluster := range clusters {
		community.Spec.Members = append(community.Spec.Members, framework.GetEndpointName(cluster.name, "connector"))
		if cluster.isHost() {
			hostCluster = cluster
		}
	}

	_ = hostCluster.client.Delete(context.TODO(), &community)
	if err := hostCluster.client.Create(context.TODO(), &community); err != nil {
		framework.Failf("host cluster %s failed to create community %s, Err: %v",
			hostCluster.name, community.Name, err)
	}

	framework.AddCleanupAction(func() {
		if err := hostCluster.client.Delete(context.TODO(), &community); err != nil {
			framework.Logf("host cluster %s failed to delete community %s, Err: %v",
				hostCluster.name, community.Name, err)
		}
	})
}

func prepareClustersNamespace(namespace string) {
	for _, cluster := range clusters {
		cluster.prepareNamespace(namespace)
	}
}

// 在每个节点创建一个nginx pod和一个net-tool pod，
// nginx pod 根据节点类型决定pod是cloudNginx还是edgeNginx的后端
// net-tool pod用来相互ping和访问service
func preparePodsOnEachClusterNode(namespace string) {
	framework.Logf("Prepare pods on each node")
	for _, cluster := range clusters {
		cluster.preparePodsOnEachNode(namespace)
	}
}

func prepareStatefulSets(namespace string) {
	for _, cluster := range clusters {
		cluster.prepareEdgeStatefulSet(serviceEdgeMySQL, namespace)
		if framework.TestContext.IPv6Enabled {
			cluster.prepareEdgeStatefulSet(serviceEdgeMySQL6, namespace)
		}
	}
}

func prepareServicesOnEachCluster(namespace string) {
	for _, cluster := range clusters {
		cluster.prepareService(cluster.serviceCloudNginx, namespace, corev1.IPv4Protocol, LocationCloud, false)
		if framework.TestContext.IPv6Enabled {
			cluster.prepareService(cluster.serviceCloudNginx6, namespace, corev1.IPv6Protocol, LocationCloud, false)
		}

		cluster.prepareHeadLessService(cluster.serviceEdgeMySQL, namespace, corev1.IPv4Protocol)
		if framework.TestContext.IPv6Enabled {
			cluster.prepareHeadLessService(cluster.serviceEdgeMySQL6, namespace, corev1.IPv6Protocol)
		}

		if !framework.TestContext.IsMultiClusterTest() {
			cluster.prepareService(cluster.serviceEdgeNginx, namespace, corev1.IPv4Protocol, LocationEdge, false)
			if framework.TestContext.IPv6Enabled {
				cluster.prepareService(cluster.serviceEdgeNginx6, namespace, corev1.IPv6Protocol, LocationEdge, false)
			}

			cluster.prepareService(cluster.serviceHostCloudNginx, namespace, corev1.IPv4Protocol, LocationCloud, true)
			cluster.prepareService(cluster.serviceHostEdgeNginx, namespace, corev1.IPv4Protocol, LocationEdge, true)
		}
	}
}

func WaitForAllClusterPodsReady(namespace string) {
	var wg sync.WaitGroup
	for i := range clusters {
		wg.Add(1)
		go clusters[i].waitForClusterPodsReady(&wg, namespace)
	}
	wg.Wait()

	for _, cluster := range clusters {
		if !cluster.ready {
			framework.Failf("clusters exist not ready pods")
		}
	}
}

func newNginxPod(node corev1.Node, namespace string) corev1.Pod {
	location := LocationCloud
	if nodeutil.IsEdgeNode(node) {
		location = LocationEdge
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nginx-%s", node.Name),
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:            appNetTool,
				labelKeyLocation:       location,
				labelKeyUseHostNetwork: "false",
				labelKeyRand:           fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: podSpec(node.Name, defaultHttpPort, defaultHttpsPort),
	}
}

func newHostNginxPod(node corev1.Node, namespace string) corev1.Pod {
	location := LocationCloud
	if nodeutil.IsEdgeNode(node) {
		location = LocationEdge
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("host-nginx-%s", node.Name),
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:            appNetTool,
				labelKeyLocation:       location,
				labelKeyUseHostNetwork: "true",
				labelKeyRand:           fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: hostNetworkPodSpec(node.Name, defaultHttpPort, defaultHttpsPort),
	}
}

func newNetToolPod(node corev1.Node, namespace string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("net-tool-%s", node.Name),
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: instanceNetTool,
				labelKeyRand:     fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: podSpec(node.Name, defaultHttpPort, defaultHttpsPort),
	}
}

func newHostNetToolPod(node corev1.Node, namespace string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("host-net-tool-%s", node.Name),
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: instanceHostNetTool,
				labelKeyRand:     fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		// change default port to avoid ports conflict with host service's endpoints
		Spec: hostNetworkPodSpec(node.Name, 10080, 10443),
	}
}

func hostNetworkPodSpec(nodeName string, httpPort, httpsPort int32) corev1.PodSpec {
	spec := podSpec(nodeName, httpPort, httpsPort)
	spec.HostNetwork = true
	return spec
}

func podSpec(nodeName string, httpPort, httpsPort int32) corev1.PodSpec {
	return corev1.PodSpec{
		HostNetwork: false,
		NodeName:    nodeName,
		DNSPolicy:   corev1.DNSClusterFirstWithHostNet,
		// workaround, or it will fail at edgecore
		AutomountServiceAccountToken: new(bool),
		Containers: []corev1.Container{
			{
				Name:            "net-tool",
				Image:           framework.TestContext.NetToolImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{
					{
						Name:          "http",
						ContainerPort: httpPort,
					},
					{
						Name:          "https",
						ContainerPort: httpsPort,
					},
				},
				Env: []corev1.EnvVar{
					{
						Name:  "HTTP_PORT",
						Value: fmt.Sprint(httpPort),
					},
					{
						Name:  "HTTPS_PORT",
						Value: fmt.Sprint(httpsPort),
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "",
				Operator: corev1.TolerationOpExists,
			},
		},
	}
}

func createObject(cli client.Client, object client.Object) {
	err := cli.Create(context.TODO(), object)
	if framework.TestContext.ReuseResource && errors.IsAlreadyExists(err) {
		err = nil
		framework.Logf("%s/%s exists", object.GetNamespace(), object.GetName())
	}
	framework.ExpectNoError(err)
	framework.AddCleanupAction(func() {
		if err := cli.Delete(context.TODO(), object); err != nil {
			klog.Errorf("Failed to delete object %s, please delete it manually. Err: %s", object.GetName(), err)
		}
	})
}
