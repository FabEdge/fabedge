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

package netconf

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

type VirtualServer struct {
	IP       string          `yaml:"ip,omitempty"`
	Port     int32           `yaml:"port,omitempty"`
	Protocol corev1.Protocol `yaml:"protocol,omitempty"`

	Scheduler           string                 `yaml:"scheduler,omitempty"`
	SessionAffinity     corev1.ServiceAffinity `yaml:"sessionAffinity,omitempty"`
	StickyMaxAgeSeconds int32                  `yaml:"stickyMaxAgeSeconds,omitempty"`

	RealServers RealServers `yaml:"realServers,omitempty"`
}

type RealServer struct {
	IP   string `yaml:"ip,omitempty"`
	Port int32  `yaml:"port,omitempty"`
}

func (s RealServer) String() string {
	return fmt.Sprintf("%s:%d", s.IP, s.Port)
}

type VirtualServers []VirtualServer

func (s VirtualServers) Len() int {
	return len(s)
}

func (s VirtualServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s VirtualServers) Less(i, j int) bool {
	if s[i].IP < s[j].IP {
		return true
	}

	if s[i].IP == s[j].IP {
		if s[i].Port < s[j].Port {
			return true
		}
	}

	return false
}

type RealServers []RealServer

func (s RealServers) Len() int {
	return len(s)
}

func (s RealServers) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s RealServers) Less(i, j int) bool {
	if s[i].IP < s[j].IP {
		return true
	}

	if s[i].IP == s[j].IP {
		if s[i].Port < s[j].Port {
			return true
		}
	}

	return false
}
