// Copyright 2021 BoCloud
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

package iptables

import (
	"github.com/coreos/go-iptables/iptables"
)

const (
	TableFilter  = "filter"
	ChainForward = "FORWARD"
	ChainFabEdge = "FABEDGE"
)

func EnsureIPTablesRules(cidr string) error {
	ipt, err := iptables.New()
	if err != nil {
		return err
	}

	// ensure chain exists
	exists, err := ipt.ChainExists(TableFilter, ChainFabEdge)
	if err != nil {
		return err
	}

	if !exists {
		return ipt.NewChain(TableFilter, ChainFabEdge)
	}

	// ensure rules exist
	if err = ipt.AppendUnique(TableFilter, ChainForward, "-j", ChainFabEdge); err != nil {
		return err
	}

	if err = ipt.AppendUnique(TableFilter, ChainFabEdge, "-s", cidr, "-j", "ACCEPT"); err != nil {
		return err
	}

	if err = ipt.AppendUnique(TableFilter, ChainFabEdge, "-d", cidr, "-j", "ACCEPT"); err != nil {
		return err
	}

	return nil
}
