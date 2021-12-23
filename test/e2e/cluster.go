package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	"github.com/fabedge/fabedge/test/e2e/framework"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cluster struct {
	name                  string
	role                  string
	config                *rest.Config
	client                client.Client
	clientset             kubernetes.Interface
	serviceCloudNginx     string
	serviceEdgeNginx      string
	serviceHostCloudNginx string
	serviceHostEdgeNginx  string
	ready                 bool
}

func (c Cluster) isHost() bool {
	return c.role == "host"
}

func generateCluster(cfgPath string) (cluster Cluster, err error) {
	// path e.g. /tmp/e2ekubeconfig/10.20.8.20
	cfg, err := clientcmd.BuildConfigFromFlags("", cfgPath)
	if err != nil {
		return
	}

	cli, err := client.New(cfg, client.Options{})
	if err != nil {
		return
	}

	// get each cluster name and role with kubeconfig
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return
	}

	cluster = Cluster{
		config:    cfg,
		client:    cli,
		clientset: clientset,
	}
	err = cluster.generateNameAndRole()
	if err != nil {
		return
	}
	cluster.generateServiceNames()

	return cluster, nil
}

func (c *Cluster) generateNameAndRole() error {
	// ns fabedge, deploy fabedge-operator
	fabedgeOperator, err := c.clientset.AppsV1().Deployments("fabedge").Get(context.TODO(), "fabedge-operator", metav1.GetOptions{})
	if err != nil {
		return err
	}
	args := fabedgeOperator.Spec.Template.Spec.Containers[0].Args
	for _, v := range args {
		if strings.HasPrefix(v, "--cluster=") {
			c.name = v[len("--cluster="):]
		}
		if strings.HasPrefix(v, "--cluster-role=") {
			c.role = v[len("--cluster-role="):]
		}
	}
	if len(c.name) == 0 || len(c.role) == 0 {
		return fmt.Errorf("clusterName=%s or clusterRole=%s", c.name, c.role)
	}
	return nil
}

// 将所有边缘节点添加到同一个社区，确保所有节点上的pod可以通信
func (c Cluster) addAllEdgesToCommunity(name string) {
	framework.Logf("add all edge nodes to community %s", communityName)

	var nodes corev1.NodeList
	err := c.client.List(context.TODO(), &nodes)
	if err != nil {
		framework.Failf("failed to get edge nodes: %s", err)
	}

	var edgeNodes []corev1.Node
	for _, node := range nodes.Items {
		if nodeutil.IsEdgeNode(node) {
			edgeNodes = append(edgeNodes, node)
		}
	}

	if len(edgeNodes) == 0 {
		framework.Failf("no edge nodes are available")
	}

	community := apis.Community{
		ObjectMeta: metav1.ObjectMeta{
			Name: communityName,
		},
		Spec: apis.CommunitySpec{},
	}

	for _, node := range edgeNodes {
		community.Spec.Members = append(community.Spec.Members, framework.GetEndpointName(c.name, node.Name))
	}

	_ = c.client.Delete(context.TODO(), &community)
	if err = c.client.Create(context.TODO(), &community); err != nil {
		framework.Failf("failed to create community: %s", err)
	}

	framework.AddCleanupAction(func() {
		if err := c.client.Delete(context.TODO(), &community); err != nil {
			framework.Logf("failed to delete community %s. Err: %s", communityName, err)
		}
	})
}

func (c *Cluster) generateServiceNames() {
	c.serviceCloudNginx = getName(serviceCloudNginx)
	c.serviceEdgeNginx = getName(serviceEdgeNginx)
	c.serviceHostCloudNginx = getName(serviceHostCloudNginx)
	c.serviceHostEdgeNginx = getName(serviceHostEdgeNginx)
}

func (c Cluster) prepareNamespace(namespace string) {
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}

	_ = c.client.Delete(context.Background(), &ns)

	// 等待上次的测试资源清除
	err := framework.WaitForNamespacesDeleted(c.client, []string{namespace}, 5*time.Minute)
	if err != nil {
		framework.Failf("cluster %s namespace %q is not deleted. err: %v", c.name, namespace, err)
	}

	framework.Logf("cluster %s create new test namespace: %s", c.name, namespace)
	createObject(c.client, &ns)
}

// 在每个节点创建一个nginx pod和一个net-tool pod，
// nginx pod 根据节点类型决定pod是cloudNginx还是edgeNginx的后端
// net-tool pod用来相互ping和访问service
func (c Cluster) preparePodsOnEachNode(namespace string) {
	var nodes corev1.NodeList
	framework.ExpectNoError(c.client.List(context.TODO(), &nodes))

	for _, node := range nodes.Items {
		serviceName := c.serviceCloudNginx
		if nodeutil.IsEdgeNode(node) {
			if framework.TestContext.IsMultiClusterTest() {
				// multi cluster e2e-test not create pod on edge nodes
				continue
			}
			serviceName = c.serviceEdgeNginx
		}

		framework.Logf("create nginx pod on node %s.%s", c.name, node.Name)
		pod := newNginxPod(node, namespace, serviceName)
		createObject(c.client, &pod)

		framework.Logf("create net-tool pod on node %s.%s", c.name, node.Name)
		pod = newNetToolPod(node, namespace)
		createObject(c.client, &pod)
	}
}

func (c Cluster) prepareHostNetworkPodsOnEachNode(namespace string) {
	var nodes corev1.NodeList
	framework.ExpectNoError(c.client.List(context.TODO(), &nodes))

	for _, node := range nodes.Items {
		serviceName := c.serviceHostCloudNginx
		if nodeutil.IsEdgeNode(node) {
			serviceName = c.serviceHostEdgeNginx
		}

		framework.Logf("create hostNetwork nginx pod on node %s.%s", c.name, node.Name)
		pod := newHostNginxPod(node, serviceName, namespace)
		createObject(c.client, &pod)

		framework.Logf("create hostNetwork net-tool pod on node %s.%s", c.name, node.Name)
		pod = newHostNetToolPod(node, namespace)
		createObject(c.client, &pod)
	}
}

func (c Cluster) prepareService(name, namespace string) {
	framework.Logf("create service %s/%s on %s", namespace, name, c.name)
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

	createObject(c.client, &svc)
}

func (c *Cluster) waitForClusterPodsReady(wg *sync.WaitGroup, namespace string) {
	defer wg.Done()

	framework.Logf("Waiting for cluster %s all pods to be ready", c.name)
	timeout := time.Duration(framework.TestContext.WaitTimeout) * time.Second
	err := wait.PollImmediate(2*time.Second, timeout, func() (bool, error) {
		var pods corev1.PodList
		err := c.client.List(context.TODO(), &pods, client.InNamespace(namespace), client.MatchingLabels{
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
		framework.Logf("net-tool pods in cluster %s are not ready after %d seconds. Error: %v", c.name, framework.TestContext.WaitTimeout, err)
	} else {
		c.ready = true
	}
}

func (c Cluster) getServiceIP(namespace, servicename string) (string, error) {
	svc, err := c.clientset.CoreV1().Services(namespace).Get(context.TODO(), servicename, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return svc.Spec.ClusterIP, nil
}

func (c Cluster) ping(pod corev1.Pod, ip string) error {
	timeout := fmt.Sprint(framework.TestContext.PingTimeout)
	_, _, err := c.execute(pod, []string{"ping", "-w", timeout, "-c", "1", ip})
	return err
}

func (c Cluster) execCurl(pod corev1.Pod, url string) (string, string, error) {
	timeout := fmt.Sprint(framework.TestContext.CurlTimeout)
	return c.execute(pod, []string{"curl", "-sS", "-m", timeout, url})
}

func (c Cluster) execute(pod corev1.Pod, cmd []string) (string, string, error) {
	req := c.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: pod.Spec.Containers[0].Name,
		Command:   cmd,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(c.config, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if err != nil && framework.TestContext.ShowExecError {
		framework.Logf("failed to execute cmd: %s. stderr: %s. err: %s", strings.Join(cmd, " "), stderr.String(), err)
	}

	return stdout.String(), stderr.String(), err
}
