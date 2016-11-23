// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/pkg/errors"
)

// TODO(tsandall): update policy store to use correct transaction ids

// policyStore provides a storage abstraction for policy definitions and modules.
type policyStore struct {
	policyDir string
	raw       map[string][]byte
	modules   map[string]*ast.Module
}

// loadPolicies is the default callback function that will be used when
// opening the policy store.
func loadPolicies(bufs map[string][]byte) (map[string]*ast.Module, error) {

	parsed := map[string]*ast.Module{}

	for id, bs := range bufs {
		mod, err := ast.ParseModule(id, string(bs))
		if err != nil {
			return nil, err
		}
		parsed[id] = mod
	}

	c := ast.NewCompiler()
	if c.Compile(parsed); c.Failed() {
		return nil, c.Errors
	}

	return parsed, nil
}

// NewPolicyStore returns an empty PolicyStore.
func newPolicyStore(policyDir string) *policyStore {
	return &policyStore{
		policyDir: policyDir,
		raw:       map[string][]byte{},
		modules:   map[string]*ast.Module{},
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

// Open initializes the policy store.
//
// This should be called on startup to load policies from persistent storage.
// The callback function "f" will be invoked with the buffers representing the
// persisted policies. The callback should return the compiled version of the
// policies so that they can be installed into the  store.
func (p *policyStore) Open(txn Transaction, f func(map[string][]byte) (map[string]*ast.Module, error)) error {

	if len(p.policyDir) == 0 {
		return nil
	}

	info, err := ioutil.ReadDir(p.policyDir)
	if err != nil {
		return err
	}

	raw := map[string][]byte{}

	for _, i := range info {

		f := i.Name()
		bs, err := ioutil.ReadFile(filepath.Join(p.policyDir, f))

		if err != nil {
			return err
		}

		id := p.getID(f)
		raw[id] = bs
	}

	mods, err := f(raw)
	if err != nil {
		return err
	}

	for id, mod := range mods {
		if err := p.Add(id, mod, raw[id], false); err != nil {
			return err
		}
	}

	return nil
}

// Add inserts the policy module into the store. If an existing policy module exists with the same ID,
// it is overwritten. If persist is false, then the policy will not be persisted.
func (p *policyStore) Add(id string, mod *ast.Module, raw []byte, persist bool) error {

	if persist && len(p.policyDir) == 0 {
		return fmt.Errorf("cannot persist without --policy-dir set")
	}

	p.raw[id] = raw
	p.modules[id] = mod

	if persist {
		filename := p.getFilename(id)
		if err := ioutil.WriteFile(filename, raw, 0644); err != nil {
			return errors.Wrapf(err, "failed to persist definition but new version was installed: %v", id)
		}
	}

	return nil
}

// Remove removes the policy module for id.
func (p *policyStore) Remove(id string) error {

	filename := p.getFilename(id)

	if strings.HasPrefix(filename, p.policyDir) {
		if err := os.Remove(filename); err != nil {
			if !os.IsNotExist(err) {
				return errors.Wrapf(err, "failed to delete persisted definition but module was uninstalled: %v", id)
			}
		}
	}

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
		return nil, notFoundErrorf("definition not found: %v", id)
	}
	return bs, nil
}

func (p *policyStore) getFilename(id string) string {
	return filepath.Join(p.policyDir, id)
}

func (p *policyStore) getID(f string) string {
	return filepath.Base(f)
}
