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
	"sync"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/test/e2e/framework"
)

const (
	testNamespace = "fabedge-e2e-test"
	appNetTool    = "fabedge-net-tool"
	communityName = "all-edge-nodes"

	multiClusterCommunityName = "multi-cluster-all-nodes"
	multiClusterNamespace     = "multi-cluster-fabedge-e2e-test"
	clusterKeySingle          = "localhost"

	instanceNetTool     = "net-tool"
	instanceHostNetTool = "host-net-tool"

	labelKeyApp      = "app"
	labelKeyInstance = "instance"

	// add a random label, prevent kubeedge to cache it
	labelKeyRand = "random"

	serviceCloudNginx     = "cloud-nginx"
	serviceEdgeNginx      = "edge-nginx"
	serviceHostCloudNginx = "host-cloud-nginx"
	serviceHostEdgeNginx  = "host-edge-nginx"
)

var (
	// 标记是否有失败的spec
	hasFailedSpec = false

	// cluster ip list for testing
	clusterIPs = []string{}
	// key: cluster IP val: cluster Detail
	clusterByIP = make(map[string]*Cluster)
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
	err = cluster.generateNameAndRole()
	if err != nil {
		framework.Failf("Error generating cluster name or role: %v", err)
	}
	cluster.generateServiceNames()

	clusterIPs = append(clusterIPs, clusterKeySingle)
	clusterByIP[clusterKeySingle] = &cluster // global

	cluster.addAllEdgesToCommunity(communityName)
	prepareClustersNamespace(testNamespace)
	preparePodsOnEachClusterNode(testNamespace)
	cluster.prepareHostNetworkPodsOnEachNode(testNamespace)
	prepareServicesOnEachCluster(testNamespace)

	WaitForAllClusterPodsReady(testNamespace)
}

func multiClusterE2eTestPrepare() {
	framework.Logf("multi cluster e2e test")
	// read dir get all cluster IPs
	configDir := framework.TestContext.MultiClusterConfigDir
	filelist, err := ioutil.ReadDir(configDir)
	if err != nil {
		framework.Failf("Error reading kubeconfig dir: %v", err)
	}
	for _, f := range filelist {
		ipStr := f.Name()
		clusterIPs = append(clusterIPs, ipStr)
	}
	if len(clusterIPs) <= 1 {
		framework.Failf("Error no multi cluster condition, cluster IP list: %v", clusterIPs)
	}
	framework.Logf("kubeconfigDir=%v get cluster IP list: %v", configDir, clusterIPs)

	clusterNameList := []string{}
	hasHostCluster := false
	for i := 0; i < len(clusterIPs); {
		clusterIP := clusterIPs[i]
		cluster, err := generateCluster(configDir, clusterIP)
		if err != nil {
			framework.Logf("Error generating cluster <%s> err: %v", clusterIP, err)
			clusterIPs = append(clusterIPs[:i], clusterIPs[i+1:]...)
			continue
		}

		if cluster.isHost() {
			hasHostCluster = true
		}
		clusterNameList = append(clusterNameList, cluster.name+":"+clusterIP)
		clusterByIP[clusterIP] = &cluster
		i++
	}

	if len(clusterIPs) <= 1 {
		framework.Failf("Error no multi cluster condition, cluster list: %v", clusterNameList)
	}
	if !hasHostCluster {
		framework.Failf("Error can not find host cluster role")
	}
	framework.Logf("cluster list: %v", clusterNameList)

	addAllMultiClusterNodesToCommunity()
	prepareClustersNamespace(multiClusterNamespace)
	preparePodsOnEachClusterNode(multiClusterNamespace)
	prepareServicesOnEachCluster(multiClusterNamespace)

	WaitForAllClusterPodsReady(multiClusterNamespace)
}

// 将各集群connector添加到同一个社区，确保所有集群非边缘节点上的pod可以互相通信
func addAllMultiClusterNodesToCommunity() {
	framework.Logf("add all multi cluster nodes to community %s", multiClusterCommunityName)

	community := apis.Community{
		ObjectMeta: metav1.ObjectMeta{
			Name: multiClusterCommunityName,
		},
		Spec: apis.CommunitySpec{},
	}
	var hostCluster *Cluster
	for _, clusterIP := range clusterIPs {
		cluster := clusterByIP[clusterIP]
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
	for _, clusterIP := range clusterIPs {
		if cluster, ok := clusterByIP[clusterIP]; ok {
			cluster.prepareNamespace(namespace)
		}
	}
}

// 在每个节点创建一个nginx pod和一个net-tool pod，
// nginx pod 根据节点类型决定pod是cloudNginx还是edgeNginx的后端
// net-tool pod用来相互ping和访问service
func preparePodsOnEachClusterNode(namespace string) {
	framework.Logf("Prepare pods on each node")
	for _, clusterIP := range clusterIPs {
		if cluster, ok := clusterByIP[clusterIP]; ok {
			cluster.preparePodsOnEachNode(namespace)
		}
	}
}

func prepareServicesOnEachCluster(namespace string) {
	for _, clusterIP := range clusterIPs {
		if cluster, ok := clusterByIP[clusterIP]; ok {
			cluster.prepareService(cluster.serviceCloudNginx, namespace)
			if !framework.TestContext.IsMultiClusterTest() {
				cluster.prepareService(cluster.serviceEdgeNginx, namespace)
				cluster.prepareService(cluster.serviceHostCloudNginx, namespace)
				cluster.prepareService(cluster.serviceHostEdgeNginx, namespace)
			}
		}
	}
}

func WaitForAllClusterPodsReady(namespace string) {
	var wg sync.WaitGroup
	for _, cluster := range clusterByIP {
		wg.Add(1)
		go cluster.waitForClusterPodsReady(&wg, namespace)
	}
	wg.Wait()

	for _, cluster := range clusterByIP {
		if !cluster.ready {
			framework.Failf("clusters exist not ready pods")
		}
	}
}

func newNginxPod(node corev1.Node, namespace, serviceName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nginx-%s", node.Name),
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: serviceName,
				labelKeyRand:     fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: podSpec(node.Name),
	}
}

func newHostNginxPod(node corev1.Node, serviceName, namespace string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("host-nginx-%s", node.Name),
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: serviceName,
				labelKeyRand:     fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: hostNetworkPodSpec(node.Name),
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
		Spec: podSpec(node.Name),
	}
}

func newHostNetToolPod(node corev1.Node, namespace string) corev1.Pod {
	// change default port to avoid ports conflict with host service's endpoints
	spec := hostNetworkPodSpec(node.Name)
	spec.Containers[0].Env = []corev1.EnvVar{
		{
			Name:  "HTTP_PORT",
			Value: "18080",
		},
		{
			Name:  "HTTPS_PORT",
			Value: "18083",
		},
	}

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
		Spec: spec,
	}
}

func hostNetworkPodSpec(nodeName string) corev1.PodSpec {
	spec := podSpec(nodeName)
	spec.HostNetwork = true
	spec.Containers[0].Ports = nil

	return spec
}

func podSpec(nodeName string) corev1.PodSpec {
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
						ContainerPort: 80,
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
	framework.ExpectNoError(cli.Create(context.TODO(), object))
	framework.AddCleanupAction(func() {
		if err := cli.Delete(context.TODO(), object); err != nil {
			klog.Errorf("Failed to delete object %s, please delete it manually. Err: %s", object.GetName(), err)
		}
	})
}
