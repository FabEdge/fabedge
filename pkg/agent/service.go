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

import (
	"io/ioutil"
	"net"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"

	"github.com/fabedge/fabedge/pkg/common/netconf"
	"github.com/fabedge/fabedge/third_party/ipvs"
)

type server struct {
	virtualServer *ipvs.VirtualServer
	realServers   []*ipvs.RealServer
}

func loadServiceConf(path string) (netconf.VirtualServers, error) {
	var conf netconf.VirtualServers

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return conf, err
	}

	return conf, yaml.Unmarshal(data, &conf)
}

func toServers(vssConf netconf.VirtualServers) []server {
	servers := []server{}
	for _, vsConf := range vssConf {
		server := server{
			virtualServer: toVirtualServer(vsConf),
		}
		for _, rsConf := range vsConf.RealServers {
			server.realServers = append(server.realServers, toRealServer(rsConf))
		}
		servers = append(servers, server)
	}
	return servers
}

func toVirtualServer(vsConf netconf.VirtualServer) *ipvs.VirtualServer {
	vs := ipvs.VirtualServer{
		Address:  net.ParseIP(vsConf.IP),
		Protocol: string(vsConf.Protocol),
		Port:     uint16(vsConf.Port),
	}

	if len(vsConf.Scheduler) == 0 {
		vsConf.Scheduler = ipvs.DefaultScheduler
	}
	vs.Scheduler = vsConf.Scheduler

	if vsConf.SessionAffinity == v1.ServiceAffinityClientIP {
		vs.Flags |= ipvs.FlagPersistent
		vs.Timeout = uint32(vsConf.StickyMaxAgeSeconds)
	}

	return &vs
}

func toRealServer(rsConf netconf.RealServer) *ipvs.RealServer {
	return &ipvs.RealServer{
		Address: net.ParseIP(rsConf.IP),
		Port:    uint16(rsConf.Port),
		Weight:  1,
	}
}
