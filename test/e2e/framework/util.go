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

package framework

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
)

const (
	// How often to poll for conditions
	pollInterval = 2 * time.Second

	//  Default time to wait for operations to complete
	defaultTimeout = 30 * time.Minute
)

// loadConfig loads a REST Config as per the rules specified in GetConfig
func LoadConfig() (*rest.Config, error) {
	// If a flag is specified with the config location, use that
	if len(TestContext.KubeConfig) > 0 {
		return clientcmd.BuildConfigFromFlags("", TestContext.KubeConfig)
	}
	// If an env variable is specified with the config location, use that
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	}
	// If no explicit location, try the in-cluster config
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	// If no in-cluster config, try the default location in the user's home directory
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"", filepath.Join(usr.HomeDir, ".kube", "config")); err == nil {
			return c, nil
		}
	}

	return nil, fmt.Errorf("could not locate a kubeconfig")
}

func CreateClient() (client.Client, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	return client.New(config, client.Options{})
}

func CreateClientSet() (kubernetes.Interface, error) {
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func CreateKubeNamespace(name string, client client.Client) (*corev1.Namespace, error) {
	Logf("Create namespace: %s", name)
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"ns-name": name,
			},
		},
	}

	err := wait.PollImmediate(pollInterval, defaultTimeout, func() (bool, error) {
		if err := client.Create(context.TODO(), &ns); err != nil {
			Logf("Unexpected error while creating namespace: %v", err)
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, err
	}

	return &ns, nil
}

func WaitForNamespacesDeleted(client client.Client, namespaces []string, timeout time.Duration) error {
	Logf("Waiting for namespaces to vanish")
	nsMap := map[string]bool{}
	for _, ns := range namespaces {
		nsMap[ns] = true
	}
	//Now POLL until all namespaces have been eradicated.
	return wait.Poll(2*time.Second, timeout,
		func() (bool, error) {
			var nsList corev1.NamespaceList
			if err := client.List(context.TODO(), &nsList); err != nil {
				return false, err
			}
			for _, item := range nsList.Items {
				if _, ok := nsMap[item.Name]; ok {
					return false, nil
				}
			}
			return true, nil
		})
}

func GetEdgeNodeNames(cli client.Client) (sets.String, error) {
	var nodes corev1.NodeList
	err := cli.List(context.TODO(), &nodes)
	if err != nil {
		return nil, err
	}

	nameSet := sets.NewString()
	for _, node := range nodes.Items {
		if nodeutil.IsEdgeNode(node) {
			nameSet.Insert(node.Name)
		}
	}

	return nameSet, nil
}

// ListCloudAndEdgePods 根据条件返回Pod列表，但分为两类: 在云端节点的pod和在边缘节点的pod
func ListCloudAndEdgePods(cli client.Client, opts ...client.ListOption) (cloudPods []corev1.Pod, edgePods []corev1.Pod, err error) {
	var pods corev1.PodList
	err = cli.List(context.TODO(), &pods, opts...)
	if err != nil {
		return
	}

	edgeNames, err := GetEdgeNodeNames(cli)
	if err != nil {
		return
	}

	for _, pod := range pods.Items {
		if edgeNames.Has(pod.Spec.NodeName) {
			edgePods = append(edgePods, pod)
		} else {
			cloudPods = append(cloudPods, pod)
		}
	}

	return
}

func GetEndpointName(clusterName, nodeName string) string {
	return fmt.Sprintf("%s.%s", clusterName, nodeName)
}
