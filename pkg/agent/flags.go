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
	"flag"
	"fmt"
	"time"
)

var (
	version bool

	tunnelsConfPath  string
	servicesConfPath string

	localCert        string
	debounceDuration int64
	syncPeriod       int64

	cniVersion     string
	cniConfDir     string
	cniNetworkName string
	cniBridgeName  string

	masqOutgoing bool
	edgePodCIDR  string

	dummyInterfaceName string
	xfrmInterfaceName  string
	xfrmInterfaceID    uint
	useXfrm            bool

	enableProxy bool
)

func init() {
	flag.BoolVar(&version, "version", false, "display version info")

	flag.StringVar(&tunnelsConfPath, "tunnels-conf", "/etc/fabedge/tunnels.yaml", "The path to tunnels configuration file")
	flag.StringVar(&servicesConfPath, "services-conf", "/etc/fabedge/services.yaml", "The file that records information about services and endpointslices")

	flag.StringVar(&localCert, "local-cert", "edgecert.pem", "The path to cert file. If it's a relative path, the cert file should be put under /etc/ipsec.d/certs")
	flag.Int64Var(&debounceDuration, "debounce", 1, "The debounce delay(seconds) to avoid too much network reconfiguring")
	flag.Int64Var(&syncPeriod, "sync-period", 30, "The period(seconds) to synchronize network configuration")

	flag.StringVar(&cniVersion, "cni-version", "0.3.1", "cni version")
	flag.StringVar(&cniConfDir, "cni-conf-path", "/etc/cni/net.d", "cni version")
	flag.StringVar(&cniNetworkName, "cni-network-name", "fabedge", "the name of network")
	flag.StringVar(&cniBridgeName, "cni-bridge-name", "br-fabedge", "the name of bridge")

	flag.BoolVar(&masqOutgoing, "masq-outgoing", true, "Configure faberge networking to perform outbound NAT for connections from pods to outside of the cluster")
	flag.StringVar(&edgePodCIDR, "edge-pod-cidr", "2.0.0.0/8", "CIDR of the edge pod")

	flag.StringVar(&dummyInterfaceName, "dummy-interface-name", "fabedge-ipvs0", "the name of dummy interface")
	flag.BoolVar(&useXfrm, "use-xfrm", false, "use xfrm when OS has this feature")
	flag.StringVar(&xfrmInterfaceName, "xfrm-interface-name", "ipsec42", "the name of xfrm interface")
	flag.UintVar(&xfrmInterfaceID, "xfrm-interface-id", 42, "the id of xfrm interface")

	flag.BoolVar(&enableProxy, "enable-proxy", true, "Enable the proxy feature")
}

func validateFlags() error {
	if debounceDuration <= 0 {
		return fmt.Errorf("the least debounce value is 1")
	}

	if syncPeriod <= 0 {
		return fmt.Errorf("the least sync period value is 1")
	}

	return nil
}

func AsSecond(value int64) time.Duration {
	return time.Duration(value) * time.Second
}
