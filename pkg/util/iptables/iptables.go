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

package iptables

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	utilexec "k8s.io/utils/exec"
)

type Table = utiliptables.Table

type Chain = utiliptables.Chain

type RulePosition = utiliptables.RulePosition

const (
	Append  RulePosition = utiliptables.Append
	Prepend              = utiliptables.Prepend
)

type FlushFlag = utiliptables.FlushFlag

const (
	// FlushTables a boolean true constant for option flag FlushFlag
	FlushTables FlushFlag = true

	// NoFlushTables a boolean false constant for option flag FlushFlag
	NoFlushTables FlushFlag = false
)

// RestoreCountersFlag is an option flag for Restore
type RestoreCountersFlag = utiliptables.RestoreCountersFlag

const (
	RestoreCounters   RestoreCountersFlag = true
	NoRestoreCounters RestoreCountersFlag = false
)

type JumpChain struct {
	Table    Table
	SrcChain Chain
	DstChain Chain
	Position RulePosition
}

type Interface interface {
	utiliptables.Interface
	// CreateChains create custom chains and insert them in specified positions
	CreateChains(chains []JumpChain) error
	// FlushAllChains flush rules of all custom chains
	FlushAllChains(chains []JumpChain) error
	// RemoveAllChains flush rules of all custom chains and remove them all
	RemoveAllChains(chains []JumpChain) error
	// RemoveChain flush rules of specified chain from specified table and remove the chain
	RemoveChain(table Table, chain Chain) error
	// NewApplierCleaner create a ApplierCleaner with specified custom chains and rules
	NewApplierCleaner(chains []JumpChain, rulesData []byte) ApplierCleaner
}

// ApplierCleaner is used to apply or clean custom chains and rules
type ApplierCleaner interface {
	// Apply will create custom chains and insert rules
	Apply() error
	// Remove flush rules of all custom chains and remove them all
	Remove() error
	// Flush rules of all custom chains
	Flush() error
}

type iptablesHelper struct {
	utiliptables.Interface
	ipt utiliptables.Interface
}

type applierCleaner struct {
	helper    Interface
	chains    []JumpChain
	rulesData []byte
}

func NewIPTablesHelper(protocol Protocol) Interface {
	execer := utilexec.New()
	ipt := utiliptables.New(execer, protocol)
	return &iptablesHelper{
		ipt:       utiliptables.New(execer, protocol),
		Interface: ipt,
	}
}

func NewApplierCleaner(protocol Protocol, chains []JumpChain, rulesData []byte) ApplierCleaner {
	return &applierCleaner{
		helper:    NewIPTablesHelper(protocol),
		chains:    chains,
		rulesData: rulesData,
	}
}

func (h *iptablesHelper) CreateChains(chains []JumpChain) error {
	var errors []error

	for _, chain := range chains {
		if _, err := h.ipt.EnsureChain(chain.Table, chain.DstChain); err != nil {
			errors = append(errors, err)
			continue
		}

		_, err := h.ipt.EnsureRule(chain.Position, chain.Table, chain.SrcChain, "-j", string(chain.DstChain))
		if err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (h *iptablesHelper) RemoveAllChains(chains []JumpChain) error {
	var errors []error

	for _, chain := range chains {
		if err := h.RemoveChain(chain.Table, chain.DstChain); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (h *iptablesHelper) FlushAllChains(chains []JumpChain) error {
	var errors []error

	for _, chain := range chains {
		if err := h.ipt.FlushChain(chain.Table, chain.DstChain); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (h *iptablesHelper) RemoveChain(table Table, chain Chain) error {
	if err := h.FlushChain(table, chain); err != nil {
		return err
	}

	return h.ipt.DeleteChain(table, chain)
}

func (h *iptablesHelper) NewApplierCleaner(chains []JumpChain, rulesData []byte) ApplierCleaner {
	return &applierCleaner{
		helper:    h,
		chains:    chains,
		rulesData: rulesData,
	}
}

func (ac *applierCleaner) Apply() error {
	if err := ac.helper.CreateChains(ac.chains); err != nil {
		return err
	}

	return ac.helper.RestoreAll(ac.rulesData, NoFlushTables, RestoreCounters)
}

func (ac *applierCleaner) Flush() error {
	return ac.helper.FlushAllChains(ac.chains)
}

func (ac *applierCleaner) Remove() error {
	return ac.helper.RemoveAllChains(ac.chains)
}
