// Copyright 2023 FabEdge Team
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

package vxlan

import (
	"bytes"
	"fmt"
	"github.com/fabedge/fabedge/pkg/tunnel"
	"github.com/vishvananda/netlink"
	"io"
	"net"
	"os/exec"
	"strconv"
)

var _ tunnel.Manager = &VxlanManager{}

type VxlanManager struct {
	MTU         int
	VNI         int
	Port        int
	VtepDevName string
	Connections map[string]VxlanUnicastConfig
}

type VxlanMulticastConfig struct {
	Name         string
	VtepAddress  string
	GroupAddress net.IP
}

type VxlanUnicastConfig struct {
	Name          string
	VtepAddress   string
	LocalAddress  string
	RemoteAddress string
}

func CreateVxlanManager(mtu, vni, port int, vtepDevName string) VxlanManager {
	return VxlanManager{
		MTU:         mtu,
		VNI:         vni,
		Port:        port,
		VtepDevName: vtepDevName,
		Connections: make(map[string]VxlanUnicastConfig),
	}
}

func (m VxlanManager) IsRunning() bool {
	return true
}

func (m VxlanManager) ListConnNames() ([]string, error) {
	var ret []string
	for _, value := range m.Connections {
		ret = append(ret, value.Name)
	}
	return ret, nil
}

func (m VxlanManager) LoadConn(conn tunnel.ConnConfig) error {
	connection := VxlanUnicastConfig{
		Name:          conn.Name,
		VtepAddress:   conn.EndpointAddress,
		LocalAddress:  conn.LocalAddress[0],
		RemoteAddress: conn.RemoteAddress[0],
	}
	m.Connections[connection.Name] = connection
	return nil
}

func (m VxlanManager) InitiateConn(name string) error {
	if connection, exist := m.Connections[name]; !exist {
		return fmt.Errorf("cannot find connection %s", name)
	} else {
		err := m.deleteVxlan(name)
		if err != nil {
			return err
		}
		err = m.createUnicastVxlan(connection)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m VxlanManager) UnloadConn(name string) error {
	if _, exist := m.Connections[name]; !exist {
		return fmt.Errorf("cannot find connection %s", name)
	} else {
		err := m.deleteVxlan(name)
		if err != nil {
			return err
		}
		delete(m.Connections, name)
	}
	return nil
}

func (m VxlanManager) IsActive() (bool, error) {
	return true, nil
}

func (m VxlanManager) createMulticastVxlan(config VxlanMulticastConfig) error {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = config.Name
	attrs.MTU = m.MTU
	link, err := netlink.LinkByName(m.VtepDevName)
	if err != nil {
		return err
	}

	vxlan := &netlink.Vxlan{
		LinkAttrs:    attrs,
		VxlanId:      m.VNI,
		Port:         m.Port,
		VtepDevIndex: link.Attrs().Index,
		Group:        config.GroupAddress,
	}

	err = netlink.LinkAdd(vxlan)
	if err != nil {
		return err
	}

	err = m.configAddress(vxlan, config.VtepAddress)
	if err != nil {
		return err
	}

	return nil
}

func (m VxlanManager) createUnicastVxlan(config VxlanUnicastConfig) error {
	args := []string{"link", "add", config.Name, "mtu", strconv.Itoa(m.MTU),
		"type", "vxlan", "id", strconv.Itoa(m.VNI),
		"dstport", strconv.Itoa(m.Port),
		"local", config.LocalAddress, "remote", config.RemoteAddress,
		"dev", m.VtepDevName}
	_, _, err := m.runCommand("ip", args, nil)
	if err != nil {
		return err
	}

	vxlan, err := netlink.LinkByName(config.Name)
	if err != nil {
		return err
	}

	err = m.configAddress(vxlan, config.VtepAddress)
	if err != nil {
		return err
	}
	return nil
}

func (m VxlanManager) configAddress(link netlink.Link, address string) error {
	err := netlink.LinkSetUp(link)
	if err != nil {
		return err
	}

	addr, err := netlink.ParseAddr(address)
	if err != nil {
		return err
	}

	err = netlink.AddrAdd(link, addr)
	if err != nil {
		return err
	}

	return nil
}

func (m VxlanManager) runCommand(command string, args []string, stdin io.Reader) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(command, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if stdin != nil {
		cmd.Stdin = stdin
	}

	if err := cmd.Run(); err != nil {
		return stdout.String(), stderr.String(), err
	}

	return stdout.String(), stderr.String(), nil
}

func (m VxlanManager) deleteVxlan(name string) error {
	link, err := netlink.LinkByName(name)
	if link == nil {
		return nil
	}

	if err != nil {
		return err
	}

	err = netlink.LinkSetDown(link)
	if err != nil {
		return err
	}

	err = netlink.LinkDel(link)
	if err != nil {
		return err
	}

	return nil
}
