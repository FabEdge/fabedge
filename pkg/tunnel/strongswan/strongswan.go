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
	"github.com/jjeffery/stringset"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/strongswan/govici/vici"
)

var _ tunnel.Manager = &StrongSwanManager{}

type StrongSwanManager struct {
	socketPath  string
	certsPath   string
	startAction string
	interfaceID *uint
}

type connection struct {
	LocalAddrs  []string               `vici:"local_addrs"`
	RemoteAddrs []string               `vici:"remote_addrs,omitempty"`
	Proposals   []string               `vici:"proposals,omitempty"`
	Encap       string                 `vici:"encap"` //yes,no
	DPDDelay    string                 `json:"dpd_delay,omitempty"`
	LocalAuth   authConf               `vici:"local"`
	RemoteAuth  authConf               `vici:"remote"`
	Children    map[string]childSAConf `vici:"children"`
	IF_ID_IN    *uint                  `vici:"if_id_in"`
	IF_ID_OUT   *uint                  `vici:"if_id_out"`
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
		socketPath:  "/var/run/charon.vici",
		certsPath:   filepath.Join("/etc/ipsec.d", "certs"),
		startAction: "trap",
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

func (m StrongSwanManager) listSANames(ike string) (stringset.Set, error) {
	var names stringset.Set

	request := vici.NewMessage()
	if err := request.Set("ike", ike); err != nil {
		return names, err
	}

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
						names.Add(children.Get(child).(*vici.Message).Get("name").(string))
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
		if childSANames.Contains(child) {
			continue
		}
		if err = m.initiateSA(&name, &child); err != nil {
			return err
		}
	}

	return nil
}

func (m StrongSwanManager) initiateSA(ike, child *string) error {
	msg := vici.NewMessage()

	if err := msg.Set("ike", *ike); err != nil {
		return err
	}

	if child != nil {
		if err := msg.Set("child", child); err != nil {
			return err
		}
	}

	return m.do(func(session *vici.Session) error {
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
		Children: map[string]childSAConf{
			fmt.Sprintf("%s-p2p", cnf.Name): {
				LocalTS:     cnf.LocalSubnets,
				RemoteTS:    cnf.RemoteSubnets,
				StartAction: m.startAction,
				DpdAction:   "restart",
				CloseAction: "start",
			},
			fmt.Sprintf("%s-n2p", cnf.Name): {
				LocalTS:     cnf.LocalNodeSubnets,
				RemoteTS:    cnf.RemoteSubnets,
				StartAction: m.startAction,
				DpdAction:   "restart",
				CloseAction: "start",
			},
			fmt.Sprintf("%s-p2n", cnf.Name): {
				LocalTS:     cnf.LocalSubnets,
				RemoteTS:    cnf.RemoteNodeSubnets,
				StartAction: m.startAction,
				DpdAction:   "restart",
				CloseAction: "start",
			},
		},
	}

	c, err := vici.MarshalMessage(conn)
	if err != nil {
		return err
	}

	msg := vici.NewMessage()
	if err := msg.Set(cnf.Name, c); err != nil {
		return err
	}

	return m.do(func(session *vici.Session) error {
		_, err := session.CommandRequest("load-conn", msg)
		return err
	})
}

func (m StrongSwanManager) UnloadConn(name string) error {
	return m.do(func(session *vici.Session) error {
		msg := vici.NewMessage()
		if err := msg.Set("name", name); err != nil {
			return err
		}

		_, err := session.CommandRequest("unload-conn", msg)
		return err
	})
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
