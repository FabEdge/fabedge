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

package strongswan

import (
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/strongswan/govici/vici"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/tunnel"
)

var _ tunnel.Manager = &StrongSwanManager{}

type StrongSwanManager struct {
	socketPath string
	certsPath  string

	// https://wiki.strongswan.org/projects/strongswan/wiki/Swanctlconf
	// The default of none loads the connection only, which then can be manually initiated or used as a responder configuration.
	// The value trap installs a trap policy, which triggers the tunnel as soon as matching traffic has been detected.
	// The value start initiates the connection actively.
	startAction string
	dpdAction   string
	dpdDelay    string
	interfaceID *uint

	// the time to wait for SA or child SA initiation finish
	initTimeout uint

	connectionByName map[string]tunnel.ConnConfig
	mu               *sync.RWMutex
}

type connection struct {
	LocalAddrs    []string               `vici:"local_addrs"`
	LocalPort     *uint                  `vici:"local_port"`
	RemoteAddrs   []string               `vici:"remote_addrs,omitempty"`
	RemotePort    *uint                  `vici:"remote_port"`
	LocalAuth     authConf               `vici:"local"`
	RemoteAuth    authConf               `vici:"remote"`
	Children      map[string]childSAConf `vici:"children"`
	IF_ID_IN      *uint                  `vici:"if_id_in"`
	IF_ID_OUT     *uint                  `vici:"if_id_out"`
	DpdDelay      string                 `vici:"dpd_delay,omitempty"`
	Mediation     string                 `vici:"mediation"`
	MediatedBy    string                 `vici:"mediated_by"`
	MediationPeer string                 `vici:"mediation_peer"`
}

type authConf struct {
	ID         string   `vici:"id"`
	AuthMethod string   `vici:"auth"` // (psk|pubkey)
	Certs      []string `vici:"certs,omitempty"`
}

type childSAConf struct {
	LocalTS      []string `vici:"local_ts"`
	RemoteTS     []string `vici:"remote_ts"`
	StartAction  string   `vici:"start_action"`         //none,trap,start
	CloseAction  string   `vici:"close_action"`         //none,clear,hold,restart
	DpdAction    string   `vici:"dpd_action,omitempty"` //none,clear,hold,restart
	ESPProposals []string `vici:"esp_proposals,omitempty"`
}

func New(opts ...option) (*StrongSwanManager, error) {
	manager := &StrongSwanManager{
		socketPath:       "/var/run/charon.vici",
		certsPath:        filepath.Join("/etc/ipsec.d", "certs"),
		startAction:      "none",
		connectionByName: make(map[string]tunnel.ConnConfig),
		mu:               &sync.RWMutex{},
	}

	for _, opt := range opts {
		opt(manager)
	}

	go func() {
		for {
			manager.checkConnections()
			time.Sleep(5 * time.Second)
		}
	}()

	return manager, nil
}

func (m StrongSwanManager) ListConnNames() ([]string, error) {
	var names []string

	err := m.do(func(session *vici.Session) error {
		msg, err := session.CommandRequest("get-conns", vici.NewMessage())
		if err != nil {
			return err
		}

		names = msg.Get("conns").([]string)
		return nil
	})

	return names, err
}

func (m StrongSwanManager) IsRunning() bool {
	err := m.do(func(session *vici.Session) error {
		_, err := session.CommandRequest("stats", vici.NewMessage())
		return err
	})

	return err == nil
}

func (m StrongSwanManager) InitiateConn(name string) error {
	conn, found := m.getConnection(name)
	if !found {
		return fmt.Errorf("connection %s not found", name)
	}

	// mediation connection don't have any child sa, so just initiate the SA
	if conn.Mediation {
		initiated, err := m.IsSAInitiated(name)
		if err != nil {
			return err
		}
		if initiated {
			return nil
		}

		return m.initiateSA(name)
	}

	childSANames, err := m.listSANames(name)
	if err != nil {
		return err
	}

	if conn.NeedMediation {
		initiated, err := m.IsSAInitiated(conn.MediatedBy)
		if err != nil {
			return err
		}
		if !initiated {
			return fmt.Errorf("mediator SA %s is not initiated", conn.MediatedBy)
		}
	}

	childNames := []string{
		fmt.Sprintf("%s-p2p", name),
		fmt.Sprintf("%s-p2n", name),
		fmt.Sprintf("%s-n2p", name),
	}

	for _, child := range childNames {
		if childSANames.Has(child) {
			continue
		}
		if err = m.initiateChildSA(child); err != nil {
			return err
		}
	}

	return nil
}

func (m StrongSwanManager) IsSAInitiated(ike string) (bool, error) {
	request := vici.NewMessage()
	_ = request.Set("ike", ike)

	initiated := false
	err := m.do(func(session *vici.Session) error {
		ms, err := session.StreamedCommandRequest("list-sas", "list-sa", request)
		if err != nil {
			return err
		}

		for _, msg := range ms.Messages() {
			if err = msg.Err(); err != nil {
				return err
			} else {
				initiated = len(msg.Keys()) > 0
			}
			break
		}

		return nil
	})

	return initiated, err
}

func (m StrongSwanManager) listSANames(ike string) (sets.String, error) {
	names := sets.NewString()

	request := vici.NewMessage()
	_ = request.Set("ike", ike)

	err := m.do(func(session *vici.Session) error {
		ms, err := session.StreamedCommandRequest("list-sas", "list-sa", request)
		if err != nil {
			return err
		}
		for _, msg := range ms.Messages() {
			if err = msg.Err(); err != nil {
				return err
			} else {
				for _, name := range msg.Keys() {
					children := msg.Get(name).(*vici.Message).Get("child-sas").(*vici.Message)
					for _, child := range children.Keys() {
						names.Insert(children.Get(child).(*vici.Message).Get("name").(string))
					}
				}
			}
		}
		return nil
	})

	return names, err
}

func (m StrongSwanManager) initiateChildSA(child string) error {
	return m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		_ = msg.Set("child", child)
		if m.initTimeout != 0 {
			_ = msg.Set("timeout", m.initTimeout)
		}
		if _, err := session.CommandRequest("initiate", msg); err != nil {
			return err
		}
		return nil
	})
}

func (m StrongSwanManager) initiateSA(name string) error {
	return m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		_ = msg.Set("ike", name)

		if m.initTimeout != 0 {
			_ = msg.Set("timeout", m.initTimeout)
		}
		if _, err := session.CommandRequest("initiate", msg); err != nil {
			return err
		}
		return nil
	})
}

func (m StrongSwanManager) IsActive() (bool, error) {
	var active bool
	err := m.do(func(session *vici.Session) error {
		msg, err := session.CommandRequest("stats", vici.NewMessage())
		if err != nil {
			return err
		}

		msg = msg.Get("ikesas").(*vici.Message)

		total, _ := strconv.Atoi(msg.Get("total").(string))
		active = total > 0

		return nil
	})

	return active, err
}

func (m StrongSwanManager) LoadConn(cnf tunnel.ConnConfig) error {
	certs, err := m.getCerts(cnf.LocalCerts)
	if err != nil {
		return err
	}

	conn := connection{
		LocalAddrs:  cnf.LocalAddress,
		RemoteAddrs: cnf.RemoteAddress,
		IF_ID_IN:    m.interfaceID,
		IF_ID_OUT:   m.interfaceID,
		LocalAuth: authConf{
			ID:         cnf.LocalID,
			AuthMethod: "pubkey",
			Certs:      certs,
		},

		DpdDelay: m.dpdDelay,
	}

	if cnf.RemoteID != "" {
		conn.RemoteAuth = authConf{
			ID:         cnf.RemoteID,
			AuthMethod: "pubkey",
		}
	}

	if cnf.Mediation {
		conn.Mediation = "yes"
		// no child and no remote auth for mediation connection
	} else {
		if cnf.NeedMediation {
			conn.MediatedBy = cnf.MediatedBy
			// although mediation_peer can be omitted, but strongswan has a bug which make
			// mediation_peer is necessary
			// https://github.com/strongswan/strongswan/discussions/1569
			conn.MediationPeer = cnf.MediationPeer

			// when use mediation, remote addresses are not needed, because the real remote addresses are
			// different from the remote addresses in tunnel config
			conn.LocalAddrs = nil
			conn.RemoteAddrs = nil
		}

		conn.Children = map[string]childSAConf{
			fmt.Sprintf("%s-p2p", cnf.Name): {
				LocalTS:     cnf.LocalSubnets,
				RemoteTS:    cnf.RemoteSubnets,
				StartAction: m.startAction,
				DpdAction:   m.dpdAction,
			},
			fmt.Sprintf("%s-n2p", cnf.Name): {
				LocalTS:     cnf.LocalNodeSubnets,
				RemoteTS:    cnf.RemoteSubnets,
				StartAction: m.startAction,
				DpdAction:   m.dpdAction,
			},
			fmt.Sprintf("%s-p2n", cnf.Name): {
				LocalTS:     cnf.LocalSubnets,
				RemoteTS:    cnf.RemoteNodeSubnets,
				StartAction: m.startAction,
				DpdAction:   m.dpdAction,
			},
		}
	}

	if cnf.RemotePort != nil && *cnf.RemotePort != 500 {
		// https://docs.strongswan.org/docs/5.9/features/natTraversal.html
		// By default local_port is 500, but when remote_port is
		// not 500, local_port also should use non-500, it can be 4500 or other number,
		// here, I choose 4500 which is default nat-t port.
		localPort := uint(4500)
		conn.RemotePort = cnf.RemotePort
		conn.LocalPort = &localPort
	}

	oldConn, found := m.getConnection(cnf.Name)
	if found {
		if reflect.DeepEqual(cnf, oldConn) {
			return nil
		}

		// we call UnloadConn to remove old Connection in strongswan, but if it failed, we ignore it
		// because the failure won't cause trouble for loadConn
		_ = m.UnloadConn(cnf.Name)
	}

	err = m.loadConn(cnf.Name, conn)
	if err == nil {
		m.rememberConn(cnf)
	}
	return err
}

func (m StrongSwanManager) loadConn(name string, conn connection) error {
	return m.do(func(session *vici.Session) error {
		c, err := vici.MarshalMessage(conn)
		if err != nil {
			return err
		}

		msg := vici.NewMessage()
		_ = msg.Set(name, c)

		_, err = session.CommandRequest("load-conn", msg)
		return err
	})
}

func (m StrongSwanManager) terminateSA(name string) error {
	return m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		_ = msg.Set("ike", name)

		_, err := session.CommandRequest("terminate", msg)
		return err
	})
}

func (m StrongSwanManager) UnloadConn(name string) error {
	m.forgetConn(name)

	err := m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		_ = msg.Set("name", name)

		_, err := session.CommandRequest("unload-conn", msg)
		return err
	})
	if err != nil {
		return err
	}

	return m.terminateSA(name)
}

// checkConnections will remove any in-memory connection from manager if
// its counterpart does not exist in strongswan, this will keep StrongswanManager
// keep in sync with strongswan in a certain extent
func (m StrongSwanManager) checkConnections() {
	names, err := m.ListConnNames()
	// If error happens, skip checking this time. Normally list-conns won't return error,
	// unless strongswan is not running
	if err != nil {
		return
	}
	nameSet := sets.NewString(names...)

	m.mu.Lock()
	m.mu.Unlock()
	for name := range m.connectionByName {
		if !nameSet.Has(name) {
			delete(m.connectionByName, name)
		}
	}
}

func (m StrongSwanManager) do(fn func(session *vici.Session) error) error {
	session, err := vici.NewSession(vici.WithSocketPath(m.socketPath))
	if err != nil {
		return err
	}
	defer session.Close()

	return fn(session)
}

func (m StrongSwanManager) getCerts(filenames []string) ([]string, error) {
	var certs []string
	for _, filename := range filenames {
		cert, err := m.getCert(filename)
		if err != nil {
			return certs, err
		}

		certs = append(certs, cert)
	}

	return certs, nil
}

func (m StrongSwanManager) getCert(filename string) (string, error) {
	if !strings.HasPrefix(filename, "/") {
		filename = filepath.Join(m.certsPath, filename)
	}

	raw, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	block, _ := pem.Decode(raw)
	pemBytes := pem.EncodeToMemory(block)

	return string(pemBytes), nil
}

func (m StrongSwanManager) rememberConn(conn tunnel.ConnConfig) {
	m.mu.Lock()
	m.mu.Unlock()

	m.connectionByName[conn.Name] = conn
}

func (m StrongSwanManager) forgetConn(name string) {
	m.mu.Lock()
	m.mu.Unlock()

	delete(m.connectionByName, name)
}

func (m StrongSwanManager) getConnection(name string) (tunnel.ConnConfig, bool) {
	m.mu.RLock()
	m.mu.RUnlock()

	conn, found := m.connectionByName[name]
	return conn, found
}
