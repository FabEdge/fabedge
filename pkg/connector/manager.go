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
	"github.com/fabedge/fabedge/pkg/common/about"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coreos/go-iptables/iptables"
	"k8s.io/klog/v2"
	k8sexec "k8s.io/utils/exec"

	"github.com/fabedge/fabedge/pkg/connector/routing"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
	utilipset "github.com/fabedge/fabedge/third_party/ipset"
)

type Manager struct {
	config      *config
	tm          tunnel.Manager
	ipt         *iptables.IPTables
	connections []tunnel.ConnConfig
	ipset       utilipset.Interface
	router      routing.Routing
}

type config struct {
	interval         time.Duration //sync interval
	debounceDuration time.Duration
	tunnelConfigFile string
	certFile         string
	viciSocket       string
	cniType          string
}

func NewManager() *Manager {
	c := &config{
		interval:         syncPeriod,
		tunnelConfigFile: tunnelConfig,
		certFile:         certFile,
		viciSocket:       viciSocket,
		debounceDuration: debounceDuration,
		cniType:          cniType,
	}

	tm, err := strongswan.New(
		strongswan.SocketFile(c.viciSocket),
		strongswan.StartAction("none"),
	)
	if err != nil {
		klog.Fatal(err)
	}

	ipt, err := iptables.New()
	if err != nil {
		klog.Fatal(err)
	}

	execer := k8sexec.New()
	ipset := utilipset.New(execer)

	router, err := routing.GetRouter(cniType)
	if err != nil {
		klog.Fatalf("%s", err)
	}

	return &Manager{
		config: c,
		tm:     tm,
		ipt:    ipt,
		ipset:  ipset,
		router: router,
	}
}

func runTasks(interval time.Duration, handler ...func()) {
	t := time.Tick(interval)
	for {
		for _, h := range handler {
			h()
		}
		<-t
	}
}

func (m *Manager) Start() {
	routeTaskFn := func() {
		active, err := m.tm.IsActive()
		if err != nil {
			klog.Errorf("failed to get tunnel manager status: %s", err)
			return
		}
		err = m.router.SyncRoutes(active, m.connections)
		if err != nil {
			klog.Errorf("failed to sync routes: %s", err)
			return
		}

		klog.Info("routes are synced")
	}

	iptablesTaskFn := func() {
		if err := m.ensureForwardIPTablesRules(); err != nil {
			klog.Errorf("error when to add iptables forward rules: %s", err)
		} else {
			klog.Infof("iptables forward rules are added")
		}

		if err := m.ensureSNatIPTablesRules(); err != nil {
			klog.Errorf("error when to add iptables SNAT rules: %s", err)
		} else {
			klog.Infof("iptables SNAT rules are added")
		}

		if err := m.ensureInputIPTablesRules(); err != nil {
			klog.Errorf("error when to add iptables input rules: %s", err)
		} else {
			klog.Infof("iptables input rules are added")
		}
	}

	tunnelTaskFn := func() {
		if err := m.syncConnections(); err != nil {
			klog.Errorf("error when to sync tunnels: %s", err)
		} else {
			klog.Infof("tunnels are synced")
		}
	}

	ipsetTaskFn := func() {
		if err := m.syncEdgeNodeIPSet(); err != nil {
			klog.Errorf("error when to sync ipset %s: %s", IPSetEdgeNodeIP, err)
		} else {
			klog.Infof("ipset %s are synced", IPSetEdgeNodeIP)
		}

		if err := m.syncCloudPodCIDRSet(); err != nil {
			klog.Errorf("error when to sync ipset %s: %s", IPSetCloudPodCIDR, err)
		} else {
			klog.Infof("ipset %s are synced", IPSetCloudPodCIDR)
		}

		if err := m.syncCloudNodeCIDRSet(); err != nil {
			klog.Errorf("error when to sync ipset %s: %s", IPSetCloudNodeCIDR, err)
		} else {
			klog.Infof("ipset %s are synced", IPSetCloudNodeCIDR)
		}
	}
	tasks := []func(){tunnelTaskFn, routeTaskFn, ipsetTaskFn, iptablesTaskFn}

	// repeats regular tasks periodically
	go runTasks(m.config.interval, tasks...)

	// sync ALL when config file changed
	go m.onConfigFileChange(m.config.tunnelConfigFile, tasks...)

	about.DisplayVersion()
	klog.Info("manager started")
	klog.V(5).Infof("current config:%+v", m.config)

	// wait os signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	m.gracefulShutdown()
	klog.Info("connector stopped")
}

func (m *Manager) gracefulShutdown() {
	err := m.router.CleanRoutes(m.connections)
	if err != nil {
		klog.Errorf("failed to clean routers: %s", err)
	}

	err = m.SNatIPTablesRulesCleanup()
	if err != nil {
		klog.Errorf("failed to clean iptables: %s", err)
	}
}
