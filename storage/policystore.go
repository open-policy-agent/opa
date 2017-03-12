// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"path/filepath"

	"github.com/open-policy-agent/opa/ast"
)

// TODO(tsandall): remove policy store entirely--the data store should be able
// to handle storage of source files.

// policyStore provides a storage abstraction for policy definitions and modules.
type policyStore struct {
	raw     map[string][]byte
	modules map[string]*ast.Module
}

// NewPolicyStore returns an empty PolicyStore.
func newPolicyStore() *policyStore {
	return &policyStore{
		raw:     map[string][]byte{},
		modules: map[string]*ast.Module{},
	}
}

// List returns all of the modules.
func (p *policyStore) List() map[string]*ast.Module {
	cpy := map[string]*ast.Module{}
	for k, v := range p.modules {
		cpy[k] = v
	}
	return cpy
}

// Add inserts the policy module into the store. If an existing policy module exists with the same ID,
// it is overwritten.
func (p *policyStore) Add(id string, mod *ast.Module, raw []byte) error {
	p.raw[id] = raw
	p.modules[id] = mod
	return nil
}

// Remove removes the policy module for id.
func (p *policyStore) Remove(id string) error {
	delete(p.raw, id)
	delete(p.modules, id)
	return nil
}

// Get returns the policy module for id.
func (p *policyStore) Get(id string) (*ast.Module, error) {
	mod, ok := p.modules[id]
	if !ok {
		return nil, notFoundErrorf("module not found: %v", id)
	}
	return mod, nil
}

// GetRaw returns the raw content of the module for id.
func (p *policyStore) GetRaw(id string) ([]byte, error) {
	bs, ok := p.raw[id]
	if !ok {
		return nil, notFoundErrorf("source not found: %v", id)
	}
	return bs, nil
}

func (p *policyStore) getID(f string) string {
	return filepath.Base(f)
}
