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

package manager

import (
	"github.com/fabedge/fabedge/pkg/connector/iptables"
	"github.com/fabedge/fabedge/pkg/connector/route"
	"github.com/fabedge/fabedge/pkg/connector/tunnel"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Manager struct {
	config *config
}

type config struct {
	interval         time.Duration
	edgePodCIDR      string
	tunnelConfigFile string
}

func NewManager() *Manager {
	c := &config{
		interval:         viper.GetDuration("syncPeriod"),
		edgePodCIDR:      viper.GetString("edgePodCIDR"),
		tunnelConfigFile: viper.GetString("tunnelConfig"),
	}
	return &Manager{
		config: c,
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
		if err := route.SyncRoutes(m.config.edgePodCIDR); err != nil {
			klog.Errorf("error to sync routes: %s", err)
		} else {
			klog.Infof("routes:%s are synced", m.config.edgePodCIDR)
		}
	}

	iptablesTaskFn := func() {
		if err := iptables.EnsureIPTablesRules(m.config.edgePodCIDR); err != nil {
			klog.Errorf("error when to add iptables rules: %s", err)
		} else {
			klog.Infof("iptables rules are added")
		}
	}

	tunnelTaskFn := func() {
		if err := tunnel.SyncConnections(); err != nil {
			klog.Errorf("error when to sync tunnels: %s", err)
		} else {
			klog.Infof("tunnels are synced")
		}
	}

	tasks := []func(){routeTaskFn, iptablesTaskFn, tunnelTaskFn}

	// repeats regular tasks periodically
	go runTasks(m.config.interval, tasks...)

	// sync tunnels when config file updated by cloud.
	go onConfigFileChange(m.config.tunnelConfigFile, tunnelTaskFn)

	klog.Info("manager started")

	// wait os signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	m.gracefulShutdown()
	klog.Info("manager stopped")
}

func (m *Manager) gracefulShutdown() {
	//immediately sync
	_ = route.SyncRoutes(m.config.edgePodCIDR)
}
