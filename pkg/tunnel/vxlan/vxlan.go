package vxlan

import (
	"bytes"
	"github.com/vishvananda/netlink"
	"io"
	"net"
	"os/exec"
	"strconv"
)

type VxlanMulticastConfig struct {
	Name         string
	MTU          int
	VNI          int
	Port         int
	VtepDevName  string
	VtepAddress  string
	GroupAddress net.IP
}

type VxlanUnicastConfig struct {
	Name          string
	MTU           int
	VNI           int
	Port          int
	VtepDevName   string
	VtepAddress   string
	LocalAddress  string
	RemoteAddress string
}

func createMulticastVxlan(config VxlanMulticastConfig) error {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = config.Name
	attrs.MTU = config.MTU
	link, err := netlink.LinkByName(config.VtepDevName)
	if err != nil {
		return err
	}

	vxlan := &netlink.Vxlan{
		LinkAttrs:    attrs,
		VxlanId:      config.VNI,
		Port:         config.Port,
		VtepDevIndex: link.Attrs().Index,
		Group:        config.GroupAddress,
	}

	err = netlink.LinkAdd(vxlan)
	if err != nil {
		return err
	}

	err = configAddress(vxlan, config.VtepAddress)
	if err != nil {
		return err
	}

	return nil
}

func createUnicastVxlan(config VxlanUnicastConfig) error {
	args := []string{"link", "add", config.Name, "mtu", strconv.Itoa(config.MTU),
		"type", "vxlan", "id", strconv.Itoa(config.VNI),
		"dstport", strconv.Itoa(config.Port),
		"local", config.LocalAddress, "remote", config.RemoteAddress,
		"dev", config.VtepDevName}
	_, _, err := runCommand("ip", args, nil)
	if err != nil {
		return err
	}

	vxlan, err := netlink.LinkByName(config.Name)
	if err != nil {
		return err
	}

	err = configAddress(vxlan, config.VtepAddress)
	if err != nil {
		return err
	}
	return nil
}

func configAddress(link netlink.Link, address string) error {
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

func runCommand(command string, args []string, stdin io.Reader) (string, string, error) {
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

func deleteVxlan(name string) error {
	link, err := netlink.LinkByName(name)
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
