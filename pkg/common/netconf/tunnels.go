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
	"io/ioutil"

	"gopkg.in/yaml.v2"
)


type TunnelEndpoint struct {
	ID              string   `yaml:"id,omitempty"`
	Name            string   `yaml:"name,omitempty"`
	// public addresses can be IP, DNS
	PublicAddresses []string `yaml:"publicAddresses,omitempty"`
	// pod subnets
	Subnets     []string `yaml:"subnets,omitempty"`
	// internal IPs of kubernetes node
	NodeSubnets []string `yaml:"nodeSubnets,omitempty"`
}

type NetworkConf struct {
	TunnelEndpoint `yaml:"-,inline"`
	Peers          []TunnelEndpoint `yaml:"peers,omitempty"`
}

func LoadNetworkConf(path string) (NetworkConf, error) {
	var conf NetworkConf

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return conf, err
	}

	return conf, yaml.Unmarshal(data, &conf)
}
