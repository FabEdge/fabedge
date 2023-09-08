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

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ Handler = &configHandler{}

type configHandler struct {
	namespace            string
	store                storepkg.Interface
	getEndpointName      types.GetNameFunc
	getConnectorEndpoint types.EndpointGetter
	client               client.Client
	log                  logr.Logger
}

func (handler *configHandler) Do(ctx context.Context, node corev1.Node) error {
	configName := getAgentConfigMapName(node.Name)
	log := handler.log.WithValues("nodeName", node.Name, "configName", configName, "namespace", handler.namespace)

	log.V(5).Info("Sync agent config")

	var agentConfig corev1.ConfigMap
	err := handler.client.Get(ctx, ObjectKey{Name: configName, Namespace: handler.namespace}, &agentConfig)
	if err != nil && !errors.IsNotFound(err) {
		handler.log.Error(err, "failed to get agent configmap")
		return err
	}
	isConfigNotFound := errors.IsNotFound(err)

	networkConf := handler.buildNetworkConf(node.Name)
	configDataBytes, err := yaml.Marshal(networkConf)
	if err != nil {
		handler.log.Error(err, "not able to marshal NetworkConf")
		return err
	}

	configData := string(configDataBytes)

	if isConfigNotFound {
		handler.log.V(5).Info("Agent configMap is not found, create it now")
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: handler.namespace,
				Labels: map[string]string{
					constants.KeyFabEdgeAPP: constants.AppAgent,
					constants.KeyCreatedBy:  constants.AppOperator,
				},
			},
			Data: map[string]string{
				agentConfigTunnelFileName: configData,
			},
		}

		if err = controllerutil.SetControllerReference(&node, configMap, scheme.Scheme); err != nil {
			log.Error(err, "failed to set ownerReference to configmap")
			return err
		}

		return handler.client.Create(ctx, configMap)
	}

	if configData == agentConfig.Data[agentConfigTunnelFileName] {
		log.V(5).Info("agent config is not changed, skip updating")
		return nil
	}

	agentConfig.Data[agentConfigTunnelFileName] = configData
	if err = controllerutil.SetControllerReference(&node, &agentConfig, scheme.Scheme); err != nil {
		log.Error(err, "failed to set ownerReference to configmap")
		return err
	}

	err = handler.client.Update(ctx, &agentConfig)
	if err != nil {
		log.Error(err, "failed to update agent configmap")
	}

	return err
}

func (handler *configHandler) buildNetworkConf(nodeName string) netconf.NetworkConf {
	store := handler.store

	epName := handler.getEndpointName(nodeName)
	endpoint, _ := store.GetEndpoint(epName)
	peerEndpoints := handler.getPeers(epName)

	conf := netconf.NetworkConf{
		Endpoint: endpoint,
		Peers:    make([]apis.Endpoint, 0, len(peerEndpoints)),
	}

	conf.Peers = append(conf.Peers, peerEndpoints...)

	mediator, found := store.GetEndpoint(constants.DefaultMediatorName)
	if found {
		conf.Mediator = &mediator
	}

	return conf
}

func (handler *configHandler) getPeers(name string) []apis.Endpoint {
	store := handler.store
	nameSet := sets.NewString()

	for _, community := range store.GetCommunitiesByEndpoint(name) {
		nameSet.Insert(community.Members.List()...)
	}
	nameSet.Delete(name)

	endpoints := make([]apis.Endpoint, 0, len(nameSet)+1)
	// always put connector endpoint first
	endpoints = append(endpoints, handler.getConnectorEndpoint())
	endpoints = append(endpoints, store.GetEndpoints(nameSet.List()...)...)

	return endpoints
}

func (handler *configHandler) Undo(ctx context.Context, nodeName string) error {
	config := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getAgentConfigMapName(nodeName),
			Namespace: handler.namespace,
		},
	}
	err := handler.client.Delete(ctx, &config)
	if err != nil {
		if errors.IsNotFound(err) {
			err = nil
		} else {
			handler.log.Error(err, "failed to delete configmap", "name", config.Name, "namespace", config.Namespace)
		}
	}
	return err
}

func getAgentConfigMapName(nodeName string) string {
	return fmt.Sprintf("fabedge-agent-config-%s", nodeName)
}
