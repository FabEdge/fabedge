package net

import "net"

type ProtocolVersion int

const (
	IPV4 ProtocolVersion = 4
	IPV6 ProtocolVersion = 6
)

func IPVersion(ip net.IP) ProtocolVersion {
	if ip.To4() == nil {
		return IPV6
	}
	return IPV4
}
