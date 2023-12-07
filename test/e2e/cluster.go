package e2e

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	netutil "github.com/fabedge/fabedge/pkg/util/net"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	"github.com/fabedge/fabedge/test/e2e/framework"
)

type Cluster struct {
	name                  string
	role                  string
	config                *rest.Config
	client                client.Client
	clientset             kubernetes.Interface
	serviceCloudNginx     string
	serviceCloudNginx6    string
	serviceEdgeNginx      string
	serviceEdgeNginx6     string
	serviceHostCloudNginx string
	serviceHostEdgeNginx  string
	serviceEdgeMySQL      string
	serviceEdgeMySQL6     string
	ready                 bool
}

func (c Cluster) isHost() bool {
	return c.role == "host"
}

// generateCluster generates the cluster with its corresponding kubeconfig storage Dir and kubeconfig file named by actual IP address of master node.
//
// The API server maybe like "https://vip.edge.io:6443" in kubeconfig file, thus we need to change the domain of the API server to the actual IP address of master node
// for accessing the API server outside the cluster.
func generateCluster(kubeconfigPath string) (cluster Cluster, err error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
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
	err = cluster.getNameAndRole()
	if err != nil {
		return
	}
	cluster.makeupServiceNames()

	return cluster, nil
}

func (c *Cluster) getNameAndRole() error {
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
func (c Cluster) addAllEdgesToCommunity() {
	framework.Logf("add all edge nodes to community %s", communityEdges)

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
			Name: communityEdges,
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
			framework.Logf("failed to delete community %s. Err: %s", communityEdges, err)
		}
	})
}

// Because coredns has cache, so to avoid old services DNS information
// is cached, we give each service a random suffix per run
func (c *Cluster) makeupServiceNames() {
	c.serviceCloudNginx = serviceCloudNginx
	c.serviceCloudNginx6 = serviceCloudNginx6
	c.serviceEdgeNginx = serviceEdgeNginx
	c.serviceEdgeNginx6 = serviceEdgeNginx6
	c.serviceEdgeMySQL = serviceEdgeMySQL
	c.serviceEdgeMySQL6 = serviceEdgeMySQL6
	c.serviceHostCloudNginx = serviceHostCloudNginx
	c.serviceHostEdgeNginx = serviceHostEdgeNginx
}

func (c Cluster) cloudNginxServiceNames() []string {
	serviceNames := []string{c.serviceCloudNginx}
	if framework.TestContext.IPv6Enabled {
		serviceNames = append(serviceNames, c.serviceCloudNginx6)
	}

	return serviceNames
}

func (c Cluster) edgeNginxServiceNames() []string {
	serviceNames := []string{c.serviceEdgeNginx}
	if framework.TestContext.IPv6Enabled {
		serviceNames = append(serviceNames, c.serviceEdgeNginx6)
	}

	return serviceNames
}

func (c Cluster) edgeEdgeMySQLServiceNames() []string {
	serviceNames := []string{c.serviceEdgeMySQL}
	if framework.TestContext.IPv6Enabled {
		serviceNames = append(serviceNames, c.serviceEdgeMySQL6)
	}

	return serviceNames
}

func (c Cluster) prepareNamespace(namespace string) {
	if framework.TestContext.ReuseResource {
		var ns corev1.Namespace
		err := c.client.Get(context.Background(), client.ObjectKey{Name: namespace}, &ns)
		switch {
		case err == nil:
			return
		case errors.IsNotFound(err):
			// do nothing
		default:
			framework.Failf("Failed to check if namespace %q of cluster %s exists. Error: %v", c.name, namespace, err)
		}
	}

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
		if nodeutil.IsEdgeNode(node) && framework.TestContext.IsMultiClusterTest() {
			// multi cluster e2e-test don't create pod on edge nodes
			continue
		}

		framework.Logf("create nginx pod on node %s.%s", c.name, node.Name)
		pod := newNginxPod(node, namespace)
		createObject(c.client, &pod)

		framework.Logf("create net-tool pod on node %s.%s", c.name, node.Name)
		pod = newNetToolPod(node, namespace)
		createObject(c.client, &pod)
	}
}

func (c Cluster) prepareEdgeStatefulSet(serviceName, namespace string) {
	var replicas int32 = 0
	var nodes corev1.NodeList
	framework.ExpectNoError(c.client.List(context.TODO(), &nodes))
	for _, node := range nodes.Items {
		if nodeutil.IsEdgeNode(node) {
			replicas += 1
		}
	}

	mysql := appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespace,
			Labels: map[string]string{
				labelKeyApp:      appNetTool,
				labelKeyInstance: instanceMySQL,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					labelKeyApp:      appNetTool,
					labelKeyInstance: instanceMySQL,
					labelKeyService:  serviceName,
				},
			},
			ServiceName: serviceName,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						labelKeyApp:      appNetTool,
						labelKeyInstance: instanceMySQL,
						labelKeyService:  serviceName,
					},
				},
				Spec: corev1.PodSpec{
					NodeSelector: nodeutil.GetEdgeNodeLabels(),
					Containers: []corev1.Container{
						{
							Name:            "net-tool",
							Image:           framework.TestContext.NetToolImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: defaultHttpPort,
								},
								{
									Name:          "https",
									ContainerPort: defaultHttpsPort,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "HTTP_PORT",
									Value: fmt.Sprint(defaultHttpPort),
								},
								{
									Name:  "HTTPS_PORT",
									Value: fmt.Sprint(defaultHttpsPort),
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
				},
			},
		},
	}

	createObject(c.client, &mysql)
}

func (c Cluster) prepareHostNetworkPodsOnEachNode(namespace string) {
	var nodes corev1.NodeList
	framework.ExpectNoError(c.client.List(context.TODO(), &nodes))

	for _, node := range nodes.Items {
		framework.Logf("create hostNetwork nginx pod on node %s.%s", c.name, node.Name)
		pod := newHostNginxPod(node, namespace)
		createObject(c.client, &pod)

		framework.Logf("create hostNetwork net-tool pod on node %s.%s", c.name, node.Name)
		pod = newHostNetToolPod(node, namespace)
		createObject(c.client, &pod)
	}
}

func (c Cluster) prepareService(name, namespace string, ipFamily corev1.IPFamily, location Location, useHostNetwork bool) {
	framework.Logf("create service %s/%s on %s", namespace, name, c.name)
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:       corev1.ServiceTypeNodePort,
			IPFamilies: []corev1.IPFamily{ipFamily},
			Selector: map[string]string{
				labelKeyLocation:       string(location),
				labelKeyUseHostNetwork: fmt.Sprint(useHostNetwork),
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(defaultHttpPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromInt(defaultHttpPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	createObject(c.client, &svc)
}

func (c Cluster) prepareHeadLessService(name, namespace string, ipFamily corev1.IPFamily) {
	framework.Logf("create service %s/%s on %s", namespace, name, c.name)
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ServiceSpec{
			Type:       corev1.ServiceTypeClusterIP,
			ClusterIP:  "None",
			IPFamilies: []corev1.IPFamily{ipFamily},
			Selector: map[string]string{
				labelKeyInstance: instanceMySQL,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(defaultHttpPort),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       "https",
					Port:       443,
					TargetPort: intstr.FromInt(defaultHttpsPort),
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

func (c Cluster) expectPodsCanCommunicate(p1, p2 corev1.Pod, filter PodIPFilterFunc) {
	if p1.Name == p2.Name {
		return
	}

	if filter == nil {
		filter = bothIPv4AndIPv6
	}

	framework.Logf("ping between %s and %s", p1.Name, p2.Name)
	for _, podIP := range filter(p2.Status.PodIPs) {
		framework.ExpectNoErrorWithOffset(2, c.ping(p1, podIP))
	}

	for _, podIP := range filter(p1.Status.PodIPs) {
		framework.ExpectNoErrorWithOffset(2, c.ping(p2, podIP))
	}
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

func (c Cluster) checkServiceAvailability(pod corev1.Pod, url string, servicePods []corev1.Pod) {
	type AccessRecord struct {
		PodName    string
		IP         string
		Connection string
		Message    string
	}

	records := make(map[string]*AccessRecord)
	mtx := &sync.Mutex{}

	for _, pod := range servicePods {
		records[pod.Status.PodIP] = &AccessRecord{
			PodName:    pod.Name,
			IP:         pod.Status.PodIP,
			Connection: "unknown",
		}
	}

	wg := &sync.WaitGroup{}
	endpointCount := len(servicePods)

	wg.Add(endpointCount * 2)
	var err error

	for i := 0; i < endpointCount*2; i++ {
		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			stdout, stderr, e := c.execCurl(pod, url)

			ipStr, status := "", ""
			if e == nil {
				status = "success"
				ipStr = getIP(stdout)
			} else {
				status = "failure"
				err = e
				ipStr = getIP(stderr)
			}

			if ipStr != "" {
				mtx.Lock()
				defer mtx.Unlock()

				if record, ok := records[ipStr]; ok {
					record.Connection = status
					record.Message = stderr
				}
			}

		}(wg)
	}

	wg.Wait()

	if err == nil {
		return
	}

	for _, r := range records {
		framework.Logf("PodName: %s IP: %s Connection: %s Message: %s", r.PodName, r.IP, r.Connection, r.Message)
	}

	framework.ExpectNoError(err)
}

func (c Cluster) expectCurlResultContains(pod corev1.Pod, url string, substr string) {
	timeout := fmt.Sprint(framework.TestContext.CurlTimeout)
	err := wait.Poll(time.Second, time.Duration(framework.TestContext.CurlTimeout)*time.Second, func() (bool, error) {
		stdout, _, _ := c.execute(pod, []string{"curl", "-sS", "-m", timeout, url})
		return strings.Contains(stdout, substr), nil
	})

	framework.ExpectNoError(err, fmt.Sprintf("response of curl %s should contains %s", url, substr))
}

func getIP(str string) string {
	reg, _ := regexp.Compile(`\d+\.\d+\.\d+\.\d+`)
	return string(reg.Find([]byte(str)))
}

func getName(prefix string) string {
	time.Sleep(time.Millisecond)
	return fmt.Sprintf("%s-%d", prefix, rand.Int31n(1000))
}

type PodIPFilterFunc func(podIPs []corev1.PodIP) []string

func bothIPv4AndIPv6(podIPs []corev1.PodIP) []string {
	var ips []string
	for _, podIP := range podIPs {
		ips = append(ips, podIP.IP)
	}

	return ips
}

func onlyIPv4(podIPs []corev1.PodIP) []string {
	var ips []string
	for _, podIP := range podIPs {
		if netutil.IsIPv4String(podIP.IP) {
			ips = append(ips, podIP.IP)
		}
	}

	return ips
}
