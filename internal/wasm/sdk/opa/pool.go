// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package opa

import (
	"bytes"
	"context"
	"fmt"
	"sync"
)

// pool maintains a pool of WebAssemly VM instances.
type pool struct {
	available      chan struct{}
	mutex          sync.Mutex
	initialized    bool
	closed         bool
	policy         []byte
	data           []byte
	memoryMinPages uint32
	memoryMaxPages uint32
	vms            []*vm // All current VM instances, acquired or not.
	acquired       []bool
	pendingReinit  *vm
	blockedReinit  chan struct{}
}

// newPool constructs a new pool with the pool and VM configuration provided.
func newPool(poolSize, memoryMinPages, memoryMaxPages uint32) *pool {
	available := make(chan struct{}, poolSize)
	for i := uint32(0); i < poolSize; i++ {
		available <- struct{}{}
	}

	return &pool{
		memoryMinPages: memoryMinPages,
		memoryMaxPages: memoryMaxPages,
		available:      available,
		vms:            make([]*vm, 0),
		acquired:       make([]bool, 0),
	}
}

// Acquire obtains a VM from the pool, waiting if all VMms are in use
// and building one as necessary. Returns either ErrNotReady or
// ErrInternal if an error.
func (p *pool) Acquire(ctx context.Context) (*vm, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.available:
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.initialized || p.closed {
		return nil, ErrNotReady
	}

	for i, vm := range p.vms {
		if !p.acquired[i] {
			p.acquired[i] = true
			return vm, nil
		}
	}

	policy, data := p.policy, p.data

	p.mutex.Unlock()
	vm, err := newVM(policy, data, p.memoryMinPages, p.memoryMaxPages)
	p.mutex.Lock()

	if err != nil {
		p.available <- struct{}{}
		return nil, fmt.Errorf("%v: %w", err, ErrInternal)
	}

	p.acquired = append(p.acquired, true)
	p.vms = append(p.vms, vm)
	return vm, nil
}

// Release releases the VM back to the pool.
func (p *pool) Release(vm *vm) {
	p.mutex.Lock()

	// If the policy data setting is waiting for this one, don't release it back to the general consumption.
	// Note the reinit is responsible for pushing to available channel once done with the VM.
	if vm == p.pendingReinit {
		p.mutex.Unlock()
		p.blockedReinit <- struct{}{}
		return
	}

	for i := range p.vms {
		if p.vms[i] == vm {
			p.acquired[i] = false
			p.mutex.Unlock()
			p.available <- struct{}{}
			return
		}
	}

	// VM instance not found anymore, hence pool reconfigured and can release the VM.

	p.mutex.Unlock()
	p.available <- struct{}{}

	vm.Close()
}

// Reset re-initializes the vms within the pool with the new policy
// and data. The re-initialization takes place atomically: all new vms
// are constructed in advance before touching the pool.  Returns
// either ErrNotReady, ErrInvalidPolicy or ErrInternal if an error
// occurs.
func (p *pool) SetPolicyData(policy []byte, data []byte) error {
	p.mutex.Lock()

	if !p.initialized {
		vm, err := newVM(policy, data, p.memoryMinPages, p.memoryMaxPages)
		if err == nil {
			p.initialized = true
			p.vms = append(p.vms, vm)
			p.acquired = append(p.acquired, false)
			p.policy, p.data = policy, data
		} else {
			err = fmt.Errorf("%v: %w", err, ErrInvalidPolicyOrData)
		}

		p.mutex.Unlock()
		return err
	}

	if p.closed {
		p.mutex.Unlock()
		return ErrNotReady
	}

	currentPolicy, currentData := p.policy, p.data
	p.mutex.Unlock()

	if bytes.Equal(policy, currentPolicy) && bytes.Equal(data, currentData) {
		return nil

	}

	err := p.setPolicyData(policy, data)
	if err != nil {
		return fmt.Errorf("%v: %w", err, ErrInternal)
	}

	return nil
}

// setPolicyData reinitializes the VMs one at a time.
func (p *pool) setPolicyData(policy []byte, data []byte) error {
	for i, activations := 0, 0; true; i++ {
		vm := p.wait(i)
		if vm == nil {
			// All have been converted.
			return nil
		}

		if err := vm.SetPolicyData(policy, data); err != nil {
			// No guarantee about the VM state after an error; hence, remove.
			p.remove(i)
			p.Release(vm)

			// After the first successful activation, proceed through all the VMs, ignoring the remaining errors.
			if activations == 0 {
				return err
			}
		} else {
			p.Release(vm)
		}

		// Activate the policy and data, now that a single VM has been reset without errors.

		if activations == 0 {
			p.activate(policy, data)
		}

		activations++
	}

	return nil
}

// Close waits for all the evaluations to finish and then releases the VMs.
func (p *pool) Close() {
	for range p.vms {
		<-p.available
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, vm := range p.vms {
		if vm != nil {
			vm.Close()
		}
	}

	p.closed = true
	p.vms = nil
}

// wait steals the i'th VM instance. The VM has to be released afterwards.
func (p *pool) wait(i int) *vm {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if i == len(p.vms) {
		return nil
	}

	vm := p.vms[i]
	isActive := p.acquired[i]
	p.acquired[i] = true

	if isActive {
		p.blockedReinit = make(chan struct{}, 1)
		p.pendingReinit = vm
	}

	p.mutex.Unlock()

	if isActive {
		<-p.blockedReinit
	} else {
		<-p.available
	}

	p.mutex.Lock()
	p.pendingReinit = nil
	return vm
}

// remove removes the i'th vm.
func (p *pool) remove(i int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	n := len(p.vms)
	if n > 1 {
		p.vms[i] = p.vms[n-1]
	}

	p.vms = p.vms[0 : n-1]
	p.acquired = p.acquired[0 : n-1]
}

func (p *pool) activate(policy []byte, data []byte) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.policy, p.data = policy, data
}
