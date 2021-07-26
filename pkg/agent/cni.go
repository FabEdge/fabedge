// Copyright 2021 BoCloud
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

// CNINetConf describes a network.
type CNINetConf struct {
	CNIVersion string `json:"cniVersion,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`

	Bridge           string     `json:"bridge"`
	IsGateway        bool       `json:"isGateway"`
	IsDefaultGateway bool       `json:"isDefaultGateway"`
	ForceAddress     bool       `json:"forceAddress"`
	IPAM             IPAMConfig `json:"ipam"`
}

type IPAMConfig struct {
	Type   string     `json:"type"`
	Ranges []RangeSet `json:"ranges"`
}

type RangeSet []Range
type Range struct {
	Subnet string `json:"subnet"`
}
