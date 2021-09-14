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

package store

import (
	"sync"

	"github.com/fabedge/fabedge/pkg/operator/types"
	"github.com/jjeffery/stringset"
)

type Interface interface {
	SaveEndpoint(ep types.Endpoint)
	GetEndpoint(name string) (types.Endpoint, bool)
	GetEndpoints(names ...string) []types.Endpoint
	GetAllEndpointNames() stringset.Set
	DeleteEndpoint(name string)

	SaveCommunity(ep types.Community)
	GetCommunity(name string) (types.Community, bool)
	GetCommunitiesByEndpoint(name string) []types.Community
	DeleteCommunity(name string)
}

var _ Interface = &store{}

type store struct {
	endpoints             map[string]types.Endpoint
	communities           map[string]types.Community
	endpointToCommunities map[string]stringset.Set

	mux sync.RWMutex
}

func NewStore() Interface {
	return &store{
		endpoints:             make(map[string]types.Endpoint),
		communities:           make(map[string]types.Community),
		endpointToCommunities: make(map[string]stringset.Set),
	}
}

func (s *store) SaveEndpoint(ep types.Endpoint) {
	s.mux.Lock()
	defer s.mux.Unlock()

	s.endpoints[ep.Name] = ep
}

func (s *store) GetEndpoint(name string) (types.Endpoint, bool) {
	s.mux.Lock()
	defer s.mux.Unlock()

	ep, ok := s.endpoints[name]
	return ep, ok
}

func (s *store) GetEndpoints(names ...string) []types.Endpoint {
	s.mux.Lock()
	defer s.mux.Unlock()

	endpoints := make([]types.Endpoint, 0, len(names))
	for _, name := range names {
		ep, ok := s.endpoints[name]
		if !ok {
			continue
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints
}

func (s *store) GetAllEndpointNames() stringset.Set {
	s.mux.Lock()
	defer s.mux.Unlock()

	names := make(stringset.Set, len(s.endpoints))
	for name := range s.endpoints {
		names.Add(name)
	}

	return names
}

func (s *store) DeleteEndpoint(name string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	delete(s.endpoints, name)
}

func (s *store) SaveCommunity(c types.Community) {
	s.mux.Lock()
	defer s.mux.Unlock()

	oldCommunity := s.communities[c.Name]
	if oldCommunity.Members.Equal(c.Members) {
		return
	}

	s.communities[c.Name] = c

	// add new member to communities index
	for member := range c.Members {
		cs := s.endpointToCommunities[member]
		cs.Add(c.Name)

		s.endpointToCommunities[member] = cs
	}

	// remove old member to communities index
	for member := range oldCommunity.Members {
		if c.Members.Contains(member) {
			continue
		}

		cs := s.endpointToCommunities[member]
		cs.Remove(c.Name)
		if len(cs) == 0 {
			delete(s.endpointToCommunities, member)
		}
	}
}

func (s *store) GetCommunity(name string) (types.Community, bool) {
	s.mux.Lock()
	defer s.mux.Unlock()

	c, ok := s.communities[name]
	return c, ok
}

func (s *store) GetCommunitiesByEndpoint(name string) []types.Community {
	s.mux.Lock()
	defer s.mux.Unlock()

	var communities []types.Community

	cs, ok := s.endpointToCommunities[name]
	if !ok {
		return communities
	}

	for communityName := range cs {
		cmm, ok := s.communities[communityName]
		if ok {
			communities = append(communities, cmm)
		}
	}

	return communities
}

func (s *store) DeleteCommunity(name string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	// remove this community from endpointToCommunity
	cmm := s.communities[name]
	for member := range cmm.Members {
		cs := s.endpointToCommunities[member]
		cs.Remove(name)
		if len(cs) == 0 {
			delete(s.endpointToCommunities, member)
		}
	}

	delete(s.communities, name)
}
