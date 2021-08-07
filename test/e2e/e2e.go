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
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/operator/apis/community/v1alpha1"
	"github.com/fabedge/fabedge/test/e2e/framework"
)

const (
	testNamespace         = "fabedge-e2e-test"
	appNetTool            = "fabedge-net-tool"
	communityName         = "all-edge-nodes"
	instanceNetTool       = "net-tool"
	nodeStatusNotReady    = "node.kubernetes.io/not-ready"
	nodeStatusUnreachable = "node.kubernetes.io/unreachable"
	nodeRoleEdge          = "node-role.kubernetes.io/edge"

	labelKeyApp      = "app"
	labelKeyInstance = "instance"

	// add a random label, prevent kubeedge to cache it
	labelKeyRand = "random"
)

var (
	serviceCloudNginx = "cloud-nginx"
	serviceEdgeNginx  = "edge-nginx"

	// 标记是否有失败的spec
	hasFailedSpec = false
)

func init() {
	_ = apis.AddToScheme(scheme.Scheme)
	rand.Seed(time.Now().Unix())
	serviceCloudNginx = getName(serviceCloudNginx)
	serviceEdgeNginx = getName(serviceEdgeNginx)
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
	client, err := framework.CreateClient()
	if err != nil {
		framework.Failf("Error creating client: %v", err)
	}

	addAllEdgesToCommunity(client)

	prepareNamespace(client, testNamespace)
	preparePodOnEachNode(client)
	prepareService(client, serviceCloudNginx, testNamespace)
	prepareService(client, serviceEdgeNginx, testNamespace)

	WaitForAllPodsReady(client)

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

// 将所有边缘节点添加到同一个社区，确保所有节点上的pod可以通信
func addAllEdgesToCommunity(cli client.Client) {
	framework.Logf("add all edge nodes to community %s", communityName)

	var nodes corev1.NodeList
	err := cli.List(context.TODO(), &nodes, client.HasLabels{"node-role.kubernetes.io/edge"})
	if err != nil {
		framework.Failf("failed to get edge nodes: %s", err)
	}

	if len(nodes.Items) == 0 {
		framework.Failf("no edge nodes are available")
	}

	community := apis.Community{
		ObjectMeta: metav1.ObjectMeta{
			Name: communityName,
		},
		Spec: apis.CommunitySpec{},
	}

	for _, node := range nodes.Items {
		community.Spec.Members = append(community.Spec.Members, node.Name)
	}

	if err = cli.Create(context.TODO(), &community); err != nil {
		framework.Failf("failed to create community: %s", err)
	}

	framework.AddCleanupAction(func() {
		if err := cli.Delete(context.TODO(), &community); err != nil {
			framework.Logf(" failed to delete community %s. Err: %s", communityName, err)
		}
	})
}

func prepareNamespace(client client.Client, namespace string) {
	// 等待上次的测试资源清除
	err := framework.WaitForNamespacesDeleted(client, []string{namespace}, 30*time.Second)
	if err != nil {
		framework.Failf("namespace %q is not deleted. err: %s", namespace, err)
	}

	framework.Logf("create new test namespace: %s", namespace)
	createObject(client, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
}

// 在每个节点创建一个nginx pod和一个net-tool pod，
// nginx pod 根据节点类型决定pod是cloudNginx还是edgeNginx的后端
// net-tool pod用来相互ping和访问service
func preparePodOnEachNode(cli client.Client) {
	var nodes corev1.NodeList
	framework.ExpectNoError(cli.List(context.TODO(), &nodes))

	for _, node := range nodes.Items {
		serviceName := serviceCloudNginx
		if _, isEdgeNode := node.Labels[nodeRoleEdge]; isEdgeNode {
			serviceName = serviceEdgeNginx
		}

		framework.Logf("create nginx pod on node %s", node.Name)
		pod := newNginxPod(node, serviceName)
		createObject(cli, &pod)


		framework.Logf("create net-tool pod on node %s", node.Name)
		pod = newNetToolPod(node)
		createObject(cli, &pod)
	}
}

func newNginxPod(node corev1.Node, serviceName string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("nginx-%s", node.Name),
			Namespace: testNamespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: serviceName,
				labelKeyRand: fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: podSpec(node.Name),
	}
}

func newNetToolPod(node corev1.Node) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("net-tool-%s", node.Name),
			Namespace: testNamespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: instanceNetTool,
				labelKeyRand: fmt.Sprintf("%d", time.Now().Nanosecond()),
			},
		},
		Spec: podSpec(node.Name),
	}
}

func podSpec(nodeName string) corev1.PodSpec {
	return corev1.PodSpec{
		HostNetwork: false,
		NodeName:    nodeName,
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
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/",
							Port: intstr.FromString("http"),
						},
					},
					InitialDelaySeconds: 5,
					TimeoutSeconds:      5,
					PeriodSeconds:       5,
				},
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/",
							Port: intstr.FromString("http"),
						},
					},
					InitialDelaySeconds: 5,
					TimeoutSeconds:      5,
					PeriodSeconds:       5,
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      nodeStatusNotReady,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
			{
				Key:      nodeStatusUnreachable,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoExecute,
			},
			{
				Key:      nodeRoleEdge,
				Operator: corev1.TolerationOpExists,
				Effect:   corev1.TaintEffectNoSchedule,
			},
		},
	}
}

func prepareService(cli client.Client, name, namespace string) {
	framework.Logf("Create service %s/%s", namespace, name)
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				labelKeyInstance: name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "default",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	createObject(cli, &svc)
}

func WaitForAllPodsReady(cli client.Client) {
	framework.Logf("Waiting for all pods to be ready")

	timeout := time.Duration(framework.TestContext.WaitTimeout) * time.Second
	err := wait.PollImmediate(2*time.Second, timeout, func() (bool, error) {
		var pods corev1.PodList
		err := cli.List(context.TODO(), &pods, client.InNamespace(testNamespace), client.MatchingLabels{
			"app": appNetTool,
		})
		if err != nil {
			return false, err
		}

		if len(pods.Items) == 0 {
			return false, nil
		}

		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				return false, nil
			}
		}

		// wait the pods to be ready, not only to be running, especially on slow environment
		time.Sleep(15 * time.Second)

		return true, nil
	})

	if err != nil {
		framework.Failf("net-tool pods are not ready after %d seconds. Error: %s", framework.TestContext.WaitTimeout, err)
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
