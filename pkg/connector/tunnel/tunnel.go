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

	netConf, err := netconf.LoadNetworkConf(cfgFile)
	if err != nil {
		return err
	}

	cert := viper.GetString("certFile")
	connections = []tunnel.ConnConfig{}
	connNames = stringset.New()

	for _, peer := range netConf.Peers {

		// append the ip of peer alongside remote subnets
		remoteSubnets := []string{peer.IP}
		remoteSubnets = append(remoteSubnets, peer.Subnets...)

		con := tunnel.ConnConfig{
			Name: fmt.Sprintf("cloud-%s", peer.Name),

			LocalID:      netConf.ID,
			LocalAddress: []string{netConf.IP},
			LocalSubnets: netConf.Subnets,
			LocalCerts:   []string{cert},

			RemoteID:      peer.ID,
			RemoteAddress: []string{peer.IP},
			RemoteSubnets: remoteSubnets,
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
	tm, err := strongswan.New(strongswan.WithSocketFile(viciSocket),
		strongswan.SetStartActions("none"))
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
	tm, err := strongswan.New(strongswan.WithSocketFile(viciSocket))
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
