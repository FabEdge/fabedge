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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
	IPs      []string
	PodCIDRs []string
}

type Config struct {
	Namespace       string
	Endpoint        apis.Endpoint
	ProvidedSubnets []string
	GetPodCIDRs     types.PodCIDRsGetter
	CertManager     certutil.Manager
	ConnectorLabels map[string]string

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

	nodeNameSet sets.String
	nodeCache   map[string]Node
	mux         sync.RWMutex
}

func AddToManager(cnf Config) (types.EndpointGetter, error) {
	if len(cnf.ConnectorLabels) == 0 {
		return nil, fmt.Errorf("connector labels is needed")
	}

	mgr := cnf.Manager

	ctl := &controller{
		Config: cnf,

		nodeNameSet: sets.NewString(),
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

	ctl.operateConnector()
	for {
		select {
		case <-tick.C:
			ctl.operateConnector()
		case <-ctx.Done():
			return nil
		}
	}
}

func (ctl *controller) operateConnector() {
	ctl.updateConfigMapIfNeeded()
	generated := ctl.generateCertIfNeeded()
	if generated {
		ctl.restartConnectorPods()
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
	mediator, found := ctl.Store.GetEndpoint(constants.DefaultMediatorName)
	if found {
		conf.Mediator = &mediator
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

func (ctl *controller) generateCertIfNeeded() bool {
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
			return false
		}

		log.V(5).Info("TLS secret for connector is not found, generate it now")
		secret, err = ctl.buildCertAndKeySecret(key)
		if err != nil {
			log.Error(err, "failed to create cert and key for connector")
			return false
		}

		err = ctl.client.Create(ctx, &secret)
		if err != nil {
			log.Error(err, "failed to create secret")
			return false
		}

		return true
	}

	err = ctl.verifyCert(secret)
	if err == nil {
		log.V(5).Info("connector's certificate is verified")
		return false
	}

	log.Error(err, "failed to verify cert, need to regenerate a cert for connector")
	secret, err = ctl.buildCertAndKeySecret(key)
	if err != nil {
		log.Error(err, "failed to recreate cert and key for connector")
		return false
	}

	err = ctl.client.Update(ctx, &secret)
	if err != nil {
		log.Error(err, "failed to save secret")
		return false
	}

	return true
}

func (ctl *controller) verifyCert(secret corev1.Secret) error {
	cert, err := parseCertFromSecret(secret)
	if err != nil {
		return err
	}

	if cert.Subject.CommonName != ctl.Endpoint.Name {
		return fmt.Errorf("wrong commonName %s is found, %s is expected", cert.Subject.CommonName, ctl.Endpoint.Name)
	}

	return ctl.CertManager.VerifyCert(cert, certutil.ExtKeyUsagesServerAndClient)
}

func (ctl *controller) restartConnectorPods() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var podList corev1.PodList
	if err := ctl.client.List(ctx, &podList, client.MatchingLabels(ctl.ConnectorLabels)); err != nil {
		ctl.log.Error(err, "failed to list connector pods")
		return
	}

	for _, pod := range podList.Items {
		if err := ctl.client.Delete(ctx, &pod); err != nil {
			ctl.log.Error(err, "failed to delete connector")
		}
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
			nameSet.Insert(name)
		}
	}
	nameSet.Delete(connectorName)

	endpoints := ctl.Store.GetEndpoints(nameSet.List()...)

	peers := make([]apis.Endpoint, 0, len(endpoints))
	peers = append(peers, endpoints...)

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
	nodeIPs, podCIDRs := nodeutil.GetInternalIPs(node), ctl.GetPodCIDRs(node)
	if len(nodeIPs) == 0 || len(podCIDRs) == 0 {
		ctl.log.V(5).Info("this node has no IPs or PodCIDRs, skip adding it", "nodeName", node.Name)
		return
	}

	ctl.mux.Lock()
	defer ctl.mux.Unlock()
	if ctl.nodeNameSet.Has(node.Name) {
		return
	}

	ctl.nodeNameSet.Insert(node.Name)
	ctl.nodeCache[node.Name] = Node{
		Name:     node.Name,
		IPs:      nodeIPs,
		PodCIDRs: podCIDRs,
	}

	if rebuild {
		ctl.rebuildConnectorEndpoint()
	}
}

func (ctl *controller) removeNode(nodeName string) {
	ctl.mux.Lock()
	defer ctl.mux.Unlock()

	if !ctl.nodeNameSet.Has(nodeName) {
		return
	}

	ctl.nodeNameSet.Delete(nodeName)
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
	for _, nodeName := range ctl.nodeNameSet.List() {
		node := ctl.nodeCache[nodeName]

		subnets = append(subnets, node.PodCIDRs...)
		nodeSubnets = append(nodeSubnets, node.IPs...)
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

func parseCertFromSecret(secret corev1.Secret) (*x509.Certificate, error) {
	certPEM := secretutil.GetCert(secret)
	block, _ := pem.Decode(certPEM)

	return x509.ParseCertificate(block.Bytes)
}
