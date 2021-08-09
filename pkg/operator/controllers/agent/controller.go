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

package agent

import (
	"context"
	"fmt"
	"hash/fnv"
	"net"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-logr/logr"
	"github.com/jjeffery/stringset"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	"github.com/fabedge/fabedge/pkg/operator/predicates"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

const (
	controllerName              = "agent-controller"
	agentConfigTunnelFileName   = "tunnels.yaml"
	agentConfigServicesFileName = "services.yaml"
	agentConfigTunnelsFilepath  = "/etc/fabedge/tunnels.yaml"
	agentConfigServicesFilepath = "/etc/fabedge/services.yaml"
)

type ObjectKey = client.ObjectKey

var _ reconcile.Reconciler = &agentController{}

type agentController struct {
	client      client.Client
	alloc       allocator.Interface
	store       storepkg.Interface
	newEndpoint types.NewEndpointFunc
	log         logr.Logger

	namespace       string
	agentImage      string
	strongswanImage string
	edgePodCIRD     string
	masqOutgoing    bool
}

type Config struct {
	Allocator allocator.Interface
	Store     storepkg.Interface
	Manager   manager.Manager

	Namespace       string
	AgentImage      string
	StrongswanImage string
	MasqOutgoing    bool
	EdgePodCIDR     string

	ConnectorConfig string
	NewEndpoint     types.NewEndpointFunc
}

func AddToManager(cnf Config) error {
	mgr := cnf.Manager

	reconciler := &agentController{
		namespace:       cnf.Namespace,
		agentImage:      cnf.AgentImage,
		strongswanImage: cnf.StrongswanImage,
		edgePodCIRD:     cnf.EdgePodCIDR,
		masqOutgoing:    cnf.MasqOutgoing,

		alloc:       cnf.Allocator,
		store:       cnf.Store,
		newEndpoint: cnf.NewEndpoint,

		log:    mgr.GetLogger().WithName(controllerName),
		client: mgr.GetClient(),
	}
	c, err := controller.New(
		controllerName,
		mgr,
		controller.Options{
			Reconciler: reconciler,
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: &corev1.Node{}},
		&handler.EnqueueRequestForObject{},
		predicates.EdgeNodePredicate(),
	)
}

func (ctl *agentController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	var node corev1.Node

	log := ctl.log.WithValues("key", request)
	if err := ctl.client.Get(ctx, request.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			log.Info("edge node is deleted, clear resources allocated to this node")
			return reconcile.Result{}, ctl.clearAllocatedResourcesForEdgeNode(ctx, request.Name)
		}

		log.Error(err, "unable to get edge node")
		return reconcile.Result{}, err
	}

	if node.DeletionTimestamp != nil {
		ctl.log.Info("edge node is terminating, clear resources allocated to this node")
		return reconcile.Result{}, ctl.clearAllocatedResourcesForEdgeNode(ctx, request.Name)
	}

	currentEndpoint := ctl.newEndpoint(node)
	if currentEndpoint.IP == "" {
		log.V(5).Info("This node has no ip, skip reconciling")
		return reconcile.Result{}, nil
	}

	if !ctl.isValidSubnets(currentEndpoint.Subnets) {
		if err := ctl.allocateSubnet(ctx, node); err != nil {
			return reconcile.Result{}, err
		}
	} else {
		ctl.store.SaveEndpoint(currentEndpoint)
	}

	if err := ctl.syncAgentConfig(ctx, node); err != nil {
		return reconcile.Result{}, err
	}

	if err := ctl.syncAgentPod(ctx, &node); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (ctl *agentController) isValidSubnets(cidrs []string) bool {
	for _, cidr := range cidrs {
		_, subnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return false
		}

		if !ctl.alloc.Contains(*subnet) {
			return false
		}
	}

	return true
}

func (ctl *agentController) clearAllocatedResourcesForEdgeNode(ctx context.Context, nodeName string) error {
	if err := ctl.deleteAgentPodIfNeeded(ctx, nodeName); err != nil {
		return err
	}

	if err := ctl.deleteAgentConfigIfNeeded(ctx, nodeName); err != nil {
		return err
	}

	return ctl.reclaimSubnets(nodeName)
}

func (ctl *agentController) reclaimSubnets(nodeName string) error {
	log := ctl.log.WithValues("nodeName", nodeName)

	ep, ok := ctl.store.GetEndpoint(nodeName)
	if !ok {
		return nil
	}
	ctl.store.DeleteEndpoint(nodeName)
	log.V(5).Info("endpoint is delete from store", "endpoint", ep)

	for _, sn := range ep.Subnets {
		_, subnet, err := net.ParseCIDR(sn)
		if err != nil {
			log.Error(err, "invalid subnet, skip reclaiming subnets")
			continue
		}
		ctl.alloc.Reclaim(*subnet)
		log.V(5).Info("subnet is reclaimed", "subnet", subnet)
	}

	return nil
}

func (ctl *agentController) deleteAgentPodIfNeeded(ctx context.Context, nodeName string) error {
	agentPodName := getAgentPodName(nodeName)
	key := ObjectKey{
		Name:      agentPodName,
		Namespace: ctl.namespace,
	}

	var pod corev1.Pod
	if err := ctl.client.Get(ctx, key, &pod); err != nil {
		return err
	}

	ctl.log.V(5).Info("Agent pod is found, delete it now", "nodeName", nodeName, "podName", agentPodName, "namespace", ctl.namespace)

	err := ctl.client.Delete(ctx, &pod)
	if err != nil {
		ctl.log.Error(err, "failed to delete agent pod")
	}
	return err
}

func (ctl *agentController) allocateSubnet(ctx context.Context, node corev1.Node) error {
	log := ctl.log.WithValues("nodeName", node.Name)

	log.V(5).Info("this node need subnet allocation")
	subnet, err := ctl.alloc.GetFreeSubnetBlock(node.Name)
	if err != nil {
		log.Error(err, "failed to allocate subnet for node")
		return err
	}

	log = log.WithValues("subnet", subnet.String())
	log.V(5).Info("an subnet is allocated to node")

	if node.Annotations == nil {
		node.Annotations = map[string]string{}
	}
	// for now, we just supply one subnet allocation
	node.Annotations[constants.KeyNodeSubnets] = subnet.String()

	err = ctl.client.Update(ctx, &node)
	if err != nil {
		log.Error(err, "failed to record node subnet allocation")

		ctl.alloc.Reclaim(*subnet)
		log.V(5).Info("subnet is reclaimed")
		return err
	}

	ctl.store.SaveEndpoint(ctl.newEndpoint(node))
	return nil
}

func (ctl *agentController) syncAgentConfig(ctx context.Context, node corev1.Node) error {
	configName := getAgentConfigMapName(node.Name)
	log := ctl.log.WithValues("nodeName", node.Name, "configName", configName, "namespace", ctl.namespace)

	log.V(5).Info("Sync agent config")

	var agentConfig corev1.ConfigMap
	err := ctl.client.Get(ctx, ObjectKey{Name: configName, Namespace: ctl.namespace}, &agentConfig)
	if err != nil && !errors.IsNotFound(err) {
		ctl.log.Error(err, "failed to get agent configmap")
		return err
	}
	isConfigNotFound := errors.IsNotFound(err)

	networkConf := ctl.buildNetworkConf(node.Name)
	configDataBytes, err := yaml.Marshal(networkConf)
	if err != nil {
		ctl.log.Error(err, "not able to marshal NetworkConf")
		return err
	}

	configData := string(configDataBytes)

	if isConfigNotFound {
		ctl.log.V(5).Info("Agent configMap is not found, create it now")
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: ctl.namespace,
				Labels: map[string]string{
					constants.KeyFabedgeAPP: constants.AppAgent,
					constants.KeyCreatedBy:  constants.AppOperator,
				},
			},
			Data: map[string]string{
				agentConfigTunnelFileName: configData,
				// agent controller just create configmap, the load balance rules is kept by proxy controller
				agentConfigServicesFileName: "",
			},
		}

		return ctl.client.Create(ctx, configMap)
	}

	if configData == agentConfig.Data[agentConfigTunnelFileName] {
		log.V(5).Info("agent config is not changed, skip updating")
		return nil
	}

	agentConfig.Data[agentConfigTunnelFileName] = configData
	err = ctl.client.Update(ctx, &agentConfig)
	if err != nil {
		log.Error(err, "failed to update agent configmap")
	}

	return err
}

func (ctl *agentController) syncAgentPod(ctx context.Context, node *corev1.Node) error {
	agentPodName := getAgentPodName(node.Name)

	log := ctl.log.WithValues("nodeName", node.Name, "podName", agentPodName, "namespace", ctl.namespace)

	var oldPod corev1.Pod
	err := ctl.client.Get(ctx, ObjectKey{Name: agentPodName, Namespace: ctl.namespace}, &oldPod)
	switch {
	case err == nil:
		newPod := ctl.buildAgentPod(ctl.namespace, node.Name, agentPodName)
		if newPod.Labels[constants.KeyPodHash] == oldPod.Labels[constants.KeyPodHash] {
			return nil
		}

		// we will not create agent pod now because pod termination may last for a long time,
		// during that time, create pod may get collision error
		log.V(3).Info("agent pod may be out of date, delete it")
		err = ctl.client.Delete(context.TODO(), &oldPod)
		if err != nil {
			log.Error(err, "failed to delete agent pod")
		}
		return err
	case errors.IsNotFound(err):
		log.V(5).Info("Agent pod is not found, create it now")
		newPod := ctl.buildAgentPod(ctl.namespace, node.Name, agentPodName)
		err = ctl.client.Create(ctx, newPod)
		if err != nil {
			log.Error(err, "failed to create agent pod")
		}
		return err
	default:
		log.Error(err, "failed to get agent pod")
		return err
	}
}

func (ctl *agentController) buildAgentPod(namespace, nodeName, podName string) *corev1.Pod {
	hostPathDirectory := corev1.HostPathDirectory
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	hostPathFile := corev1.HostPathFile
	privileged := true
	defaultMode := int32(420)
	agentConfigName := getAgentConfigMapName(nodeName)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				constants.KeyFabedgeAPP: constants.AppAgent,
				constants.KeyCreatedBy:  constants.AppOperator,
			},
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			HostNetwork:   true,
			RestartPolicy: corev1.RestartPolicyAlways,
			Tolerations: []corev1.Toleration{
				{
					Key:    "node-role.kubernetes.io/edge",
					Effect: corev1.TaintEffectNoSchedule,
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "agent",
					Image:           ctl.agentImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args: []string{
						"-tunnels-conf",
						agentConfigTunnelsFilepath,
						"-services-conf",
						agentConfigServicesFilepath,
						"-edge-pod-cidr",
						ctl.edgePodCIRD,
						fmt.Sprintf("-masq-outgoing=%t", ctl.masqOutgoing),
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
					Resources: corev1.ResourceRequirements{},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "netconf",
							MountPath: "/etc/fabedge",
						},
						{
							Name:      "var-run",
							MountPath: "/var/run/",
						},
						{
							Name:      "cni",
							MountPath: "/etc/cni",
						},
						{
							Name:      "lib-modules",
							MountPath: "/lib/modules",
							ReadOnly:  true,
						},
						{
							Name:      "ipsec-d",
							MountPath: "/etc/ipsec.d",
							ReadOnly:  true,
						},
					},
				},
				{
					Name:            "strongswan",
					Image:           ctl.strongswanImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
					Resources: corev1.ResourceRequirements{},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "var-run",
							MountPath: "/var/run/",
						},
						{
							Name:      "ipsec-d",
							MountPath: "/etc/ipsec.d",
							ReadOnly:  true,
						},
						{
							Name:      "ipsec-secrets",
							MountPath: "/etc/ipsec.secrets",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "var-run",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "cni",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/cni",
							Type: &hostPathDirectoryOrCreate,
						},
					},
				},
				{
					Name: "lib-modules",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/modules",
							Type: &hostPathDirectory,
						},
					},
				},
				{
					Name: "netconf",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: agentConfigName,
							},
							DefaultMode: &defaultMode,
						},
					},
				},
				{
					Name: "ipsec-d",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/fabedge/ipsec",
							Type: &hostPathDirectory,
						},
					},
				},
				{
					Name: "ipsec-secrets",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/fabedge/ipsec/ipsec.secrets",
							Type: &hostPathFile,
						},
					},
				},
			},
		},
	}

	pod.Labels[constants.KeyPodHash] = computePodHash(pod.Spec)
	return pod
}

func (ctl *agentController) deleteAgentConfigIfNeeded(ctx context.Context, nodeName string) error {
	configName := getAgentConfigMapName(nodeName)

	log := ctl.log.WithValues("nodeName", nodeName, "configName", configName, "namespace", ctl.namespace)

	var cm corev1.ConfigMap
	if err := ctl.client.Get(ctx, ObjectKey{Name: configName, Namespace: ctl.namespace}, &cm); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}

		log.Error(err, "failed to get configmap")
		return err
	}

	log.V(5).Info("Agent configmap is found, delete it now")
	err := ctl.client.Delete(ctx, &cm)

	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
		} else {
			log.Error(err, "failed to delete agent configmap")
		}
	}

	return err
}

// getNetworkConfig to parse network config from connector configmap or agent configmap
func (ctl *agentController) getNetworkConfig(ctx context.Context, namespace, cmName, configFile string) (cm corev1.ConfigMap, conf netconf.NetworkConf, err error) {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      cmName,
	}
	if err = ctl.client.Get(ctx, key, &cm); err != nil {
		return
	}

	tmp := cm.Data[configFile]
	if err = yaml.Unmarshal([]byte(tmp), &conf); err != nil {
		return
	}
	return
}

func (ctl *agentController) buildNetworkConf(name string) netconf.NetworkConf {
	store := ctl.store
	endpoint, _ := store.GetEndpoint(name)
	peerEndpoints := ctl.getPeers(name)

	conf := netconf.NetworkConf{
		TunnelEndpoint: endpoint.ConvertToTunnelEndpoint(),
		Peers:          make([]netconf.TunnelEndpoint, 0, len(peerEndpoints)),
	}

	for _, ep := range peerEndpoints {
		conf.Peers = append(conf.Peers, ep.ConvertToTunnelEndpoint())
	}

	return conf
}

func (ctl *agentController) getPeers(name string) []types.Endpoint {
	store := ctl.store
	nameSet := stringset.New(constants.ConnectorEndpointName)

	for _, community := range store.GetCommunitiesByEndpoint(name) {
		nameSet.Add(community.Members.Values()...)
	}
	nameSet.Remove(name)

	return store.GetEndpoints(nameSet.Values()...)
}

func getAgentConfigMapName(nodeName string) string {
	return fmt.Sprintf("fabedge-agent-config-%s", nodeName)
}

func getAgentPodName(nodeName string) string {
	return fmt.Sprintf("fabedge-agent-%s", nodeName)
}

// ComputeHash returns a hash value calculated from pod spec
func computePodHash(spec corev1.PodSpec) string {
	hasher := fnv.New32a()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	_, _ = printer.Fprintf(hasher, "%#v", spec)

	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}
