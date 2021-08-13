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
	"github.com/coreos/go-iptables/iptables"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

type Manager struct {
	config      *config
	tm          tunnel.Manager
	ipt         *iptables.IPTables
	connections []tunnel.ConnConfig
}

type config struct {
	interval         time.Duration //sync interval
	debounceDuration time.Duration
	edgePodCIDR      string
	tunnelConfigFile string
	certFile         string
	viciSocket       string
}

func NewManager() *Manager {
	c := &config{
		interval:         viper.GetDuration("syncPeriod"),
		edgePodCIDR:      viper.GetString("edgePodCIDR"),
		tunnelConfigFile: viper.GetString("tunnelConfig"),
		certFile:         viper.GetString("certFile"),
		viciSocket:       viper.GetString("vicisocket"),
		debounceDuration: viper.GetDuration("debounceDuration"),
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

	return &Manager{
		config: c,
		tm:     tm,
		ipt:    ipt,
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
		if err := m.syncRoutes(); err != nil {
			klog.Errorf("error to sync routes: %s", err)
		} else {
			klog.Info("routes are synced")
		}
	}

	iptablesTaskFn := func() {
		if err := m.ensureIPTablesRules(m.config.edgePodCIDR); err != nil {
			klog.Errorf("error when to add iptables rules: %s", err)
		} else {
			klog.Infof("iptables rules are added")
		}
	}

	tunnelTaskFn := func() {
		if err := m.syncConnections(); err != nil {
			klog.Errorf("error when to sync tunnels: %s", err)
		} else {
			klog.Infof("tunnels are synced")
		}
	}

	tasks := []func(){tunnelTaskFn, routeTaskFn, iptablesTaskFn}

	// repeats regular tasks periodically
	go runTasks(m.config.interval, tasks...)

	// sync tunnels when config file updated by cloud.
	go m.onConfigFileChange(m.config.tunnelConfigFile, tunnelTaskFn)

	klog.Info("manager started")

	// wait os signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	m.gracefulShutdown()
	klog.Info("manager stopped")
}

func (m *Manager) gracefulShutdown() {
	_ = m.RouteCleanup()

	command := exec.Command("ip", "xfrm", "policy", "flush")
	_ = command.Run()
	command = exec.Command("ip", "xfrm", "state", "flush")
	_ = command.Run()
}
