package agent

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"time"

	"github.com/fabedge/fabedge/pkg/common/netconf"
)

const filenameLocalEndpoints = "local-endpoints.json"

func (m *Manager) loadNetworkConf() error {
	conf, err := netconf.LoadNetworkConf(m.TunnelsConfPath)
	if err != nil {
		return err
	}

	m.lock.Lock()
	defer m.lock.Unlock()
	m.currentEndpoint = Endpoint{
		Endpoint: conf.Endpoint,
	}

	for _, peer := range conf.Peers {
		// local endpoints has higher priority than tunnel endpoint
		if old := m.peerEndpoints[peer.Name]; old.IsLocal {
			continue
		}

		m.peerEndpoints[peer.Name] = Endpoint{
			Endpoint: peer,
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

	m.lock.Lock()
	defer m.lock.Unlock()

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

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, endpoint := range endpoints {
		endpoint.ExpireTime = time.Now().Add(m.EndpointTTL)
		endpoint.IsLocal = true
		m.peerEndpoints[endpoint.Name] = endpoint
	}
}

func (m *Manager) getCurrentEndpoint() Endpoint {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.currentEndpoint
}

func (m *Manager) getPeerEndpoints() []Endpoint {
	m.lock.RLock()
	m.lock.RUnlock()

	peers := make([]Endpoint, 0, len(m.peerEndpoints))
	for _, e := range m.peerEndpoints {
		peers = append(peers, e)
	}

	return peers
}
