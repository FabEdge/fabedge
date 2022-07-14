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

	"github.com/strongswan/govici/vici"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/fabedge/fabedge/pkg/tunnel"
)

var errConnectionNotFound = fmt.Errorf("no connection found")

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
}

type connection struct {
	LocalAddrs  []string               `vici:"local_addrs"`
	RemoteAddrs []string               `vici:"remote_addrs,omitempty"`
	LocalAuth   authConf               `vici:"local"`
	RemoteAuth  authConf               `vici:"remote"`
	Children    map[string]childSAConf `vici:"children"`
	IF_ID_IN    *uint                  `vici:"if_id_in"`
	IF_ID_OUT   *uint                  `vici:"if_id_out"`
	DpdDelay    string                 `vici:"dpd_delay,omitempty"`
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

// loadedConnection is used to take data from list-conns direct.
// This struct will not take all fields of connection but only
// addresses returned by list-conns
type loadedConnection struct {
	LocalAddrs  []string                     `vici:"local_addrs"`
	RemoteAddrs []string                     `vici:"remote_addrs,omitempty"`
	Children    map[string]loadedChildSAConf `vici:"children"`
}

type loadedChildSAConf struct {
	LocalTS  []string `vici:"local-ts"`
	RemoteTS []string `vici:"remote-ts"`
}

func New(opts ...option) (*StrongSwanManager, error) {
	manager := &StrongSwanManager{
		socketPath:  "/var/run/charon.vici",
		certsPath:   filepath.Join("/etc/ipsec.d", "certs"),
		startAction: "none",
	}

	for _, opt := range opts {
		opt(manager)
	}

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

func (m StrongSwanManager) InitiateConn(name string) error {
	childSANames, err := m.listSANames(name)
	if err != nil {
		return err
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

func (m StrongSwanManager) initiateChildSA(child string) error {
	return m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		_ = msg.Set("child", child)
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
		RemoteAuth: authConf{
			ID:         cnf.RemoteID,
			AuthMethod: "pubkey",
		},
		DpdDelay: m.dpdDelay,
		Children: map[string]childSAConf{
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
		},
	}

	loadedConn, err := m.getConn(cnf.Name)
	switch {
	case err == nil:
		if areConnectionsIdentical(conn, loadedConn) {
			return nil
		}
		// we call UnloadConn to remove old Connection in strongswan, but if it failed, we ignore it
		// because the failure won't cause trouble for loadConn
		_ = m.UnloadConn(cnf.Name)

		return m.loadConn(cnf.Name, conn)
	case err == errConnectionNotFound:
		return m.loadConn(cnf.Name, conn)
	default:
		return err
	}
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

func (m StrongSwanManager) getConn(name string) (conn loadedConnection, err error) {
	err = m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		_ = msg.Set("ike", name)

		streamMsg, err := session.StreamedCommandRequest("list-conns", "list-conn", msg)
		if err != nil {
			return err
		}

		msgs := streamMsg.Messages()
		if len(msgs) == 0 {
			return errConnectionNotFound
		}
		connMsg := msgs[0]
		if connMsg.Get(name) == nil {
			return errConnectionNotFound
		}

		connMsg = connMsg.Get(name).(*vici.Message)
		return vici.UnmarshalMessage(connMsg, &conn)
	})

	if err != nil {
		return conn, err
	}

	// if LocalAddrs or RemoteAddrs is ["%any"], then we take it as nil
	if len(conn.LocalAddrs) == 1 && conn.LocalAddrs[0] == "%any" {
		conn.LocalAddrs = nil
	}

	if len(conn.RemoteAddrs) == 1 && conn.RemoteAddrs[0] == "%any" {
		conn.RemoteAddrs = nil
	}

	return conn, err
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

// we take connection as identical to a loadedConnection if their LocalAddrs and RemoteAddrs are the same
//  and there children's LocalTS and RemoteTS are the same
func areConnectionsIdentical(c1 connection, c2 loadedConnection) bool {
	if !reflect.DeepEqual(c1.LocalAddrs, c2.LocalAddrs) {
		return false
	}

	if !reflect.DeepEqual(c1.RemoteAddrs, c2.RemoteAddrs) {
		return false
	}

	if len(c1.Children) != len(c2.Children) {
		return false
	}

	for name, sc1 := range c1.Children {
		sc2, ok := c2.Children[name]
		if !ok {
			return false
		}

		if !areSubnetIdentical(sc1.LocalTS, sc2.LocalTS) {
			return false
		}

		if !areSubnetIdentical(sc1.RemoteTS, sc1.RemoteTS) {
			return false
		}
	}

	return true
}

func areSubnetIdentical(cidrs1, cidrs2 []string) bool {
	if len(cidrs1) != len(cidrs2) {
		return false
	}

	for i := range cidrs1 {
		cidr1 := normalizeCIDR(cidrs1[i])
		cidr2 := normalizeCIDR(cidrs2[i])

		if cidr1 != cidr2 {
			return false
		}
	}

	return true
}

func normalizeCIDR(value string) string {
	if strings.IndexByte(value, '/') > -1 {
		return value
	}

	maskLen := 32
	if strings.IndexByte(value, ':') > -1 {
		maskLen = 64
	}

	return fmt.Sprintf("%s/%d", value, maskLen)
}
