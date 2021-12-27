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

package connector

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/jjeffery/stringset"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerpkg "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
	certutil "github.com/fabedge/fabedge/pkg/util/cert"
	nodeutil "github.com/fabedge/fabedge/pkg/util/node"
	secretutil "github.com/fabedge/fabedge/pkg/util/secret"
)

const (
	controllerName = "connector-controller"
)

type Node struct {
	Name     string
	IP       string
	PodCIDRs []string
}

type Config struct {
	Namespace       string
	Endpoint        apis.Endpoint
	ProvidedSubnets []string
	GetPodCIDRs     types.PodCIDRsGetter
	CertManager     certutil.Manager

	CertOrganization string
	SyncInterval     time.Duration

	Store   storepkg.Interface
	Manager manager.Manager
}

// controller generate tunnels config for connector and
// provide connector endpoint info for others
type controller struct {
	Config

	client client.Client
	log    logr.Logger

	nodeNameSet stringset.Set
	nodeCache   map[string]Node
	mux         sync.RWMutex
}

func AddToManager(cnf Config) (types.EndpointGetter, error) {
	mgr := cnf.Manager

	ctl := &controller{
		Config: cnf,

		nodeNameSet: stringset.New(),
		nodeCache:   make(map[string]Node),
		client:      mgr.GetClient(),
		log:         mgr.GetLogger().WithName(controllerName),
	}

	err := ctl.initializeConnectorEndpoint()
	if err != nil {
		return nil, err
	}

	err = mgr.Add(manager.RunnableFunc(ctl.SyncConnectorConfig))
	if err != nil {
		return nil, err
	}

	c, err := controllerpkg.New(
		controllerName,
		mgr,
		controllerpkg.Options{
			Reconciler: reconcile.Func(ctl.onNodeRequest),
		},
	)
	if err != nil {
		return nil, err
	}

	return ctl.getConnectorEndpoint, c.Watch(
		&source.Kind{Type: &corev1.Node{}},
		&handler.EnqueueRequestForObject{},
	)
}

func (ctl *controller) SyncConnectorConfig(ctx context.Context) error {
	tick := time.NewTicker(ctl.SyncInterval)

	ctl.updateConfigMapIfNeeded()
	ctl.generateCertIfNeeded()
	for {
		select {
		case <-tick.C:
			ctl.updateConfigMapIfNeeded()
			ctl.generateCertIfNeeded()
		case <-ctx.Done():
			return nil
		}
	}
}

func (ctl *controller) updateConfigMapIfNeeded() {
	key := client.ObjectKey{
		Name:      constants.ConnectorConfigName,
		Namespace: ctl.Namespace,
	}
	log := ctl.log.WithValues("key", key)

	ctx, cancel := context.WithTimeout(context.Background(), ctl.SyncInterval)
	defer cancel()

	connectorEndpoint := ctl.getConnectorEndpoint()
	conf := netconf.NetworkConf{
		Endpoint: connectorEndpoint,
		Peers:    ctl.getPeers(),
	}

	confBytes, err := yaml.Marshal(conf)
	if err != nil {
		log.Error(err, "failed to marshal connector tunnels conf")
		return
	}

	configData := string(confBytes)

	var cm corev1.ConfigMap
	err = ctl.client.Get(ctx, key, &cm)
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "failed to get connector configmap")
		return
	}

	if errors.IsNotFound(err) {
		log.V(5).Info("connector config is not found, create it now")

		cm = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
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

func (ctl *controller) generateCertIfNeeded() {
	key := client.ObjectKey{
		Name:      constants.ConnectorTLSName,
		Namespace: ctl.Namespace,
	}
	log := ctl.log.WithValues("key", key)

	ctx, cancel := context.WithTimeout(context.Background(), ctl.SyncInterval)
	defer cancel()

	var secret corev1.Secret
	err := ctl.client.Get(ctx, key, &secret)
	if err != nil {
		if !errors.IsNotFound(err) {
			ctl.log.Error(err, "failed to get secret")
			return
		}

		log.V(5).Info("TLS secret for connector is not found, generate it now")
		secret, err = ctl.buildCertAndKeySecret(key)
		if err != nil {
			log.Error(err, "failed to create cert and key for connector")
			return
		}

		err = ctl.client.Create(ctx, &secret)
		if err != nil {
			log.Error(err, "failed to create secret")
		}
		return
	}

	certPEM := secretutil.GetCert(secret)
	err = ctl.CertManager.VerifyCertInPEM(certPEM, certutil.ExtKeyUsagesServerAndClient)
	if err == nil {
		log.V(5).Info("cert is verified")
		return
	}

	log.Error(err, "failed to verify cert, need to regenerate a cert for connector")
	secret, err = ctl.buildCertAndKeySecret(key)
	if err != nil {
		log.Error(err, "failed to recreate cert and key for connector")
		return
	}

	err = ctl.client.Update(ctx, &secret)
	if err != nil {
		log.Error(err, "failed to save secret")
	}
}

func (ctl *controller) buildCertAndKeySecret(key client.ObjectKey) (corev1.Secret, error) {
	keyDER, csr, err := certutil.NewCertRequest(certutil.Request{
		CommonName:   ctl.Endpoint.Name,
		Organization: []string{ctl.CertOrganization},
	})
	if err != nil {
		return corev1.Secret{}, err
	}

	certDER, err := ctl.CertManager.SignCert(csr)
	if err != nil {
		return corev1.Secret{}, err
	}

	return secretutil.TLSSecret().
		Name(key.Name).
		Namespace(key.Namespace).
		EncodeCert(certDER).
		EncodeKey(keyDER).
		CACertPEM(ctl.CertManager.GetCACertPEM()).
		Label(constants.KeyCreatedBy, constants.AppOperator).Build(), nil
}

func (ctl *controller) getPeers() []apis.Endpoint {
	connectorName := ctl.Endpoint.Name

	nameSet := ctl.Store.GetLocalEndpointNames()
	for _, community := range ctl.Store.GetCommunitiesByEndpoint(connectorName) {
		for name := range community.Members {
			nameSet.Add(name)
		}
	}
	nameSet.Remove(connectorName)

	endpoints := ctl.Store.GetEndpoints(nameSet.Values()...)

	peers := make([]apis.Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		peers = append(peers, ep)
	}

	return peers
}

func (ctl *controller) onNodeRequest(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := ctl.log.WithValues("request", request)

	var node corev1.Node
	if err := ctl.client.Get(ctx, request.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			ctl.removeNode(request.Name)
			return reconcile.Result{}, nil
		}

		log.Error(err, "failed to get node")
		return reconcile.Result{}, err
	}

	if node.DeletionTimestamp != nil || nodeutil.IsEdgeNode(node) {
		ctl.removeNode(request.Name)
		return reconcile.Result{}, nil
	}

	ctl.addNode(node, true)

	return reconcile.Result{}, nil
}

func (ctl *controller) addNode(node corev1.Node, rebuild bool) {
	ip, podCIDRs := nodeutil.GetIP(node), ctl.GetPodCIDRs(node)
	if len(ip) == 0 || len(podCIDRs) == 0 {
		ctl.log.V(5).Info("this node has no IP or PodCIDRs, skip adding it", "nodeName", node.Name)
		return
	}

	ctl.mux.Lock()
	defer ctl.mux.Unlock()
	if ctl.nodeNameSet.Contains(node.Name) {
		return
	}

	ctl.nodeNameSet.Add(node.Name)
	ctl.nodeCache[node.Name] = Node{
		Name:     node.Name,
		IP:       ip,
		PodCIDRs: podCIDRs,
	}

	if rebuild {
		ctl.rebuildConnectorEndpoint()
	}
}

func (ctl *controller) removeNode(nodeName string) {
	ctl.mux.Lock()
	defer ctl.mux.Unlock()

	if !ctl.nodeNameSet.Contains(nodeName) {
		return
	}

	ctl.nodeNameSet.Remove(nodeName)
	delete(ctl.nodeCache, nodeName)

	ctl.rebuildConnectorEndpoint()
}

func (ctl *controller) initializeConnectorEndpoint() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var nodes corev1.NodeList
	err := ctl.client.List(ctx, &nodes)
	if err != nil {
		return err
	}

	for _, node := range nodes.Items {
		if nodeutil.IsEdgeNode(node) {
			continue
		}
		ctl.addNode(node, false)
	}

	ctl.rebuildConnectorEndpoint()

	return nil
}

func (ctl *controller) rebuildConnectorEndpoint() {
	subnets := make([]string, 0, len(ctl.ProvidedSubnets)+len(ctl.nodeCache))
	nodeSubnets := make([]string, 0, len(ctl.nodeCache))

	subnets = append(subnets, ctl.ProvidedSubnets...)
	for _, nodeName := range ctl.nodeNameSet.Values() {
		node := ctl.nodeCache[nodeName]

		subnets = append(subnets, node.PodCIDRs...)
		nodeSubnets = append(nodeSubnets, node.IP)
	}

	ctl.Endpoint.Subnets = subnets
	ctl.Endpoint.NodeSubnets = nodeSubnets
	ctl.Endpoint.Type = apis.Connector
	ctl.Store.SaveEndpointAsLocal(ctl.Endpoint)
}

func (ctl *controller) getConnectorEndpoint() apis.Endpoint {
	ctl.mux.RLock()
	defer ctl.mux.RUnlock()

	return ctl.Endpoint
}
