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

package tunnel

import "github.com/fabedge/fabedge/pkg/common/netconf"

type Manager interface {
	ListConnNames() ([]string, error)
	LoadConn(conn ConnConfig) error
	InitiateConn(name string) error
	UnloadConn(name string) error
	IsActive() (bool, error)
}

type ConnConfig struct {
	Name string // must be unique

	LocalID          string
	LocalAddress     []string
	LocalSubnets     []string
	LocalNodeSubnets []string
	LocalCerts       []string
	LocalType        netconf.EndpointType

	RemoteID          string
	RemoteAddress     []string
	RemoteSubnets     []string
	RemoteNodeSubnets []string
	RemoteType        netconf.EndpointType
}
