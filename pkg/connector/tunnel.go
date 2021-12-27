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

package connector

import (
	"k8s.io/klog/v2"

	"github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/jjeffery/stringset"
)

var (
	connNames stringset.Set
)

func (m *Manager) readCfgFromFile() error {
	nc, err := netconf.LoadNetworkConf(m.TunnelConfigFile)
	if err != nil {
		return err
	}

	m.connections = nil
	connNames = stringset.New()

	for _, peer := range nc.Peers {

		con := tunnel.ConnConfig{
			Name: peer.Name,

			LocalID:          nc.ID,
			LocalCerts:       []string{m.CertFile},
			LocalAddress:     nc.PublicAddresses,
			LocalSubnets:     nc.Subnets,
			LocalNodeSubnets: nc.NodeSubnets,
			LocalType:        nc.Type,

			RemoteID:          peer.ID,
			RemoteAddress:     peer.PublicAddresses,
			RemoteSubnets:     peer.Subnets,
			RemoteNodeSubnets: peer.NodeSubnets,
			RemoteType:        peer.Type,
		}
		m.connections = append(m.connections, con)
		connNames.Add(con.Name)
	}

	return nil
}

func (m *Manager) syncConnections() error {
	if err := m.readCfgFromFile(); err != nil {
		return err
	}

	klog.V(5).Infof("connections:%+v", m.connections)

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
		switch c.RemoteType {
		case v1alpha1.EdgeNode:
			c.LocalAddress = nil  // we do not care local ip address
			c.RemoteAddress = nil // we just wait the connection from remote edge nodes
			if err = m.tm.LoadConn(c); err != nil {
				klog.Errorf("failed to load connection:%s", err)
			}
		case v1alpha1.Connector:
			c.LocalAddress = nil // we do not care local ip address
			if err = m.tm.LoadConn(c); err != nil {
				klog.Errorf("failed to load connection:%s", err)
			}
			if err = m.tm.InitiateConn(c.Name); err != nil {
				klog.Errorf("failed to initiate connection:%s", err)
			}
		default:
			klog.Errorf("connection type:%s is not implemented", c.RemoteType)
		}
	}

	return nil
}
