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
	"reflect"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/jjeffery/stringset"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/fabedge/fabedge/pkg/operator/predicates"
)

const (
	LabelServiceName = "kubernetes.io/service-name"
	LabelHostname    = "kubernetes.io/hostname"
)

// type shortcuts
type (
	EndpointSlice = discoveryv1.EndpointSlice
	Result        = reconcile.Result
	ObjectKey     = client.ObjectKey
)

type Config struct {
	Manager manager.Manager
	// the namespace where agent and configmap are created
	AgentNamespace string

	IPVSScheduler string

	// the interval to check if agent load balance rules is consistent with configmap
	CheckInterval time.Duration
}

// proxy keep proxy rules configmap for each service which has edge endpoints.
// An edge-endpoint is an endpoint which has corresponding pod on a edge node.
type proxy struct {
	mu               sync.Mutex
	serviceMap       ServiceMap
	endpointSliceMap EndpointSliceMap
	nodeSet          EdgeNodeSet
	// the namespace where agent and configmap are created
	namespace string
	keeper    *loadBalanceConfigKeeper

	checkInterval time.Duration

	client client.Client
	log    logr.Logger
}

func AddToManager(cnf Config) error {
	mgr := cnf.Manager

	keeper := &loadBalanceConfigKeeper{
		namespace:     cnf.AgentNamespace,
		interval:      5 * time.Second,
		nodeSet:       make(EdgeNodeSet),
		ipvsScheduler: cnf.IPVSScheduler,

		client: mgr.GetClient(),
		log:    mgr.GetLogger().WithName("load-balance-keeper"),
	}

	proxy := &proxy{
		serviceMap:       make(ServiceMap),
		endpointSliceMap: make(EndpointSliceMap),
		nodeSet:          make(EdgeNodeSet),
		keeper:           keeper,
		checkInterval:    cnf.CheckInterval,

		log:    mgr.GetLogger().WithName("fab-proxy"),
		client: mgr.GetClient(),
	}

	if err := mgr.Add(manager.RunnableFunc(keeper.Start)); err != nil {
		return err
	}

	if err := mgr.Add(manager.RunnableFunc(proxy.startCheckLoadBalanceRules)); err != nil {
		return err
	}

	err := addController(
		"proxy-endpointslice",
		mgr,
		proxy.OnEndpointSliceUpdate,
		&EndpointSlice{},
	)
	if err != nil {
		return err
	}

	err = addController(
		"proxy-node",
		mgr,
		proxy.onNodeUpdate,
		&corev1.Node{},
		predicates.EdgeNodePredicate(),
	)
	if err != nil {
		return err
	}

	return addController("proxy-service",
		mgr,
		proxy.OnServiceUpdate,
		&corev1.Service{},
	)
}

func addController(name string, mgr manager.Manager, reconciler reconcile.Func, watchObj client.Object, predicates ...predicate.Predicate) error {
	c, err := controller.New(
		name,
		mgr,
		controller.Options{
			Reconciler: reconciler,
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		&source.Kind{Type: watchObj},
		&handler.EnqueueRequestForObject{},
		predicates...,
	)
}

func (p *proxy) onNodeUpdate(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := p.log.WithValues("request", request)

	var node corev1.Node
	if err := p.client.Get(ctx, request.NamespacedName, &node); err != nil {
		if errors.IsNotFound(err) {
			p.removeNode(request.Name)
			return Result{}, nil
		}

		log.Error(err, "failed to get node")
		return Result{}, err
	}

	if node.DeletionTimestamp != nil {
		p.removeNode(request.Name)
		return Result{}, nil
	}

	p.addNode(request.Name)

	return Result{}, nil
}

func (p *proxy) addNode(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.nodeSet[name]; !exists {
		p.nodeSet[name] = newEdgeNode(name)
	}
}

func (p *proxy) removeNode(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.nodeSet, name)
}

func (p *proxy) OnServiceUpdate(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := p.log.WithValues("request", request)

	var service corev1.Service
	if err := p.client.Get(ctx, request.NamespacedName, &service); err != nil {
		log.Error(err, "failed to get service")

		if errors.IsNotFound(err) {
			log.Info("service is deleted, cleanup service and endpoints")
			p.cleanupService(request.NamespacedName)
			return Result{}, nil
		}
		return Result{}, err
	}

	// if service is updated to a invalid service, we take it as deleted and cleanup related resources
	if p.shouldSkipService(&service) {
		log.V(5).Info("service has no ClusterIP, skip it")

		p.cleanupService(request.NamespacedName)
		return Result{}, nil
	}

	changed := p.syncServiceInfoFromService(request.NamespacedName, &service)
	if changed {
		p.syncServiceChangesToAgentByKey(request.NamespacedName)
	}

	return Result{}, nil
}

// syncServiceInfoFromService only sync clusterIP, sessionAffinity and StickyMaxAgeSeconds as needed
// if these are the same, just skip synchronizing
func (p *proxy) syncServiceInfoFromService(key ObjectKey, svc *corev1.Service) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	oldService := p.serviceMap[key]
	newService := makeServiceInfo(svc)

	if oldService.ClusterIP == newService.ClusterIP &&
		oldService.SessionAffinity == newService.SessionAffinity &&
		oldService.StickyMaxAgeSeconds == newService.StickyMaxAgeSeconds {
		return false
	}

	oldService.ClusterIP = newService.ClusterIP
	oldService.SessionAffinity = newService.SessionAffinity
	oldService.StickyMaxAgeSeconds = newService.StickyMaxAgeSeconds

	if oldService.EndpointMap == nil {
		oldService.EndpointMap = make(map[Port]EndpointSet)
	}
	if oldService.EndpointToNodes == nil {
		oldService.EndpointToNodes = make(map[Endpoint]NodeName)
	}

	p.serviceMap[key] = oldService

	return true
}

func (p *proxy) cleanupService(serviceKey ObjectKey) {
	p.mu.Lock()
	defer p.mu.Unlock()

	serviceInfo, exists := p.serviceMap[serviceKey]
	if !exists {
		return
	}

	// cleanup endpoints in related edge node
	for port := range serviceInfo.EndpointMap {
		spn := ServicePortName{NamespacedName: serviceKey, Port: port.Port, Protocol: port.Protocol}
		for _, nodeName := range serviceInfo.EndpointToNodes {
			node, exists := p.nodeSet[nodeName]
			if !exists {
				continue
			}
			delete(node.ServicePortMap, spn)
			delete(node.EndpointMap, spn)
			p.nodeSet[nodeName] = node
		}
	}

	p.syncServiceChangesToAgent(serviceInfo)
	delete(p.serviceMap, serviceKey)
}

func (p *proxy) syncServiceChangesToAgentByKey(key ObjectKey) {
	p.mu.Lock()
	defer p.mu.Unlock()

	serviceInfo, ok := p.serviceMap[key]
	if !ok {
		return
	}

	p.syncServiceChangesToAgent(serviceInfo)
}

func (p *proxy) syncServiceChangesToAgent(serviceInfo ServiceInfo) {
	for _, name := range serviceInfo.EndpointToNodes {
		node, ok := p.nodeSet[name]
		if !ok {
			continue
		}
		p.keeper.AddNode(node)
	}
}

func (p *proxy) OnEndpointSliceUpdate(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := p.log.WithValues("request", request)

	var es EndpointSlice
	err := p.client.Get(ctx, request.NamespacedName, &es)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("endpointslice is deleted, cleanup related endpoints")

			p.cleanupEndpointsOfEndpointSlice(request.NamespacedName)
			return reconcile.Result{}, nil
		}

		log.Error(err, "failed to get endpointslice")
		return reconcile.Result{}, err
	}

	if es.DeletionTimestamp != nil {
		log.Info("endpointslice is terminating, cleanup related endpoints")
		p.cleanupEndpointsOfEndpointSlice(request.NamespacedName)
		return reconcile.Result{}, nil
	}

	serviceName := getServiceName(es.Labels)
	if serviceName == "" {
		log.Info("no service name found in endpointslice", "endpointslice", es)
		return Result{}, nil
	}

	var (
		service    corev1.Service
		serviceKey = ObjectKey{Name: serviceName, Namespace: request.Namespace}
	)
	if err = p.client.Get(ctx, serviceKey, &service); err != nil {
		log.Error(err, "failed to get service")

		// if service is not found, we don't handle this endpointslice
		if errors.IsNotFound(err) {
			log.Info("Corresponding service is not found, cleanup service and endpoints", "serviceKey", serviceKey)
			p.cleanupService(serviceKey)
			return Result{}, nil
		}

		return Result{}, err
	}

	if p.shouldSkipService(&service) {
		log.V(5).Info("service has no ClusterIP, skip it")
		p.cleanupService(serviceKey)
		return Result{}, nil
	}

	serviceChanged := p.syncServiceInfoFromService(serviceKey, &service)
	p.syncServiceEndpointsFromEndpointSlice(p.makeEndpointSliceInfo(&es), serviceChanged)

	return Result{}, nil
}

func (p *proxy) syncServiceEndpointsFromEndpointSlice(newES EndpointSliceInfo, serviceChanged bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key, serviceKey := newES.ObjectKey, newES.ServiceKey
	oldES := p.endpointSliceMap[key]

	isSame := reflect.DeepEqual(oldES, newES)
	if isSame {
		return
	}

	serviceInfo := p.serviceMap[serviceKey]
	// collect node which has endpoints changes
	changedNodeNames := stringset.New()

	// add new endpoints
	for port := range newES.Ports {
		servicePortName := ServicePortName{
			NamespacedName: serviceKey,
			Port:           port.Port,
			Protocol:       port.Protocol,
		}

		endpointSet := serviceInfo.EndpointMap[port]
		for _, ep := range newES.Endpoints {
			endpoint := Endpoint{
				IP:   ep.IP,
				Port: port.Port,
			}
			endpointSet.Add(endpoint)
			serviceInfo.EndpointToNodes[endpoint] = ep.NodeName

			p.addServicePortToNode(ep.NodeName, servicePortName, ServicePort{
				ClusterIP:           serviceInfo.ClusterIP,
				Port:                port.Port,
				Protocol:            port.Protocol,
				SessionAffinity:     serviceInfo.SessionAffinity,
				StickyMaxAgeSeconds: serviceInfo.StickyMaxAgeSeconds,
			})

			added := p.addEndpointToNode(ep.NodeName, servicePortName, endpoint)
			if serviceChanged || added {
				changedNodeNames.Add(ep.NodeName)
			}
		}
		serviceInfo.EndpointMap[port] = endpointSet
	}

	// remove old endpoints
	for port := range oldES.Ports {
		_, exists := newES.Ports[port]
		portRemoved := !exists
		if portRemoved {
			p.log.V(4).Info("port is remove", "port", port, "service", serviceKey)
		}

		servicePortName := ServicePortName{
			NamespacedName: serviceKey,
			Port:           port.Port,
			Protocol:       port.Protocol,
		}

		endpointSet := serviceInfo.EndpointMap[port]
		for _, ep := range oldES.Endpoints {
			_, exist := newES.Endpoints[ep.IP]
			endpointRemoved := !exist

			if portRemoved || endpointRemoved {
				endpoint := Endpoint{
					IP:   ep.IP,
					Port: port.Port,
				}
				endpointSet.Remove(endpoint)

				delete(serviceInfo.EndpointToNodes, endpoint)
				p.removeEndpointFromNode(ep.NodeName, servicePortName, endpoint)

				changedNodeNames.Add(ep.NodeName)
			}

			if portRemoved {
				p.removeServicePortFromNode(ep.NodeName, servicePortName)
			}
		}

		if len(endpointSet) == 0 {
			delete(serviceInfo.EndpointMap, port)
		} else {
			serviceInfo.EndpointMap[port] = endpointSet
		}
	}

	p.endpointSliceMap[key] = newES
	p.serviceMap[serviceKey] = serviceInfo

	for nodeName := range changedNodeNames {
		node, ok := p.nodeSet[nodeName]
		if !ok {
			continue
		}
		p.keeper.AddNode(node)
	}
}

func (p *proxy) cleanupEndpointsOfEndpointSlice(key ObjectKey) {
	es, ok := p.getEndpointSliceInfo(key)
	if !ok {
		return
	}

	// no matter what caused cleanup, we take current endpointslice which
	// has empty ports and endpoints as deleted,
	es.Ports = make(map[Port]Empty)
	es.Endpoints = make(map[string]EndpointInfo)

	p.syncServiceEndpointsFromEndpointSlice(es, false)

	p.mu.Lock()
	delete(p.endpointSliceMap, key)
	p.mu.Unlock()
}

func (p *proxy) startCheckLoadBalanceRules(ctx context.Context) error {
	tick := time.NewTicker(p.checkInterval)

	for {
		select {
		case <-tick.C:
			for _, node := range p.nodeSet {
				p.keeper.AddNodeIfNotPresent(node)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *proxy) addEndpointToNode(nodeName string, spn ServicePortName, endpoint Endpoint) bool {
	node, ok := p.nodeSet[nodeName]
	if !ok {
		node = newEdgeNode(nodeName)
	}

	endpointSet := node.EndpointMap[spn]
	if endpointSet.Contains(endpoint) {
		return false
	}

	endpointSet.Add(endpoint)

	node.EndpointMap[spn] = endpointSet
	p.nodeSet[nodeName] = node

	return true
}

func (p *proxy) removeEndpointFromNode(nodeName string, spn ServicePortName, endpoint Endpoint) {
	node, ok := p.nodeSet[nodeName]
	if !ok {
		return
	}

	eps := node.EndpointMap[spn]
	eps.Remove(endpoint)
	if len(eps) == 0 {
		delete(node.EndpointMap, spn)
		delete(node.ServicePortMap, spn)
	}

	p.nodeSet[nodeName] = node
}

func (p *proxy) addServicePortToNode(nodeName string, spn ServicePortName, servicePort ServicePort) {
	node, ok := p.nodeSet[nodeName]
	if !ok {
		node = newEdgeNode(nodeName)
	}

	node.ServicePortMap[spn] = servicePort
	p.nodeSet[nodeName] = node
}

func (p *proxy) removeServicePortFromNode(nodeName string, spn ServicePortName) {
	node, ok := p.nodeSet[nodeName]
	if !ok {
		return
	}

	delete(node.ServicePortMap, spn)

	p.nodeSet[nodeName] = node
}

func (p *proxy) makeEndpointSliceInfo(es *EndpointSlice) EndpointSliceInfo {
	info := EndpointSliceInfo{
		ObjectKey: ObjectKey{
			Name:      es.Name,
			Namespace: es.Namespace,
		},
		ServiceKey: ObjectKey{
			Name:      getServiceName(es.Labels),
			Namespace: es.Namespace,
		},
		Ports:     make(map[Port]Empty),
		Endpoints: make(map[string]EndpointInfo),
	}

	for _, port := range es.Ports {
		p := Port{
			Port:     *port.Port,
			Protocol: *port.Protocol,
		}
		info.Ports[p] = Empty{}
	}

	for _, ep := range es.Endpoints {
		nodeName := getHostname(&ep)

		if _, exists := p.nodeSet[nodeName]; !exists {
			continue
		}

		// 在边缘场景, endpoint的稳定性有些问题, 会导致conditions.Ready的状态反复变化
		// 暂时原因不明，所以我们不考虑这个问题
		// todo: 处理网络抖动导致的endpoint不稳定情况
		info.Endpoints[ep.Addresses[0]] = EndpointInfo{
			IP:       ep.Addresses[0],
			NodeName: nodeName,
		}
	}

	return info
}

func (p *proxy) getEndpointSliceInfo(key ObjectKey) (EndpointSliceInfo, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	es, ok := p.endpointSliceMap[key]
	return es, ok
}

func (p *proxy) shouldSkipService(svc *corev1.Service) bool {
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		return true
	}

	if svc.Spec.ClusterIP == corev1.ClusterIPNone || svc.Spec.ClusterIP == "" {
		return true
	}

	if svc.Spec.Selector == nil || len(svc.Spec.Selector) == 0 {
		return true
	}

	return false
}

func makeServiceInfo(svc *corev1.Service) ServiceInfo {
	var stickyMaxAgeSeconds int32
	if svc.Spec.SessionAffinity == corev1.ServiceAffinityClientIP {
		// Kube-apiserver side guarantees SessionAffinityConfig won't be nil when session affinity type is ClientIP
		stickyMaxAgeSeconds = *svc.Spec.SessionAffinityConfig.ClientIP.TimeoutSeconds
	}

	return ServiceInfo{
		ClusterIP:           svc.Spec.ClusterIP,
		SessionAffinity:     svc.Spec.SessionAffinity,
		StickyMaxAgeSeconds: stickyMaxAgeSeconds,
	}
}

func getValueByKey(data map[string]string, key string) string {
	if data == nil {
		return ""
	}

	return data[key]
}

func getServiceName(data map[string]string) string {
	return getValueByKey(data, LabelServiceName)
}

func getHostname(endpoint *discoveryv1.Endpoint) string {
	if endpoint.NodeName != nil && *endpoint.NodeName != "" {
		return *endpoint.NodeName
	}

	return getValueByKey(endpoint.Topology, LabelHostname)
}

func newEdgeNode(name string) EdgeNode {
	return EdgeNode{
		Name:           name,
		ServicePortMap: make(map[ServicePortName]ServicePort),
		EndpointMap:    make(map[ServicePortName]EndpointSet),
	}
}
