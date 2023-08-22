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
	"fmt"
	"strings"

	"github.com/fabedge/fabedge/pkg/apis/v1alpha1"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"k8s.io/apimachinery/pkg/util/sets"
)

var errInvalidEndpointType = fmt.Errorf("invalid endpoint type")

func (m *Manager) readCfgFromFile() error {
	nc, err := netconf.LoadNetworkConf(m.TunnelConfigFile)
	if err != nil {
		return err
	}

	connections := make([]tunnel.ConnConfig, 0, len(nc.Peers)+1)
	// for now, connector is the only mediator candidate,
	// for mediator itself, there is no need to configure remote settings,
	// and just connector's cert for mediator's local cert
	if nc.Mediator != nil {
		mediator := nc.Mediator
		connections = append(connections, tunnel.ConnConfig{
			Name: mediator.Name,

			LocalID:    mediator.ID,
			LocalCerts: []string{m.CertFile},
			Mediation:  true,
		})
	}

	for _, peer := range nc.Peers {
		conn := tunnel.ConnConfig{
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
			RemotePort:        peer.Port,
		}
		connections = append(connections, conn)
	}
	m.connections = connections

	m.classifyConnectionSubnets()
	return nil
}

func (m *Manager) classifyConnectionSubnets() {
	var (
		edgePodCIDRs   = sets.NewString()
		edgePodCIDRs6  = sets.NewString()
		edgeNodeCIDRs  = sets.NewString()
		edgeNodeCIDRs6 = sets.NewString()

		cloudPodCIDRs   = sets.NewString()
		cloudPodCIDRs6  = sets.NewString()
		cloudNodeCIDRs  = sets.NewString()
		cloudNodeCIDRs6 = sets.NewString()
	)

	for _, conn := range m.connections {
		if conn.Mediation {
			continue
		}

		for _, cidr := range conn.RemoteSubnets {
			if isIPv6(cidr) {
				edgePodCIDRs6.Insert(cidr)
			} else {
				edgePodCIDRs.Insert(cidr)
			}
		}

		for _, cidr := range conn.LocalSubnets {
			if isIPv6(cidr) {
				cloudPodCIDRs6.Insert(cidr)
			} else {
				cloudPodCIDRs.Insert(cidr)
			}
		}

		if !inSameCluster(conn) {
			continue
		}

		for _, cidr := range conn.RemoteNodeSubnets {
			if isIPv6(cidr) {
				edgeNodeCIDRs6.Insert(cidr)
			} else {
				edgeNodeCIDRs.Insert(cidr)
			}
		}

		for _, cidr := range conn.LocalNodeSubnets {
			if isIPv6(cidr) {
				cloudNodeCIDRs6.Insert(cidr)
			} else {
				cloudNodeCIDRs.Insert(cidr)
			}
		}
	}

	m.iptHandler.setIPSetEntrySet(edgePodCIDRs, edgeNodeCIDRs, cloudPodCIDRs, cloudNodeCIDRs)
	m.ipt6Handler.setIPSetEntrySet(edgePodCIDRs6, edgeNodeCIDRs6, cloudPodCIDRs6, cloudNodeCIDRs6)
}

func (m *Manager) syncConnections() error {
	err := m.readCfgFromFile()
	if err != nil {
		m.log.Error(err, "failed to read tunnel config file")
		return err
	} else {
		m.log.V(5).Info("new connections is loaded", "connections", m.connections)
	}

	// load active connections
	nameSet := sets.NewString()
	for _, c := range m.connections {
		nameSet.Insert(c.Name)

		log := m.log.WithValues("connection", c)
		switch {
		case c.Mediation:
			if err = m.tm.LoadConn(c); err != nil {
				log.Error(err, "failed to load connection")
			}
		case c.RemoteType == v1alpha1.EdgeNode:
			c.LocalAddress = nil  // we do not care local ip address
			c.RemoteAddress = nil // we just wait the connection from remote edge nodes
			if err = m.tm.LoadConn(c); err != nil {
				log.Error(err, "failed to load connection")
			}
		case c.RemoteType == v1alpha1.Connector:
			c.LocalAddress = nil // we do not care local ip address
			if err = m.tm.LoadConn(c); err != nil {
				log.Error(err, "failed to load connection")
			}
			if err = m.tm.InitiateConn(c.Name); err != nil {
				log.Error(err, "failed to initiate connection")
			}
		default:
			log.Error(errInvalidEndpointType, "failed to load connection", "remoteType", c.RemoteType)
		}
	}

	oldNames, err := m.tm.ListConnNames()
	if err != nil {
		m.log.Error(err, "failed to get existing connection from tunnel manager")
		return err
	}

	// remove inactive connections
	for _, name := range oldNames {
		if nameSet.Has(name) {
			continue
		}

		if err = m.tm.UnloadConn(name); err != nil {
			m.log.Error(err, "failed to unload tunnel connection", "name", name)
		} else {
			m.log.V(5).Info("A staled tunnel connection is unloaded", "name", name)
		}
	}

	return nil
}

func (m *Manager) clearConnections() {
	oldNames, err := m.tm.ListConnNames()
	if err != nil {
		m.log.Error(err, "failed to get existing connection from tunnel manager")
		return
	}

	for _, name := range oldNames {
		if err = m.tm.UnloadConn(name); err != nil {
			m.log.Error(err, "failed to unload tunnel connection", "name", name)
		} else {
			m.log.V(5).Info("A staled tunnel connection is unloaded", "name", name)
		}
	}
}

// isIP6 check if ip is an IP6 address or a CIDR address
func isIPv6(ip string) bool {
	return strings.IndexByte(ip, ':') != -1
}

func inSameCluster(c tunnel.ConnConfig) bool {
	if c.RemoteType == v1alpha1.Connector {
		return false
	}

	l := strings.Split(c.LocalID, ".")  // e.g. fabedge.connector
	r := strings.Split(c.RemoteID, ".") // e.g. fabedge.edge1

	return l[0] == r[0]
}
