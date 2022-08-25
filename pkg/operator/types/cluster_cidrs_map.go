package types

import "sync"

type ClusterCIDRsMap struct {
	lock        sync.RWMutex
	cidrsByName map[string][]string

	readonlyCopy map[string][]string
}

func NewClusterCIDRsMap() *ClusterCIDRsMap {
	return &ClusterCIDRsMap{
		cidrsByName: make(map[string][]string),
	}
}

func (m *ClusterCIDRsMap) Set(name string, cidrs []string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.cidrsByName[name] = cidrs
	m.readonlyCopy = nil
}

func (m *ClusterCIDRsMap) Get(name string) ([]string, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	cidrs, found := m.cidrsByName[name]
	return cidrs, found
}

func (m *ClusterCIDRsMap) Delete(name string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, found := m.cidrsByName[name]; !found {
		return
	}

	m.readonlyCopy = nil
	delete(m.cidrsByName, name)
}

// GetCopy return a copy of inner data, the returned data should not be changed
func (m *ClusterCIDRsMap) GetCopy() map[string][]string {
	m.lock.RLock()
	cp := m.readonlyCopy
	m.lock.RUnlock()

	if cp != nil {
		return cp
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	if m.readonlyCopy != nil {
		return m.readonlyCopy
	}

	cp = make(map[string][]string)
	for key, value := range m.cidrsByName {
		cp[key] = value
	}
	m.readonlyCopy = cp

	return cp
}
