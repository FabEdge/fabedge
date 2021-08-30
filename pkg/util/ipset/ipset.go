package ipset

import (
	"fmt"
	"net"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/exec"

	"github.com/fabedge/fabedge/third_party/ipset"
)

const (
	HashIP  = ipset.HashIP
	HashNet = ipset.HashNet
)

type Interface interface {
	EnsureIPSet(setName string, setType ipset.Type) (*ipset.IPSet, error)
	AddIPSetEntry(set *ipset.IPSet, ip string, setType ipset.Type) error
	DelIPSetEntry(set *ipset.IPSet, ip string, setType ipset.Type) error
	ListEntries(setName string, setType ipset.Type) (sets.String, error)
	SyncIPSetEntries(ipsetObj *ipset.IPSet, allIPSetEntrySet, oldIPSetEntrySet sets.String) error
	ConvertIPToCIDR(ip string) string
}

type execer struct {
	ipset ipset.Interface
}

func New() Interface {
	return &execer{
		ipset: ipset.New(exec.New()),
	}
}

func (e *execer) EnsureIPSet(setName string, setType ipset.Type) (*ipset.IPSet, error) {
	set := &ipset.IPSet{
		Name:    setName,
		SetType: setType,
	}
	if err := e.ipset.CreateSet(set, true); err != nil {
		return nil, err
	}
	return set, nil
}

func (e *execer) AddIPSetEntry(set *ipset.IPSet, ip string, setType ipset.Type) error {
	entry := &ipset.Entry{
		SetType: setType,
	}

	switch setType {
	case ipset.HashIP:
		entry.IP = ip
	case ipset.HashNet:
		entry.Net = ip
	}

	if !entry.Validate(set) {
		return fmt.Errorf("failed to validate ipset entry, ipset: %v, entry: %v", set, entry)
	}

	return e.ipset.AddEntry(entry.String(), set, true)
}

func (e *execer) DelIPSetEntry(set *ipset.IPSet, ip string, setType ipset.Type) error {
	entry := &ipset.Entry{
		SetType: setType,
	}

	switch setType {
	case ipset.HashIP:
		entry.IP = ip
	case ipset.HashNet:
		entry.Net = ip
	}

	if !entry.Validate(set) {
		return fmt.Errorf("failed to validate ipset entry, ipset: %v, entry: %v", set, entry)
	}

	return e.ipset.DelEntry(entry.String(), set.Name)
}

func (e *execer) ListEntries(setName string, setType ipset.Type) (sets.String, error) {
	entrySet := sets.NewString()
	entries, err := e.ipset.ListEntries(setName)
	if err != nil {
		return nil, err
	}

	if setType != ipset.HashNet {
		entrySet.Insert(entries...)
		return entrySet, nil
	}

	for _, entry := range entries {
		// translate the IP address to CIDR is needed
		// because hash:net ipset saves 10.20.8.4/32 to 10.20.8.4
		if _, _, err := net.ParseCIDR(entry); err != nil {
			entry = e.ConvertIPToCIDR(entry)
		}
		entrySet.Insert(entry)
	}

	return entrySet, nil
}

func (e *execer) SyncIPSetEntries(ipsetObj *ipset.IPSet, allIPSetEntrySet, oldIPSetEntrySet sets.String) error {
	needAddEdgePeerCIDRs := allIPSetEntrySet.Difference(oldIPSetEntrySet)
	for cidr := range needAddEdgePeerCIDRs {
		if err := e.AddIPSetEntry(ipsetObj, cidr, ipset.HashNet); err != nil {
			return err
		}
	}

	needDelEdgePeerCIDRs := oldIPSetEntrySet.Difference(allIPSetEntrySet)
	for cidr := range needDelEdgePeerCIDRs {
		if err := e.DelIPSetEntry(ipsetObj, cidr, ipset.HashNet); err != nil {
			return err
		}
	}
	return nil
}

func (e *execer) ConvertIPToCIDR(ip string) string {
	return strings.Join([]string{ip, "32"}, "/")
}
