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

package agent

import (
	"encoding/json"
	"net"
	"time"
)

func (m *Manager) broadcastEndpoint() {
	addr, err := net.ResolveUDPAddr("udp", m.MulticastAddress)
	if err != nil {
		m.log.Error(err, "failed to resolve broadcast address")
		return
	}

	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		m.log.Error(err, "failed to dial UDP")
		return
	}

	send := func() {
		current := m.getCurrentEndpoint().Endpoint
		if current.Name != "" {
			msg := Message{
				Endpoint: current,
				Token:    m.MulticastToken,
			}

			msgBytes, err := json.Marshal(&msg)
			if err != nil {
				m.log.Error(err, "failed to marshal broadcast message", "message", msg)
				return
			}

			if _, err = c.Write(msgBytes); err != nil {
				m.log.Error(err, "failed to broadcast message", "message", msg)
			}
		}
	}

	for {
		send()
		time.Sleep(m.MulticastInterval)
	}
}

func (m *Manager) receiveEndpoint() {
	addr, err := net.ResolveUDPAddr("udp", m.MulticastAddress)
	if err != nil {
		m.log.Error(err, "failed to resolve UDP address")
		return
	}

	const maxDatagramSize = 8192
	l, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		m.log.Error(err, "failed to listen on address", "address", m.MulticastAddress)
		return
	}

	if err = l.SetReadBuffer(maxDatagramSize); err != nil {
		m.log.Error(err, "failed to set ReadBuffer")
		return
	}

	for {
		buffer := make([]byte, maxDatagramSize)
		n, src, err := l.ReadFromUDP(buffer)
		if err != nil {
			m.log.Error(err, "Failed to read from UDP")
			continue
		}

		var msg Message
		if err = json.Unmarshal(buffer[:n], &msg); err != nil {
			m.log.Error(err, "failed to unmarshal message", "message", string(buffer[:n]), "source", src.String())
			continue
		}

		m.log.V(5).Info("An endpoint is received", "message", msg, "source", src.String())
		if m.getCurrentEndpoint().Name == msg.Name {
			m.log.V(5).Info("The endpoint is from current agent, skip it", "message", msg, "source", src.String())
			continue
		}

		if msg.Token != m.MulticastToken {
			m.log.V(5).Info("The token of message is not matched, skip it", "message", msg, "source", src.String())
			continue
		}

		m.log.V(5).Info("Message is saved", "message", msg, "source", src.String())
		func() {
			m.endpointLock.Lock()
			defer m.endpointLock.Unlock()

			m.peerEndpoints[msg.Name] = Endpoint{
				Endpoint:   msg.Endpoint,
				IsLocal:    true,
				ExpireTime: time.Now().Add(m.EndpointTTL),
			}
		}()
	}
}
