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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fabedge/fabedge/pkg/common/netconf"
)

// Endpoint format: IP:PORT
type Endpoint = netconf.RealServer
type NodeName = string
type ServiceMap map[client.ObjectKey]ServiceInfo
type EndpointSliceMap map[client.ObjectKey]EndpointSliceInfo
type Empty struct{}
type EndpointByIP map[string]EndpointInfo
type PortSet map[Port]Empty
type EdgeNodeSet map[NodeName]EdgeNode

type ServiceInfo struct {
	ClusterIP           string
	SessionAffinity     corev1.ServiceAffinity
	StickyMaxAgeSeconds int32

	EndpointMap     map[Port]EndpointSet
	EndpointToNodes map[Endpoint]NodeName
}

type Port struct {
	Protocol corev1.Protocol
	Port     int32
}

func (p Port) String() string {
	return fmt.Sprintf("%d:%s", p.Port, p.Protocol)
}

type ServicePort struct {
	ClusterIP           string
	Port                int32
	Protocol            corev1.Protocol
	StickyMaxAgeSeconds int32
	SessionAffinity     corev1.ServiceAffinity
}

func (s ServicePort) String() string {
	return fmt.Sprintf("%s:%d", s.ClusterIP, s.Port)
}

type ServicePortName struct {
	types.NamespacedName
	Port     int32
	Protocol corev1.Protocol
}

func (s ServicePortName) String() string {
	return fmt.Sprintf("%s:%d", s.NamespacedName, s.Port)
}

type EndpointInfo struct {
	IP       string
	NodeName string
}

type EndpointSliceInfo struct {
	ObjectKey
	ServiceKey ObjectKey
	Ports      PortSet
	Endpoints  EndpointByIP
}

type EdgeNode struct {
	Name           string
	ServicePortMap map[ServicePortName]ServicePort
	EndpointMap    map[ServicePortName]EndpointSet
}

type EndpointSet map[Endpoint]Empty

func (set *EndpointSet) Add(v ...Endpoint) EndpointSet {
	if *set == nil {
		*set = make(EndpointSet)
	}
	for _, s := range v {
		(*set)[s] = struct{}{}
	}
	return *set
}

func (set EndpointSet) Remove(v ...Endpoint) EndpointSet {
	if set != nil {
		for _, s := range v {
			delete(set, s)
		}
	}
	return set
}

func (set EndpointSet) Len() int {
	return len(set)
}

func (set EndpointSet) Contains(s Endpoint) bool {
	_, ok := set[s]
	return ok
}

func (set EndpointSet) Equal(other EndpointSet) bool {
	if len(set) != len(other) {
		return false
	}
	for s := range set {
		if _, ok := other[s]; !ok {
			return false
		}
	}
	return true
}
