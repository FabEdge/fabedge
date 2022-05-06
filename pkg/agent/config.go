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
	"fmt"
	"time"

	debpkg "github.com/bep/debounce"
	"github.com/coreos/go-iptables/iptables"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2/klogr"
	"k8s.io/utils/exec"

	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
	"github.com/fabedge/fabedge/pkg/util/ipset"
	"github.com/fabedge/fabedge/third_party/ipvs"
)

type CNI struct {
	Version     string
	ConfDir     string
	NetworkName string
	BridgeName  string
}

type Config struct {
	LocalCerts       []string
	SyncPeriod       time.Duration
	DebounceDuration time.Duration
	TunnelsConfPath  string
	ServicesConfPath string
	MASQOutgoing     bool

	DummyInterfaceName string

	UseXFRM           bool
	XFRMInterfaceName string
	XFRMInterfaceID   uint

	EnableIPAM        bool
	EnableHairpinMode bool
	NetworkPluginMTU  int
	CNI               CNI

	EnableProxy bool
}

func (cfg *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&cfg.TunnelsConfPath, "tunnels-conf", "/etc/fabedge/tunnels.yaml", "The path to tunnels configuration file")
	fs.StringVar(&cfg.ServicesConfPath, "services-conf", "/etc/fabedge/services.yaml", "The file that records information about services and endpointslices")

	fs.StringSliceVar(&cfg.LocalCerts, "local-cert", []string{"edgecert.pem"}, "The path to cert files, comma separated. If it's a relative path, the cert file should be put under /etc/ipsec.d/certs")
	fs.DurationVar(&cfg.DebounceDuration, "debounce", time.Second, "The debounce delay to avoid too much network reconfiguring")
	fs.DurationVar(&cfg.SyncPeriod, "sync-period", 30*time.Second, "The period to synchronize network configuration")

	fs.BoolVar(&cfg.EnableIPAM, "enable-ipam", true, "enable the IPAM feature")
	fs.BoolVar(&cfg.EnableHairpinMode, "enable-hairpinmode", true, "enable the Hairpin feature")
	fs.IntVar(&cfg.NetworkPluginMTU, "network-plugin-mtu", 1400, "Set network plugin MTU for edge nodes")
	fs.StringVar(&cfg.CNI.Version, "cni-version", "0.3.1", "cni version")
	fs.StringVar(&cfg.CNI.ConfDir, "cni-conf-path", "/etc/cni/net.d", "cni version")
	fs.StringVar(&cfg.CNI.NetworkName, "cni-network-name", "fabedge", "the name of network")
	fs.StringVar(&cfg.CNI.BridgeName, "cni-bridge-name", "br-fabedge", "the name of bridge")

	fs.BoolVar(&cfg.MASQOutgoing, "masq-outgoing", true, "Configure faberge networking to perform outbound NAT for connections from pods to outside of the cluster")

	fs.StringVar(&cfg.DummyInterfaceName, "dummy-interface-name", "fabedge-ipvs0", "the name of dummy interface")
	fs.BoolVar(&cfg.UseXFRM, "use-xfrm", false, "use xfrm when OS has this feature")
	fs.StringVar(&cfg.XFRMInterfaceName, "xfrm-interface-name", "ipsec42", "the name of xfrm interface")
	fs.UintVar(&cfg.XFRMInterfaceID, "xfrm-interface-id", 42, "the id of xfrm interface")

	fs.BoolVar(&cfg.EnableProxy, "enable-proxy", true, "Enable the proxy feature")
}

func (cfg *Config) Validate() error {
	if cfg.DebounceDuration < time.Second {
		return fmt.Errorf("the least debounce value is 1 second")
	}

	if cfg.SyncPeriod < time.Second {
		return fmt.Errorf("the least sync period value is 1 second")
	}

	return nil
}

func (cfg Config) Manager() (*Manager, error) {
	kernelHandler := ipvs.NewLinuxKernelHandler()
	if cfg.EnableProxy {
		if _, err := ipvs.CanUseIPVSProxier(kernelHandler); err != nil {
			return nil, err
		}
	}

	cfg.MASQOutgoing = cfg.EnableIPAM && cfg.MASQOutgoing

	var opts strongswan.Options
	if cfg.UseXFRM {
		supportXFRM, err := ipvs.SupportXfrmInterface(kernelHandler)
		if err != nil {
			return nil, err
		}
		if !supportXFRM {
			return nil, fmt.Errorf("xfrm interfaces have been supported since kernel 4.19, the current kernel version is too low")
		}

		opts = append(opts, strongswan.InterfaceID(&cfg.XFRMInterfaceID))
	}
	tm, err := strongswan.New(opts...)
	if err != nil {
		return nil, err
	}

	ipt, err := iptables.New()
	if err != nil {
		return nil, err
	}

	ip6t, err := iptables.NewWithProtocol(iptables.ProtocolIPv6)
	if err != nil {
		return nil, err
	}

	m := &Manager{
		Config: cfg,
		tm:     tm,
		ipt:    ipt,
		ip6t:   ip6t,
		log:    klogr.New().WithName("manager"),

		events:   make(chan struct{}),
		debounce: debpkg.New(cfg.DebounceDuration),

		netLink: ipvs.NewNetLinkHandle(false),
		ipvs:    ipvs.New(exec.New()),
		ipset:   ipset.New(),
	}

	return m, nil
}
