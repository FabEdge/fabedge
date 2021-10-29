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
	"net"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fabedge/fabedge/pkg/common/constants"
	"github.com/fabedge/fabedge/pkg/operator/allocator"
	storepkg "github.com/fabedge/fabedge/pkg/operator/store"
	"github.com/fabedge/fabedge/pkg/operator/types"
)

var _ Handler = &allocatablePodCIDRsHandler{}

type allocatablePodCIDRsHandler struct {
	client          client.Client
	allocator       allocator.Interface
	store           storepkg.Interface
	newEndpoint     types.NewEndpointFunc
	getEndpointName types.GetNameFunc
	log             logr.Logger
}

func (handler *allocatablePodCIDRsHandler) Do(ctx context.Context, node corev1.Node) error {
	currentEndpoint := handler.newEndpoint(node)

	if !handler.isValidSubnets(currentEndpoint.Subnets) {
		if err := handler.allocateSubnet(ctx, node); err != nil {
			return err
		}
	} else {
		handler.store.SaveEndpointAsLocal(currentEndpoint)
	}

	return nil
}

func (handler *allocatablePodCIDRsHandler) isValidSubnets(cidrs []string) bool {
	if len(cidrs) == 0 {
		return false
	}

	for _, cidr := range cidrs {
		_, subnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return false
		}

		if !handler.allocator.Contains(*subnet) {
			return false
		}
	}

	return true
}

func (handler *allocatablePodCIDRsHandler) allocateSubnet(ctx context.Context, node corev1.Node) error {
	log := handler.log.WithValues("nodeName", node.Name)

	log.V(5).Info("this node need subnet allocation")
	subnet, err := handler.allocator.GetFreeSubnetBlock(node.Name)
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
	node.Annotations[constants.KeyPodSubnets] = subnet.String()

	err = handler.client.Update(ctx, &node)
	if err != nil {
		log.Error(err, "failed to record node subnet allocation")

		handler.allocator.Reclaim(*subnet)
		log.V(5).Info("subnet is reclaimed")
		return err
	}

	handler.store.SaveEndpointAsLocal(handler.newEndpoint(node))
	return nil
}

func (handler *allocatablePodCIDRsHandler) Undo(ctx context.Context, nodeName string) error {
	log := handler.log.WithValues("nodeName", nodeName)

	epName := handler.getEndpointName(nodeName)
	ep, ok := handler.store.GetEndpoint(epName)
	if !ok {
		return nil
	}

	handler.store.DeleteEndpoint(ep.Name)
	log.V(5).Info("endpoint is delete from store", "endpoint", ep)

	for _, sn := range ep.Subnets {
		_, subnet, err := net.ParseCIDR(sn)
		if err != nil {
			log.Error(err, "invalid subnet, skip reclaiming subnets")
			continue
		}
		handler.allocator.Reclaim(*subnet)
		log.V(5).Info("subnet is reclaimed", "subnet", subnet)
	}

	return nil
}

var _ Handler = &rawPodCIDRsHandler{}

type rawPodCIDRsHandler struct {
	store           storepkg.Interface
	getEndpointName types.GetNameFunc
	newEndpoint     types.NewEndpointFunc
}

func (handler *rawPodCIDRsHandler) Do(ctx context.Context, node corev1.Node) error {
	endpoint := handler.newEndpoint(node)
	handler.store.SaveEndpointAsLocal(endpoint)
	return nil
}

func (handler *rawPodCIDRsHandler) Undo(ctx context.Context, nodeName string) error {
	epName := handler.getEndpointName(nodeName)
	handler.store.DeleteEndpoint(epName)
	return nil
}
