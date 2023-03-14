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
	"encoding/json"
	"io/ioutil"
	"path"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/common/netconf"
)

const filenameLocalEndpoints = "local-endpoints.json"

func (m *Manager) loadNetworkConf() error {
	conf, err := netconf.LoadNetworkConf(m.TunnelsConfPath)
	if err != nil {
		return err
	}

	m.endpointLock.Lock()
	defer m.endpointLock.Unlock()
	m.currentEndpoint = Endpoint{
		Endpoint: conf.Endpoint,
	}

	if conf.Mediator != nil {
		m.mediatorEndpoint = &Endpoint{
			Endpoint: *conf.Mediator,
		}
	}

	nameSet := sets.NewString()
	for _, peer := range conf.Peers {
		// local endpoints has higher priority than tunnel endpoint
		if old := m.peerEndpoints[peer.Name]; old.IsLocal {
			continue
		}

		m.peerEndpoints[peer.Name] = Endpoint{
			Endpoint: peer,
		}
		nameSet.Insert(peer.Name)
	}

	for name, peer := range m.peerEndpoints {
		if peer.IsLocal {
			continue
		}

		if !nameSet.Has(name) {
			delete(m.peerEndpoints, name)
		}
	}

	return nil
}

func (m *Manager) backupLoop() {
	for {
		time.Sleep(m.BackupInterval)
		m.saveLocalEndpoints()
	}
}

func (m *Manager) cleanExpiredEndpoints() {
	if !m.EnableAutoNetworking {
		return
	}

	m.endpointLock.Lock()
	defer m.endpointLock.Unlock()

	now := time.Now()

	var names []string
	for _, ep := range m.peerEndpoints {
		if ep.IsLocal && ep.ExpireTime.Before(now) {
			names = append(names, ep.Name)
		}
	}

	for _, name := range names {
		delete(m.peerEndpoints, name)
	}
}

func (m *Manager) saveLocalEndpoints() {
	peerEndpoints := m.getPeerEndpoints()
	endpoints := make([]Endpoint, 0, len(peerEndpoints))

	now := time.Now()
	for _, peer := range peerEndpoints {
		if peer.IsLocal && peer.ExpireTime.After(now) {
			endpoints = append(endpoints, peer)
		}
	}

	data, err := json.Marshal(&endpoints)
	if err != nil {
		m.log.Error(err, "failed to marshal local endpoints")
		return
	}

	filename := path.Join(m.Workdir, filenameLocalEndpoints)
	if err = ioutil.WriteFile(filename, data, 0644); err != nil {
		m.log.Error(err, "failed to save local endpoints")
		return
	}
}

// loadLocalEndpoints will try its best to load local endpoints
// but if any error happens, it will just give up and won't return any error
func (m *Manager) loadLocalEndpoints() {
	filename := path.Join(m.Workdir, filenameLocalEndpoints)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		m.log.Error(err, "failed to load local endpoints")
		return
	}

	var endpoints []Endpoint
	if err = json.Unmarshal(data, &endpoints); err != nil {
		m.log.Error(err, "failed to unmarshal local endpoints data")
		return
	}

	m.endpointLock.Lock()
	defer m.endpointLock.Unlock()

	for _, endpoint := range endpoints {
		endpoint.ExpireTime = time.Now().Add(m.EndpointTTL)
		endpoint.IsLocal = true
		m.peerEndpoints[endpoint.Name] = endpoint
	}
}

func (m *Manager) getCurrentEndpoint() Endpoint {
	m.endpointLock.RLock()
	defer m.endpointLock.RUnlock()

	return m.currentEndpoint
}

func (m *Manager) getMediatorEndpoint() *Endpoint {
	m.endpointLock.RLock()
	defer m.endpointLock.RUnlock()

	return m.mediatorEndpoint
}

func (m *Manager) getPeerEndpoints() []Endpoint {
	m.endpointLock.RLock()
	m.endpointLock.RUnlock()

	peers := make([]Endpoint, 0, len(m.peerEndpoints))
	for _, e := range m.peerEndpoints {
		peers = append(peers, e)
	}

	return peers
}
