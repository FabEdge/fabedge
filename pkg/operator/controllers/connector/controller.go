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

package connector

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

const (
	controllerName = "connector-config-controller"
)

// controller only controls connector tunnels configmap
type controller struct {
	interval     time.Duration
	configMapKey client.ObjectKey

	store                storepkg.Interface
	getConnectorEndpoint types.EndpointGetter
	client               client.Client
	log                  logr.Logger
}

type Config struct {
	Namespace           string
	ConnectorConfigName string
	Interval            time.Duration

	Store                storepkg.Interface
	GetConnectorEndpoint types.EndpointGetter

	Manager manager.Manager
}

func AddToManager(cnf Config) error {
	mgr := cnf.Manager

	ctl := &controller{
		configMapKey: client.ObjectKey{Name: cnf.ConnectorConfigName, Namespace: cnf.Namespace},
		interval:     cnf.Interval,

		getConnectorEndpoint: cnf.GetConnectorEndpoint,
		store:                cnf.Store,
		log:                  mgr.GetLogger().WithName(controllerName),
		client:               mgr.GetClient(),
	}

	return mgr.Add(manager.RunnableFunc(ctl.Start))
}

func (ctl *controller) Start(ctx context.Context) error {
	tick := time.NewTicker(ctl.interval)

	for {
		select {
		case <-tick.C:
			ctl.updateConfigMapIfNeeded()
		case <-ctx.Done():
			return nil
		}
	}
}

func (ctl *controller) updateConfigMapIfNeeded() {
	log := ctl.log.WithValues("key", ctl.configMapKey)

	ctx, cancel := context.WithTimeout(context.Background(), ctl.interval)
	defer cancel()

	connectorEndpoint := ctl.getConnectorEndpoint()
	conf := netconf.NetworkConf{
		TunnelEndpoint: connectorEndpoint.ConvertToTunnelEndpoint(),
		Peers:          ctl.getPeers(),
	}

	confBytes, err := yaml.Marshal(conf)
	if err != nil {
		log.Error(err, "failed to marshal connector tunnels conf")
		return
	}

	configData := string(confBytes)

	var cm corev1.ConfigMap
	err = ctl.client.Get(ctx, ctl.configMapKey, &cm)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to get connector configmap")
		return
	}

	if errors.IsNotFound(err) {
		log.V(5).Info("connector config is not found, create it now")

		cm = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ctl.configMapKey.Name,
				Namespace: ctl.configMapKey.Namespace,
			},
			Data: map[string]string{
				constants.ConnectorConfigFileName: configData,
			},
		}
		if err = ctl.client.Create(ctx, &cm); err != nil {
			log.Error(err, "failed to create connector configmap")
		}
		return
	}

	if cm.Data[constants.ConnectorConfigFileName] == configData {
		log.V(5).Info("node endpoints are not changed, skip updating")
		return
	}

	log.V(5).Info("connector tunnels are changed, update it now")
	cm.Data[constants.ConnectorConfigFileName] = configData
	if err = ctl.client.Update(ctx, &cm); err != nil {
		log.Error(err, "failed to update connector configmap")
	}
}

func (ctl *controller) getPeers() []netconf.TunnelEndpoint {
	nameSet := ctl.store.GetAllEndpointNames()
	endpoints := ctl.store.GetEndpoints(nameSet.Values()...)

	peers := make([]netconf.TunnelEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		peers = append(peers, ep.ConvertToTunnelEndpoint())
	}

	return peers
}
