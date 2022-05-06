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
	"time"

	apis "github.com/fabedge/fabedge/pkg/apis/v1alpha1"
)

// CNINetConf describes a network.
type CNINetConf struct {
	CNIVersion string        `json:"cniVersion,omitempty"`
	Name       string        `json:"name,omitempty"`
	Plugins    []interface{} `json:"plugins"`
}

type IPAMConfig struct {
	Type   string     `json:"type"`
	Ranges []RangeSet `json:"ranges"`
}

type RangeSet []Range
type Range struct {
	Subnet string `json:"subnet"`
}

type BridgeConfig struct {
	Type             string     `json:"type"`
	Bridge           string     `json:"bridge"`
	IsGateway        bool       `json:"isGateway,omitempty"`
	IsDefaultGateway bool       `json:"isDefaultGateway,omitempty"`
	ForceAddress     bool       `json:"forceAddress,omitempty"`
	HairpinMode      bool       `json:"hairpinMode,omitempty"`
	MTU              int        `json:"mtu,omitempty"`
	IPAM             IPAMConfig `json:"ipam"`
}

type CapbilitiesConfig struct {
	Type         string          `json:"type"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
}

type Endpoint struct {
	apis.Endpoint

	// IsLocal mark an endpoint from LAN
	IsLocal bool

	// ExpireTime works only on local endpoint
	ExpireTime time.Time `json:"-"`
}

type Message struct {
	apis.Endpoint

	Token string
}
