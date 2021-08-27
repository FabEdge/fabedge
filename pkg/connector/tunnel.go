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

package connector

import (
	"fmt"

	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/jjeffery/stringset"
)

var (
	connNames stringset.Set
)

func (m *Manager) readCfgFromFile() error {
	nc, err := netconf.LoadNetworkConf(m.config.tunnelConfigFile)
	if err != nil {
		return err
	}

	netconf.EnsureNodeSubnets(&nc)

	m.connections = nil
	connNames = stringset.New()

	for _, peer := range nc.Peers {

		con := tunnel.ConnConfig{
			Name: fmt.Sprintf("cloud-%s", peer.Name),

			LocalID:          nc.ID,
			LocalCerts:       []string{m.config.certFile},
			LocalAddress:     []string{nc.IP},
			LocalSubnets:     nc.Subnets,
			LocalNodeSubnets: nc.NodeSubnets,

			RemoteID:          peer.ID,
			RemoteAddress:     []string{peer.IP},
			RemoteSubnets:     peer.Subnets,
			RemoteNodeSubnets: peer.NodeSubnets,
		}
		m.connections = append(m.connections, con)
		connNames.Add(con.Name)
	}

	return nil
}

// remote local and remote address to support IPSec NAT_T
func removeLocalAndRemoteAddress(conn tunnel.ConnConfig) tunnel.ConnConfig {
	c := conn
	c.LocalAddress = nil
	c.RemoteAddress = nil
	return c
}

func (m *Manager) syncConnections() error {
	if err := m.readCfgFromFile(); err != nil {
		return err
	}

	oldNames, err := m.tm.ListConnNames()
	if err != nil {
		return err
	}

	// remove inactive connections
	for _, name := range oldNames {
		if !connNames.Contains(name) {
			if err = m.tm.UnloadConn(name); err != nil {
				return err
			}
		}
	}

	// load active connections
	for _, c := range m.connections {
		if err = m.tm.LoadConn(removeLocalAndRemoteAddress(c)); err != nil {
			return err
		}
	}

	return nil
}
