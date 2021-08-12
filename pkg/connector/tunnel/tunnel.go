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

package tunnel

import (
	"fmt"
	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
	"github.com/jjeffery/stringset"
	"github.com/spf13/viper"
)

var (
	connections []tunnel.ConnConfig
	connNames   stringset.Set
)

func ReadCfgFromFile() error {
	cfgFile := viper.GetString("tunnelconfig")

	nc, err := netconf.LoadNetworkConf(cfgFile)
	if err != nil {
		return err
	}

	netconf.EnsureNodeSubnets(&nc)

	cert := viper.GetString("certFile")
	connections = []tunnel.ConnConfig{}
	connNames = stringset.New()

	for _, peer := range nc.Peers {

		con := tunnel.ConnConfig{
			Name: fmt.Sprintf("cloud-%s", peer.Name),

			LocalID:          nc.ID,
			LocalCerts:       []string{cert},
			LocalSubnets:     nc.Subnets,
			LocalNodeSubnets: nc.NodeSubnets,

			RemoteID:          peer.ID,
			RemoteAddress:     []string{peer.IP},
			RemoteSubnets:     peer.Subnets,
			RemoteNodeSubnets: peer.NodeSubnets,
		}
		connections = append(connections, con)
		connNames.Add(con.Name)
	}

	return nil
}

func SyncConnections() error {
	if err := ReadCfgFromFile(); err != nil {
		return err
	}

	viciSocket := viper.GetString("vicisocket")
	tm, err := strongswan.New(
		strongswan.SocketFile(viciSocket),
		strongswan.StartAction("none"),
	)
	if err != nil {
		return err
	}

	oldNames, err := tm.ListConnNames()
	if err != nil {
		return err
	}

	// remove inactive connections
	for _, name := range oldNames {
		if !connNames.Contains(name) {
			if err = tm.UnloadConn(name); err != nil {
				return err
			}
		}
	}

	// load active connections
	for _, c := range connections {
		if err = tm.LoadConn(c); err != nil {
			return err
		}
	}

	return nil
}

func UnloadConnections() error {
	viciSocket := viper.GetString("vicisocket")
	tm, err := strongswan.New(strongswan.SocketFile(viciSocket))
	if err != nil {
		return err
	}

	allNames, err := tm.ListConnNames()
	if err != nil {
		return err
	}

	for _, name := range allNames {
		_ = tm.UnloadConn(name)
	}

	return nil
}
