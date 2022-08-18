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

package ipset

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/exec"

	"github.com/fabedge/fabedge/third_party/ipset"
)

type IPSet = ipset.IPSet

const (
	HashIP       = ipset.HashIP
	HashNet      = ipset.HashNet
	HashIPPortIP = ipset.HashIPPortIP

	ProtocolFamilyIPV4 = ipset.ProtocolFamilyIPV4
	ProtocolFamilyIPV6 = ipset.ProtocolFamilyIPV6
)

type Interface interface {
	// EnsureIPSet ensure specified are created and ensure this ipset only contains all entries
	// specified in entrySet.
	// Allowed SetType are HashNet and HashIP
	// Data in entrySet must be either an IP or a CIDR
	EnsureIPSet(set *ipset.IPSet, entrySet sets.String) error
}

type execer struct {
	ipset ipset.Interface
}

func New() Interface {
	return &execer{
		ipset: ipset.New(exec.New()),
	}
}

func (e *execer) EnsureIPSet(set *ipset.IPSet, allIPSetEntrySet sets.String) error {
	if err := e.ipset.CreateSet(set, true); err != nil {
		return err
	}

	oldIPSetEntrySet, err := e.ListEntries(set.Name)
	if err != nil {
		return err
	}

	needAddEntries := allIPSetEntrySet.Difference(oldIPSetEntrySet)
	for entry := range needAddEntries {
		if err := e.AddIPSetEntry(set, entry); err != nil {
			return err
		}
	}

	needDelEntries := oldIPSetEntrySet.Difference(allIPSetEntrySet)
	for entry := range needDelEntries {
		if err := e.DelIPSetEntry(set, entry); err != nil {
			return err
		}
	}
	return nil
}

func (e *execer) AddIPSetEntry(set *ipset.IPSet, ip string) error {
	entry, err := newEntry(set, ip)
	if err != nil {
		return err
	}

	return e.ipset.AddEntry(entry.String(), set, true)
}

func (e *execer) DelIPSetEntry(set *ipset.IPSet, ip string) error {
	entry, err := newEntry(set, ip)
	if err != nil {
		return err
	}

	return e.ipset.DelEntry(entry.String(), set.Name)
}

func (e *execer) ListEntries(setName string) (sets.String, error) {
	entries, err := e.ipset.ListEntries(setName)
	if err != nil {
		return nil, err
	}

	return sets.NewString(entries...), nil
}

func newEntry(set *ipset.IPSet, addr string) (entry *ipset.Entry, err error) {
	entry = &ipset.Entry{
		IP:      addr,
		SetType: set.SetType,
	}

	if set.SetType == HashIPPortIP {
		parts := strings.SplitN(addr, ",", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("%s not an valid hash:ip,port,ip entry", addr)
		}

		entry.IP, entry.IP2 = parts[0], parts[2]

		parts = strings.SplitN(parts[1], ":", 2)
		switch len(parts) {
		case 1:
			entry.Port, err = strconv.Atoi(parts[0])
		case 2:
			entry.Protocol = parts[0]
			entry.Port, err = strconv.Atoi(parts[1])
		default:
			return nil, fmt.Errorf("%s not an valid hash:ip,port,ip entry", addr)
		}
		if err != nil {
			return nil, err
		}
	}

	if !entry.Validate(set) {
		return nil, fmt.Errorf("failed to validate ipset entry, ipset: %v, entry: %v", set, entry)
	}

	return entry, nil
}
