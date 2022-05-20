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
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	debpkg "github.com/bep/debounce"
	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2/klogr"

	"github.com/fabedge/fabedge/pkg/common/about"
	"github.com/fabedge/fabedge/pkg/connector/routing"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/fabedge/fabedge/pkg/tunnel/strongswan"
	"github.com/fabedge/fabedge/pkg/util/memberlist"
)

type Manager struct {
	Config

	tm          tunnel.Manager
	iptHandler  *IPTablesHandler
	ipt6Handler *IPTablesHandler
	connections []tunnel.ConnConfig
	router      routing.Routing
	mc          *memberlist.Client
	log         logr.Logger

	events   chan struct{}
	debounce func(func())
}

type Config struct {
	SyncPeriod       time.Duration //sync interval
	DebounceDuration time.Duration
	TunnelConfigFile string
	CertFile         string
	ViciSocket       string
	CNIType          string
	initMembers      []string
}

func msgHandler(b []byte)         {}
func nodeLeveHandler(name string) {}

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.TunnelConfigFile, "tunnel-config", "/etc/fabedge/tunnels.yaml", "tunnel config file")
	fs.StringVar(&c.CertFile, "cert-file", "/etc/ipsec.d/certs/tls.crt", "TLS certificate file")
	fs.StringVar(&c.ViciSocket, "vici-socket", "/var/run/charon.vici", "vici socket file")
	fs.StringVar(&c.CNIType, "cni-type", "flannel", "CNI type used in cloud")
	fs.DurationVar(&c.SyncPeriod, "sync-period", 5*time.Minute, "period to sync routes/rules")
	fs.DurationVar(&c.DebounceDuration, "debounce-duration", 5*time.Second, "period to sync routes/rules")
	fs.StringSliceVar(&c.initMembers, "connector-node-addresses", []string{}, "internal address of all connector nodes")
}

func (c Config) Manager() (*Manager, error) {
	tm, err := strongswan.New(
		strongswan.SocketFile(c.ViciSocket),
		strongswan.StartAction("none"),
	)
	if err != nil {
		return nil, err
	}

	router, err := routing.GetRouter(c.CNIType)
	if err != nil {
		return nil, err
	}

	mc, err := memberlist.New(c.initMembers, msgHandler, nodeLeveHandler)
	if err != nil {
		return nil, err
	}

	ipt, err := newIP4TablesHandler()
	if err != nil {
		return nil, err
	}

	ipt6, err := newIP6TablesHandler()
	if err != nil {
		return nil, err
	}

	return &Manager{
		Config:      c,
		tm:          tm,
		iptHandler:  ipt,
		ipt6Handler: ipt6,
		router:      router,
		mc:          mc,
		log:         klogr.New().WithName("manager"),
		events:      make(chan struct{}),
		debounce:    debpkg.New(c.DebounceDuration),
	}, nil
}

func (m *Manager) startTick() {
	tick := time.NewTicker(m.SyncPeriod)
	defer tick.Stop()

	for {
		m.notify()
		<-tick.C
	}
}

func (m *Manager) notify() {
	m.debounce(func() {
		m.events <- struct{}{}
	})
}

func (m *Manager) Start() {
	about.DisplayVersion()

	m.clearFabEdgeIptablesChains()

	go m.workLoop()
	go m.startTick()
	go m.onConfigFileChange(m.TunnelConfigFile)

	m.log.V(5).Info("manager started", "config", m.Config)

	// wait os signal
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	m.gracefulShutdown()

	m.log.Info("manager stopped")
}

func (m *Manager) gracefulShutdown() {
	err := m.router.CleanRoutes(m.connections)
	if err != nil {
		m.log.Error(err, "failed to clean routers")
	}

	m.CleanSNatIPTablesRules()
}

func (m *Manager) CleanSNatIPTablesRules() {
	for _, ipt := range []*IPTablesHandler{m.iptHandler, m.ipt6Handler} {
		if err := ipt.CleanSNatIPTablesRules(); err != nil {
			m.log.Error(err, "failed to clean iptables")
		}
	}
}

func (m *Manager) clearFabEdgeIptablesChains() {
	for _, ipt := range []*IPTablesHandler{m.iptHandler, m.ipt6Handler} {
		if err := ipt.clearFabEdgeIptablesChains(); err != nil {
			m.log.Error(err, "failed to clean iptables")
		}
	}
}

func (m *Manager) mainRoutes() {
	active, err := m.tm.IsActive()
	if err != nil {
		m.log.Error(err, "failed to get tunnel manager status")
		return
	}

	if active {
		m.log.V(5).Info("tunnel manager is active, try to synchronize routes in table 220")
		if err = m.router.SyncRoutes(m.connections); err != nil {
			m.log.Error(err, "failed to sync routes")
			return
		}
	} else {
		m.log.V(5).Info("tunnel manager is not active, try to clean routes in route table 220")
		if err = m.router.CleanRoutes(m.connections); err != nil {
			m.log.Error(err, "failed to clean routes")
			return
		}
	}

	m.log.V(5).Info("routes are synced")
}

func (m *Manager) mainTunnels() {
	if err := m.syncConnections(); err != nil {
		m.log.Error(err, "error when to sync tunnels")
	} else {
		m.log.V(5).Info("tunnels are synced")
	}
}

// broadcastConnectorPrefixes broadcasts the active routing info to all cloud agents.
func (m *Manager) broadcastConnectorPrefixes() {
	cp, err := m.router.GetConnectorPrefixes()
	if err != nil {
		m.log.Error(err, "failed to get connector prefixes")
		return
	}

	log := m.log.WithValues("connectorPrefixes", cp)
	log.V(5).Info("get connector prefixes")
	b, err := json.Marshal(cp)
	if err != nil {
		log.Error(err, "failed to marshal prefixes")
		return
	}

	m.mc.Broadcast(b)
	log.V(5).Info("connector prefixes is broadcast to cloud-agents")
}

func (m *Manager) workLoop() {
	for range m.events {
		m.mainTunnels()
		m.mainRoutes()
		m.broadcastConnectorPrefixes()
		m.iptHandler.maintainIPTables()
		m.ipt6Handler.maintainIPTables()
		m.iptHandler.maintainIPSet()
		m.ipt6Handler.maintainIPSet()
	}
}
