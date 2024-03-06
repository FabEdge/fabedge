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
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	debpkg "github.com/bep/debounce"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	"go.uber.org/atomic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2/klogr"

	cloud_agent "github.com/fabedge/fabedge/pkg/cloud-agent"
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

	kubeClient *clientset.Clientset
	isLeader   *atomic.Bool

	cloudAgent *cloud_agent.CloudAgent

	events   chan struct{}
	debounce func(func())
}

type Config struct {
	SyncPeriod        time.Duration //sync interval
	DebounceDuration  time.Duration
	TunnelConfigFile  string
	CertFile          string
	ViciSocket        string
	CNIType           string
	InitMembers       []string
	TunnelInitTimeout uint
	ListenAddress     string

	LeaderElection struct {
		LockName      string
		LeaseDuration time.Duration
		RenewDeadline time.Duration
		RetryPeriod   time.Duration
	}
}

func (c *Config) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.TunnelConfigFile, "tunnel-config", "/etc/fabedge/tunnels.yaml", "tunnel config file")
	fs.StringVar(&c.CertFile, "cert-file", "/etc/ipsec.d/certs/tls.crt", "TLS certificate file")
	fs.StringVar(&c.ViciSocket, "vici-socket", "/var/run/charon.vici", "vici socket file")
	fs.DurationVar(&c.DebounceDuration, "debounce-duration", 5*time.Second, "period to sync routes/rules")
	fs.StringVar(&c.LeaderElection.LockName, "leader-lock-name", "connector", "The name of leader lock")
	fs.StringVar(&c.ListenAddress, "listen-address", "127.0.0.1:30306", "The address of http server")
}

func (c Config) Manager() (*Manager, error) {
	tm, err := strongswan.New(
		strongswan.SocketFile(c.ViciSocket),
		strongswan.StartAction("none"),
		strongswan.InitTimeout(10),
	)
	if err != nil {
		return nil, err
	}

	router, err := routing.GetRouter(c.CNIType)
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

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client := clientset.NewForConfigOrDie(config)

	cloudAgent, err := cloud_agent.NewCloudAgent()
	if err != nil {
		return nil, err
	}

	manager := &Manager{
		Config:      c,
		tm:          tm,
		iptHandler:  ipt,
		ipt6Handler: ipt6,
		router:      router,

		kubeClient: client,
		isLeader:   atomic.NewBool(false),

		cloudAgent: cloudAgent,

		log: klogr.New().WithName("manager"),

		events:   make(chan struct{}),
		debounce: debpkg.New(c.DebounceDuration),
	}

	mc, err := memberlist.New(c.InitMembers, manager.handleMessage, manager.handleNodeLeave)
	if err != nil {
		return nil, err
	}
	manager.mc = mc

	return manager, nil
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

	go m.runLeaderElection()
	go m.runHTTPServer()
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

func (m *Manager) runLeaderElection() {
	lock := newLock(m.LeaderElection.LockName, m.kubeClient)
	leaderID := lock.Identity()

	for {
		m.log.V(3).Info("Begin acquiring leader lock", "id", leaderID)
		leaderelection.RunOrDie(context.Background(), leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   m.LeaderElection.LeaseDuration,
			RenewDeadline:   m.LeaderElection.RenewDeadline,
			RetryPeriod:     m.LeaderElection.RetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(c context.Context) {
					m.log.V(3).Info("Get leader role, clear iptables rules generated as cloud agent")
					m.cloudAgent.CleanAll()
					m.isLeader.Store(true)
					m.notify()
				},
				OnStoppedLeading: func() {
					m.log.V(3).Info("Lose leader role, clear iptables and routes")
					m.clearAll()
					m.isLeader.Store(false)
				},
				OnNewLeader: func(currentID string) {
					if currentID == leaderID {
						m.log.V(5).Info("Still be the leader!")
					} else {
						m.log.V(3).Info("Leader has changed", "NewLeaderID", currentID)
					}
				},
			},
		})
		m.log.V(3).Info("Give up leader election", "id", leaderID)
	}
}

func (m *Manager) gracefulShutdown() {
	err := m.router.CleanRoutes(m.connections)
	if err != nil {
		m.log.Error(err, "failed to clean routes")
	}

	m.removeAllChains()
}

func (m *Manager) clearAll() {
	err := m.router.CleanRoutes(m.connections)
	if err != nil {
		m.log.Error(err, "failed to clean routes")
	}

	m.flushAllChains()
	m.clearConnections()
}

func (m *Manager) flushAllChains() {
	for _, h := range []*IPTablesHandler{m.iptHandler, m.ipt6Handler} {
		if err := h.ipt.Flush(); err != nil {
			m.log.Error(err, "failed to flush iptables rules")
		}
	}
}

func (m *Manager) removeAllChains() {
	for _, h := range []*IPTablesHandler{m.iptHandler, m.ipt6Handler} {
		if err := h.ipt.Remove(); err != nil {
			m.log.Error(err, "failed to clean iptables")
		}
	}
}

func (m *Manager) maintainRoutes() {
	m.log.V(5).Info("tunnel manager is active, try to synchronize routes in table 220")
	if err := m.router.SyncRoutes(m.connections); err != nil {
		m.log.Error(err, "failed to sync routes")
		return
	}
	m.log.V(5).Info("routes are synced")
}

func (m *Manager) maintainTunnels() {
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

func (m *Manager) handleMessage(msgBytes []byte) {
	if m.isLeader.Load() {
		return
	}

	m.cloudAgent.HandleMessage(msgBytes)
}

func (m *Manager) handleNodeLeave(name string) {
	m.cloudAgent.HandleNodeLeave(name)
}

func (m *Manager) workLoop() {
	for range m.events {
		if !m.isLeader.Load() {
			continue
		}

		m.maintainRoutes()

		m.iptHandler.maintainIPTables()
		m.ipt6Handler.maintainIPTables()
		m.broadcastConnectorPrefixes()

		// maintainTunnels may last for minutes, so put it at the end, otherwise it may cause error, such as wrong iptables
		// rules and wrong routes are generated after isLeader is set to false
		m.maintainTunnels()
	}
}

func (m *Manager) runHTTPServer() {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Get("/is-leader", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(m.isLeader.String()))
	})
	server := &http.Server{
		Addr:    m.ListenAddress,
		Handler: r,
	}

	for {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.log.Error(err, "failed to start http server")
		}
		break
	}
}

// getConnectorName will return a valid name as leader election ID
func getConnectorName() string {
	hostname, _ := os.Hostname()
	if hostname != "" {
		return hostname
	}

	hostname = os.Getenv("HOSTNAME")
	if hostname != "" {
		return hostname
	}

	podName := os.Getenv("POD_NAME")
	return podName
}

// getNamespace return the namespace where connector pod is running
func getNamespace() string {
	return os.Getenv("NAMESPACE")
}

func newLock(lockName string, client *clientset.Clientset) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockName,
			Namespace: getNamespace(),
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: getConnectorName(),
		},
	}
}
