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

type Protocol = utiliptables.Protocol

const (
	ProtocolIPv4 Protocol = "IPv4"
	ProtocolIPv6 Protocol = "IPv6"
)

type Table = utiliptables.Table

const (
	TableFilter Table = "filter"
	TableNat    Table = "nat"
)

type Chain = utiliptables.Chain

const (
	ChainInput       Chain = "INPUT"
	ChainForward     Chain = "FORWARD"
	ChainPostRouting Chain = "POSTROUTING"
)

const (
	ChainFabEdgeInput       Chain = "FABEDGE-INPUT"
	ChainFabEdgeForward     Chain = "FABEDGE-FORWARD"
	ChainFabEdgePostRouting Chain = "FABEDGE-POSTROUTING"
)

type RulePosition = utiliptables.RulePosition

const (
	Append  RulePosition = utiliptables.Append
	Prepend RulePosition = utiliptables.Prepend
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
	// SafeFlushChain flush rules of all custom chains, it won't return error if chain doesn't exist
	SafeFlushChain(table Table, chain Chain) error
	// FlushChains flush rules of all custom chains
	FlushChains(chains []JumpChain) error
	// DeleteChains flush rules of all custom chains and remove them all
	DeleteChains(chains []JumpChain) error
	// SafeDeleteChain flush rules of specified chain from specified table and delete the chain,
	// it won't return error if chain doesn't exist
	SafeDeleteChain(chain JumpChain) error
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

func (h *iptablesHelper) DeleteChains(chains []JumpChain) error {
	var errors []error

	for _, chain := range chains {
		if err := h.SafeDeleteChain(chain); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (h *iptablesHelper) FlushChains(chains []JumpChain) error {
	var errors []error

	for _, chain := range chains {
		if err := h.SafeFlushChain(chain.Table, chain.DstChain); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (h *iptablesHelper) SafeFlushChain(table Table, chain Chain) error {
	exists, err := h.ChainExists(table, chain)
	if exists {
		return h.ipt.FlushChain(table, chain)
	} else if err != nil && isNotFoundError(err) {
		return nil
	} else {
		return err
	}
}

func (h *iptablesHelper) SafeDeleteChain(chain JumpChain) error {
	exists, err := h.ChainExists(chain.Table, chain.DstChain)
	if exists {
		if err = h.DeleteRule(chain.Table, chain.SrcChain, "-j", string(chain.DstChain)); err != nil {
			return err
		}

		if err = h.FlushChain(chain.Table, chain.DstChain); err != nil {
			return err
		}

		return h.ipt.DeleteChain(chain.Table, chain.DstChain)
	} else if err != nil && isNotFoundError(err) {
		return nil
	} else {
		return err
	}
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
	return ac.helper.FlushChains(ac.chains)
}

func (ac *applierCleaner) Remove() error {
	return ac.helper.DeleteChains(ac.chains)
}

func isNotFoundError(err error) bool {
	if utiliptables.IsNotFoundError(err) {
		return true
	}

	if ee, isExitError := err.(utilexec.ExitError); isExitError {
		return ee.ExitStatus() == 1
	}
	return false
}
