package types

import "sync"

type PodCIDRStore interface {
	Append(nodeName string, cidr ...string)
	Remove(nodeName string, cidr ...string)
	RemoveAll(nodeName string)
	RemoveByPodCIDR(podCIDR string)
	Get(nodeName string) []string
	GetNodeNameByPodCIDR(cidr string) (string, bool)
}

var _ PodCIDRStore = &podCIDRStore{}

type podCIDRStore struct {
	// key is nodeName, value is pod CIDR list
	nodeToPodCIDRs map[string][]string
	podCIDRToNode  map[string]string
	mux            sync.RWMutex
}

func NewPodCIDRStore() PodCIDRStore {
	return &podCIDRStore{
		nodeToPodCIDRs: make(map[string][]string),
		podCIDRToNode:  make(map[string]string),
	}
}

func (s *podCIDRStore) Append(nodeName string, podCIDRs ...string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	nodePodCIDRs := s.nodeToPodCIDRs[nodeName]
	for _, cidr := range podCIDRs {
		if !findPodCIDR(cidr, nodePodCIDRs) {
			nodePodCIDRs = append(nodePodCIDRs, cidr)
			s.podCIDRToNode[cidr] = nodeName
		}
	}

	s.nodeToPodCIDRs[nodeName] = nodePodCIDRs
}

func (s *podCIDRStore) Remove(nodeName string, podCIDRs ...string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	nodePodCIDRs := s.nodeToPodCIDRs[nodeName]
	for _, value := range podCIDRs {
		nodePodCIDRs = deletePodCIDR(value, nodePodCIDRs)
		delete(s.podCIDRToNode, value)
	}

	if len(nodePodCIDRs) > 0 {
		s.nodeToPodCIDRs[nodeName] = nodePodCIDRs
	} else {
		delete(s.nodeToPodCIDRs, nodeName)
	}
}

func (s *podCIDRStore) RemoveByPodCIDR(podCIDR string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	nodeName := s.podCIDRToNode[podCIDR]

	s.nodeToPodCIDRs[nodeName] = deletePodCIDR(podCIDR, s.nodeToPodCIDRs[nodeName])
	delete(s.podCIDRToNode, podCIDR)
}

func (s *podCIDRStore) RemoveAll(nodeName string) {
	s.mux.Lock()
	defer s.mux.Unlock()

	for _, cidr := range s.nodeToPodCIDRs[nodeName] {
		delete(s.podCIDRToNode, cidr)
	}
	delete(s.nodeToPodCIDRs, nodeName)
}

func (s *podCIDRStore) Get(nodeName string) []string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.nodeToPodCIDRs[nodeName]
}

func (s *podCIDRStore) GetNodeNameByPodCIDR(cidr string) (string, bool) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	nodeName, ok := s.podCIDRToNode[cidr]

	return nodeName, ok
}

func findPodCIDR(value string, cidrs []string) bool {
	for _, cidr := range cidrs {
		if cidr == value {
			return true
		}
	}

	return false
}

func deletePodCIDR(value string, cidrs []string) []string {
	for i, cidr := range cidrs {
		if cidr == value {
			return append(cidrs[0:i], cidrs[i+1:]...)
		}
	}

	return cidrs
}
