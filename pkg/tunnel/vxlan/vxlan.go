package vxlan

import (
	"github.com/vishvananda/netlink"
	"net"
)

type VxlanConfig struct {
	Name         string
	MTU          int
	VNI          int
	Port         int
	VtepDevIndex int
	VtepAddress  string
	GroupAddress net.IP
}

func createVxlan(config VxlanConfig) error {
	attrs := netlink.NewLinkAttrs()
	attrs.Name = config.Name
	attrs.MTU = config.MTU

	vxlan := &netlink.Vxlan{
		LinkAttrs:    attrs,
		VxlanId:      config.VNI,
		Port:         config.Port,
		VtepDevIndex: config.VtepDevIndex,
		Group:        config.GroupAddress,
	}

	err := netlink.LinkAdd(vxlan)
	if err != nil {
		return err
	}

	err = netlink.LinkSetUp(vxlan)
	if err != nil {
		return err
	}

	addr, err := netlink.ParseAddr(config.VtepAddress)
	if err != nil {
		return err
	}

	err = netlink.AddrAdd(vxlan, addr)
	if err != nil {
		return err
	}

	return nil
}

func deleteVxlan(config VxlanConfig) error {
	link, err := netlink.LinkByName(config.Name)
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
