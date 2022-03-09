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

	"github.com/davecgh/go-spew/spew"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/fabedge/fabedge/pkg/common/constants"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

var _ Handler = &agentPodHandler{}

type agentPodHandler struct {
	namespace string

	logLevel          int
	agentImage        string
	strongswanImage   string
	imagePullPolicy   corev1.PullPolicy
	useXfrm           bool
	masqOutgoing      bool
	enableProxy       bool
	enableIPAM        bool
	enableHairpinMode bool
	reserveIPMACDays  int
	networkPluginMTU  int

	client client.Client
	log    logr.Logger
}

func (handler *agentPodHandler) Do(ctx context.Context, node corev1.Node) error {
	agentPodName := getAgentPodName(node.Name)

	log := handler.log.WithValues("nodeName", node.Name, "podName", agentPodName, "namespace", handler.namespace)

	var oldPod corev1.Pod
	err := handler.client.Get(ctx, ObjectKey{Name: agentPodName, Namespace: handler.namespace}, &oldPod)
	switch {
	case err == nil:
		needRestart := ctx.Value(keyRestartAgent) == errRestartAgent
		if !needRestart {
			newPod := handler.buildAgentPod(handler.namespace, node.Name, agentPodName)
			needRestart = newPod.Labels[constants.KeyPodHash] != oldPod.Labels[constants.KeyPodHash]
		}

		if !needRestart {
			return nil
		}

		// we will not create agent pod now because pod termination may last for a long time,
		// during that time, create pod may get collision error
		log.V(3).Info("need to restart pod, delete it now")
		err = handler.client.Delete(context.TODO(), &oldPod)
		if err != nil {
			log.Error(err, "failed to delete agent pod")
		}
		return err
	case errors.IsNotFound(err):
		log.V(5).Info("Agent pod is not found, create it now")
		newPod := handler.buildAgentPod(handler.namespace, node.Name, agentPodName)

		if err = controllerutil.SetControllerReference(&node, newPod, scheme.Scheme); err != nil {
			log.Error(err, "failed to set ownerReference to TLS secret")
			return err
		}

		err = handler.client.Create(ctx, newPod)
		if err != nil {
			log.Error(err, "failed to create agent pod")
		}
		return err
	default:
		log.Error(err, "failed to get agent pod")
		return err
	}
}

func (handler *agentPodHandler) buildAgentPod(namespace, nodeName, podName string) *corev1.Pod {
	hostPathDirectory := corev1.HostPathDirectory
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	privileged := true
	defaultMode := int32(420)
	automountServiceAccountToken := false

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
			AutomountServiceAccountToken: &automountServiceAccountToken,
			NodeName:                     nodeName,
			HostNetwork:                  true,
			RestartPolicy:                corev1.RestartPolicyAlways,
			Tolerations: []corev1.Toleration{
				{
					Key:      "",
					Operator: corev1.TolerationOpExists,
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "agent",
					Image:           handler.agentImage,
					ImagePullPolicy: handler.imagePullPolicy,
					Args: []string{
						"--tunnels-conf",
						agentConfigTunnelsFilepath,
						"--services-conf",
						agentConfigServicesFilepath,
						"--local-cert",
						"tls.crt",
						fmt.Sprintf("--masq-outgoing=%t", handler.masqOutgoing),
						fmt.Sprintf("--enable-ipam=%t", handler.enableIPAM),
						fmt.Sprintf("--enable-hairpinmode=%t", handler.enableHairpinMode),
						fmt.Sprintf("--reserve-ip-mac-days=%d", handler.reserveIPMACDays),
						fmt.Sprintf("--network-plugin-mtu=%d", handler.networkPluginMTU),
						fmt.Sprintf("--use-xfrm=%t", handler.useXfrm),
						fmt.Sprintf("--enable-proxy=%t", handler.enableProxy),
						fmt.Sprintf("-v=%d", handler.logLevel),
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
								Name: getAgentConfigMapName(nodeName),
							},
							DefaultMode: &defaultMode,
						},
					},
				},
				{
					Name: "ipsec-d",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName:  getCertSecretName(nodeName),
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
							SecretName:  getCertSecretName(nodeName),
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
			},
		},
	}

	if handler.enableIPAM {
		container := handler.buildEnvPrepareContainer()
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)

		cniVolumes := []corev1.Volume{
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
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, cniVolumes...)

		volMount := corev1.VolumeMount{
			Name:      "cni-config",
			MountPath: "/etc/cni/net.d",
		}
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, volMount)
	}

	pod.Labels[constants.KeyPodHash] = computePodHash(pod.Spec)
	return pod
}

func (handler *agentPodHandler) buildEnvPrepareContainer() corev1.Container {
	privileged := true
	return corev1.Container{
		Name:            "environment-prepare",
		Image:           handler.agentImage,
		ImagePullPolicy: handler.imagePullPolicy,
		Command: []string{
			"env_prepare.sh",
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
	}
}

func (handler *agentPodHandler) Undo(ctx context.Context, nodeName string) error {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getAgentPodName(nodeName),
			Namespace: handler.namespace,
		},
	}
	err := handler.client.Delete(ctx, &pod)
	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
		} else {
			handler.log.Error(err, "failed to delete pod", "name", pod.Name, "namespace", pod.Namespace)
		}
	}
	return err
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
