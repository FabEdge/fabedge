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

package proxy

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fabedge/fabedge/pkg/common/netconf"
)

const (
	agentConfigLoadBalanceFileName = "services.yaml"
)

// loadBalanceConfigKeeper is responsible for generating
// proxy rules for each edge node
// todo: put load balance configmap maintenance to agent controller
type loadBalanceConfigKeeper struct {
	mu            sync.Mutex
	nodeSet       EdgeNodeSet
	namespace     string
	interval      time.Duration
	ipvsScheduler string

	client client.Client
	log    logr.Logger
}

func (k *loadBalanceConfigKeeper) Start(ctx context.Context) error {
	tick := time.NewTicker(k.interval)

	for {
		select {
		case <-tick.C:
			k.syncRules()
		case <-ctx.Done():
			return nil
		}
	}
}

func (k *loadBalanceConfigKeeper) AddNode(node EdgeNode) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.addNode(node)
}

func (k *loadBalanceConfigKeeper) AddNodeIfNotPresent(node EdgeNode) {
	k.mu.Lock()
	defer k.mu.Unlock()
	_, ok := k.nodeSet[node.Name]
	if ok {
		return
	}

	k.addNode(node)
}

func (k *loadBalanceConfigKeeper) addNode(node EdgeNode) {
	nodeCopy := newEdgeNode(node.Name)
	for spn, svc := range node.ServicePortMap {
		nodeCopy.ServicePortMap[spn] = svc
	}
	for spn, ep := range node.EndpointMap {
		nodeCopy.EndpointMap[spn] = ep
	}

	k.nodeSet[node.Name] = node
}

func (k *loadBalanceConfigKeeper) syncRules() {
	k.mu.Lock()
	nodeSet := k.nodeSet
	k.nodeSet = make(EdgeNodeSet)
	k.mu.Unlock()

	for _, node := range nodeSet {
		servers := make(netconf.VirtualServers, 0, len(node.ServicePortMap))
		for spn, sp := range node.ServicePortMap {
			servers = append(servers, netconf.VirtualServer{
				IP:                  sp.ClusterIP,
				Port:                sp.Port,
				Protocol:            sp.Protocol,
				SessionAffinity:     sp.SessionAffinity,
				StickyMaxAgeSeconds: sp.StickyMaxAgeSeconds,
				Scheduler:           k.ipvsScheduler,
				RealServers:         convertEndpointSetToRealServers(node.EndpointMap[spn]),
			})
		}
		sort.Sort(servers)

		if err := k.writeToConfigMap(node.Name, servers); err != nil {
			// add node back to nodeset to wait next sync loop
			// because the node is old, we should override newest node
			k.AddNodeIfNotPresent(node)
		}
	}
}

func (k *loadBalanceConfigKeeper) writeToConfigMap(nodeName string, servers netconf.VirtualServers) error {
	configName := getAgentConfigMapName(nodeName)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	configDataBytes, err := yaml.Marshal(&servers)
	if err != nil {
		k.log.Info("failed to generate load balance config")
		return err
	}
	configData := string(configDataBytes)

	log := k.log.WithValues("nodeName", nodeName, "configName", configName, "namespace", k.namespace)
	log.V(5).Info("Sync services to configmap")

	var agentConfig corev1.ConfigMap
	err = k.client.Get(ctx, ObjectKey{Name: configName, Namespace: k.namespace}, &agentConfig)
	if err != nil {
		// for now, we don't handle not found error, agent config is created by another controller
		log.Error(err, "failed to get agent configmap")
		return err
	}

	if configData == agentConfig.Data[agentConfigLoadBalanceFileName] {
		log.V(5).Info("services are not changed, skip updating")
		return nil
	}

	agentConfig.Data[agentConfigLoadBalanceFileName] = configData
	err = k.client.Update(ctx, &agentConfig)
	if err != nil {
		log.Error(err, "failed to update agent configmap")
	}

	return err
}

func convertEndpointSetToRealServers(endpointSet EndpointSet) netconf.RealServers {
	servers := make(netconf.RealServers, 0, len(endpointSet))
	for endpoint := range endpointSet {
		servers = append(servers, endpoint)
	}

	sort.Sort(servers)

	return servers
}

func getAgentConfigMapName(nodeName string) string {
	return fmt.Sprintf("fabedge-agent-config-%s", nodeName)
}
