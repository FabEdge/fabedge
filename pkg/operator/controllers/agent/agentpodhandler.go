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

package agent

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/types"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

const (
	agentNamePrefix = "fabedge-agent-"
	keyArgument     = "argument.fabedge.io"
)

var _ Handler = &agentPodHandler{}

type agentPodHandler struct {
	namespace string

	agentImage      string
	strongswanImage string
	imagePullPolicy corev1.PullPolicy
	argMap          types.AgentArgumentMap
	args            []string
	agentNameSet    *types.SafeStringSet

	client client.Client
	log    logr.Logger
}

func (handler *agentPodHandler) Do(ctx context.Context, node corev1.Node) error {
	agentName := getAgentName(node.Name)

	log := handler.log.WithValues("nodeName", node.Name, "agentName", agentName, "namespace", handler.namespace)

	oldPod, err := handler.getAgentPod(ctx, agentName)
	switch {
	case err == nil:
		if oldPod.DeletionTimestamp != nil {
			return errRequeueRequest
		}

		needRestart := ctx.Value(keyRestartAgent) == errRestartAgent
		if !needRestart {
			newPod := handler.buildAgentPod(handler.namespace, agentName, node)
			needRestart = newPod.Labels[constants.KeyPodHash] != oldPod.Labels[constants.KeyPodHash]
		}

		if !needRestart {
			return nil
		}

		// we will not create agent pod now because pod termination may last for a long time,
		// during that time, create pod may get collision error
		log.V(3).Info("need to restart pod, delete it now")
		if err = handler.client.Delete(context.TODO(), &oldPod); err != nil {
			log.Error(err, "failed to delete agent pod")
			return err
		}

		handler.agentNameSet.Delete(agentName)
		return nil
	case errors.IsNotFound(err):
		// sometimes agentController might receive successive events for the same node in short time,
		// this might cause agentPodHandler to create redundant agent pods for the same node, because
		// the pods in cache might be different from real pods in apiserver. So here agentPodHandler will
		// check if an agent has already been created for a node, if agentNameSet contains the agentName,
		// we requeue this request to avoid cache problem.
		if handler.agentNameSet.Has(agentName) {
			log.Error(nil, "agent for this node has already created, this might caused by cache problem", "node", node.Name)

			// Sometimes pods are deleted by other tools, if that happens agentNameSet might contain agentName while
			// agent pod doesn't exist. So here we delete agentName from agentNameSet no matter what happened. Anyway
			// when delayed request arrives again, the cache should catch up the data from api-server
			handler.agentNameSet.Delete(agentName)
			return errRequeueRequest
		}

		log.V(5).Info("Agent pod is not found, create it now")
		newPod := handler.buildAgentPod(handler.namespace, agentName, node)

		if err = controllerutil.SetControllerReference(&node, newPod, scheme.Scheme); err != nil {
			log.Error(err, "failed to set ownerReference to TLS secret")
			return err
		}

		if err = handler.client.Create(ctx, newPod); err != nil {
			log.Error(err, "failed to create agent pod")
			return err
		}

		handler.agentNameSet.Insert(agentName)
		return nil
	default:
		log.Error(err, "failed to get agent pod")
		return err
	}
}

func (handler *agentPodHandler) getAgentPod(ctx context.Context, podName string) (pod corev1.Pod, err error) {
	var podList corev1.PodList
	err = handler.client.List(ctx, &podList,
		client.MatchingLabels{
			constants.KeyFabEdgeName: podName,
			constants.KeyFabEdgeAPP:  constants.AppAgent,
			constants.KeyCreatedBy:   constants.AppOperator,
		},
		client.InNamespace(handler.namespace))
	if err != nil {
		return
	}

	if len(podList.Items) == 0 {
		return pod, errors.NewNotFound(schema.GroupResource{Resource: "pods"}, podName)
	}

	return podList.Items[0], nil
}

func (handler *agentPodHandler) buildAgentPod(namespace, podName string, node corev1.Node) *corev1.Pod {
	hostPathDirectory := corev1.HostPathDirectory
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	privileged := true
	defaultMode := int32(420)
	automountServiceAccountToken := false

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: agentNamePrefix,
			Namespace:    namespace,
			Labels: map[string]string{
				constants.KeyFabEdgeAPP:  constants.AppAgent,
				constants.KeyCreatedBy:   constants.AppOperator,
				constants.KeyFabEdgeName: podName,
			},
		},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: &automountServiceAccountToken,
			NodeName:                     node.Name,
			HostNetwork:                  true,
			RestartPolicy:                corev1.RestartPolicyAlways,
			Tolerations: []corev1.Toleration{
				{
					Key:      "",
					Operator: corev1.TolerationOpExists,
				},
			},
			InitContainers: []corev1.Container{
				{
					Name:            "environment-prepare",
					Image:           handler.agentImage,
					ImagePullPolicy: handler.imagePullPolicy,
					Command: []string{
						"/plugins/env_prepare.sh",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: &privileged,
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "cni-bin",
							MountPath: "/opt/cni/bin",
						},
						{
							Name:      "cni-cache",
							MountPath: "/var/lib/cni/cache",
						},
						{
							Name:      "cni-config",
							MountPath: "/etc/cni/net.d",
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "agent",
					Image:           handler.agentImage,
					ImagePullPolicy: handler.imagePullPolicy,
					Args:            handler.buildAgentArgs(node),
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
							Name:      "cni-config",
							MountPath: "/etc/cni/net.d",
						},
						{
							Name:      "var-run",
							MountPath: "/var/run/",
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
						{
							Name:      "agent-workdir",
							MountPath: "/var/lib/fabedge",
						},
					},
				},
				{
					Name:            "strongswan",
					Image:           handler.strongswanImage,
					ImagePullPolicy: handler.imagePullPolicy,
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
							SubPath:   "ipsec.secrets",
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
								Name: getAgentConfigMapName(node.Name),
							},
							DefaultMode: &defaultMode,
						},
					},
				},
				{
					Name: "ipsec-d",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  getCertSecretName(node.Name),
							DefaultMode: &defaultMode,
							Items: []corev1.KeyToPath{
								{
									Key:  secretutil.KeyCACert,
									Path: "cacerts/ca.crt",
								},
								{
									Key:  corev1.TLSCertKey,
									Path: "certs/tls.crt",
								},
								{
									Key:  corev1.TLSPrivateKeyKey,
									Path: "private/tls.key",
								},
							},
						},
					},
				},
				{
					Name: "ipsec-secrets",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  getCertSecretName(node.Name),
							DefaultMode: &defaultMode,
							Items: []corev1.KeyToPath{
								{
									Key:  secretutil.KeyIPSecSecretsFile,
									Path: "ipsec.secrets",
								},
							},
						},
					},
				},
				{
					Name: "cni-config",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/cni/net.d",
							Type: &hostPathDirectoryOrCreate,
						},
					},
				},
				{
					Name: "cni-bin",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/opt/cni/bin",
							Type: &hostPathDirectoryOrCreate,
						},
					},
				},
				{
					Name: "cni-cache",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/var/lib/cni/cache",
							Type: &hostPathDirectoryOrCreate,
						},
					},
				},
				{
					Name: "agent-workdir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
		},
	}

	if handler.argMap.IsProxyEnabled() {
		xtablesHostType := corev1.HostPathFileOrCreate
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "xtables-lock",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Type: &xtablesHostType,
					Path: "/run/xtables.lock",
				},
			},
		})

		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "xtables-lock",
			MountPath: "/run/xtables.lock",
		})
	}

	if handler.argMap.IsDNSProbeEnabled() && handler.argMap.IsDNSProbeEnabled() {
		pod.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/health",
					Port:   intstr.FromInt(8080),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			FailureThreshold: 3,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			TimeoutSeconds:   1,
		}

		pod.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/ready",
					Port:   intstr.FromInt(8181),
					Scheme: corev1.URISchemeHTTP,
				},
			},
			FailureThreshold: 3,
			PeriodSeconds:    10,
			SuccessThreshold: 1,
			TimeoutSeconds:   1,
		}
	}

	pod.Labels[constants.KeyPodHash] = computePodHash(pod.Spec)
	return pod
}

func (handler *agentPodHandler) buildAgentArgs(node corev1.Node) []string {
	argMap := types.NewAgentArgumentMap()
	for key, value := range node.Annotations {
		if strings.HasPrefix(key, keyArgument) {
			// 20 is the length of "argument.fabedge.io/"
			argMap.Set(key[20:], value)
		}
	}

	if len(argMap) == 0 {
		return handler.args
	}

	for key, value := range handler.argMap {
		if !argMap.HasKey(key) {
			argMap.Set(key, value)
		}
	}

	return argMap.ArgumentArray()
}

func (handler *agentPodHandler) Undo(ctx context.Context, nodeName string) error {
	agentName := getAgentName(nodeName)
	pod, err := handler.getAgentPod(ctx, agentName)
	if err != nil {
		if errors.IsNotFound(err) {
			handler.agentNameSet.Delete(agentName)
			return nil
		}

		handler.log.Error(err, "failed to get pod", "name", agentName, "namespace", handler.namespace)
		return err
	}

	err = handler.client.Delete(ctx, &pod)
	if err != nil {
		handler.log.Error(err, "failed to delete pod", "name", pod.Name, "namespace", pod.Namespace)
		return err
	}

	handler.agentNameSet.Delete(agentName)
	return nil
}

func getAgentName(nodeName string) string {
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
