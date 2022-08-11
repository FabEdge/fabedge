package net

import (
	"net"
	"strings"
)

// IsIPv6 returns if netIP is IPv6.
func IsIPv6(netIP net.IP) bool {
	return netIP != nil && netIP.To4() == nil
}

// IsIPv6String returns if ip is IPv6.
func IsIPv6String(ip string) bool {
	netIP := net.ParseIP(ip)
	return IsIPv6(netIP)
}

// IsIPv6CIDRString returns if cidr is IPv6.
// This assumes cidr is a valid CIDR.
func IsIPv6CIDRString(cidr string) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	return IsIPv6(ip)
}

// IsIPv6OrIPv6CIDRString returns if netIP is IPv6 family.
func IsIPv6OrIPv6CIDRString(ipOrCIDR string) bool {
	if strings.IndexByte(ipOrCIDR, ':') == -1 {
		return false
	}

	return IsIPv6String(ipOrCIDR) || IsIPv6CIDRString(ipOrCIDR)
}

// IsIPv6CIDR returns if a cidr is ipv6
func IsIPv6CIDR(cidr *net.IPNet) bool {
	ip := cidr.IP
	return IsIPv6(ip)
}

// IsIPv4 returns if netIP is IPv4.
func IsIPv4(netIP net.IP) bool {
	return netIP != nil && netIP.To4() != nil
}

// IsIPv4String returns if ip is IPv4.
func IsIPv4String(ip string) bool {
	netIP := net.ParseIP(ip)
	return IsIPv4(netIP)
}

// IsIPv4CIDR returns if a cidr is ipv4
func IsIPv4CIDR(cidr *net.IPNet) bool {
	ip := cidr.IP
	return IsIPv4(ip)
}

// IsIPv4CIDRString returns if cidr is IPv4.
// This assumes cidr is a valid CIDR.
func IsIPv4CIDRString(cidr string) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	return IsIPv4(ip)
}

// IsIPv4OrIPv4CIDRString returns if netIP is IPv4 family.
func IsIPv4OrIPv4CIDRString(ipOrCIDR string) bool {
	if strings.IndexByte(ipOrCIDR, ':') == -1 {
		return false
	}

	return IsIPv6String(ipOrCIDR) || IsIPv6CIDRString(ipOrCIDR)
}

func IsCompatible(ip *net.IPNet, ipNet net.IP) bool {
	return (IsIPv4CIDR(ip) && IsIPv4(ipNet)) || (IsIPv6CIDR(ip) && IsIPv6(ipNet))
}

func HasIPv6CIDRString(cidrs []string) bool {
	for _, cidr := range cidrs {
		if IsIPv6CIDRString(cidr) {
			return true
		}
	}

	return false
}
