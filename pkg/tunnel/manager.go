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

import (
	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
)

type Manager interface {
	IsRunning() bool
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
	LocalType        apis.EndpointType

	RemoteID          string
	RemoteAddress     []string
	RemoteSubnets     []string
	RemoteNodeSubnets []string
	RemoteType        apis.EndpointType
	RemotePort        *uint

	// Whether this connection is used for mediation
	Mediation bool

	// whether is connection need mediation
	NeedMediation bool
	// check https://docs.strongswan.org/docs/5.9/swanctl/swanctlConf.html
	// for detailed explanation for MediatedBy and MediationPeer
	MediatedBy    string
	MediationPeer string
}
