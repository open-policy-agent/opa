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

// policyStore provides a storage abstraction for policy definitions and modules.
type policyStore struct {
	store     Store
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
		return nil, c.Errors[0]
	}

	return c.Modules, nil
}

// NewPolicyStore returns an empty PolicyStore.
func newPolicyStore(store Store, policyDir string) *policyStore {
	return &policyStore{
		store:     store,
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
// policies so that they can be installed into the data store.
func (p *policyStore) Open(f func(map[string][]byte) (map[string]*ast.Module, error)) error {

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

	old := p.modules[id]

	if old != nil {
		if err := p.uninstallModule(invalidTXN, old); err != nil {
			return errors.Wrapf(err, "failed to uninstall old version of module: %v", id)
		}
	}

	if err := p.installModule(invalidTXN, mod); err != nil {
		return errors.Wrapf(err, "failed to install module but old version of module was uninstalled: %v", id)
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

	mod, err := p.Get(id)
	if err != nil {
		return err
	}

	if err := p.uninstallModule(invalidTXN, mod); err != nil {
		return errors.Wrapf(err, "failed to uninstall module: %v", id)
	}

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

func (p *policyStore) installModule(txn Transaction, mod *ast.Module) error {

	installed := map[*ast.Rule]ast.Ref{}

	for _, r := range mod.Rules {
		fqn := append(ast.Ref{}, mod.Package.Path...)
		fqn = append(fqn, &ast.Term{Value: ast.String(r.Name)})
		if err := p.installRule(txn, fqn, r); err != nil {
			for r, fqn := range installed {
				if err := p.uninstallRule(txn, fqn, r); err != nil {
					return err
				}
			}
			return err
		}
		installed[r] = fqn
	}

	return nil
}

func (p *policyStore) installRule(txn Transaction, fqn ast.Ref, rule *ast.Rule) error {

	err := makePath(p.store, txn, fqn[:len(fqn)-1])
	if err != nil {
		return errors.Wrapf(err, "unable to make path for rule set")
	}

	node, err := p.store.Read(txn, fqn)
	if err != nil {
		switch err := err.(type) {
		case *Error:
			if err.Code == NotFoundErr {
				rules := []*ast.Rule{rule}
				if err := p.store.Write(txn, AddOp, fqn, rules); err != nil {
					return errors.Wrapf(err, "unable to add new rule set")
				}
				return nil
			}
		}
		return err
	}

	rs, ok := node.([]*ast.Rule)
	if !ok {
		return fmt.Errorf("unable to add rule to base document")
	}

	for i := range rs {
		if rs[i].Equal(rule) {
			return nil
		}
	}

	rs = append(rs, rule)

	if err := p.store.Write(txn, ReplaceOp, fqn, rs); err != nil {
		return errors.Wrapf(err, "unable to add rule to existing rule set")
	}

	return nil
}

func (p *policyStore) uninstallModule(txn Transaction, mod *ast.Module) error {
	uninstalled := map[*ast.Rule]ast.Ref{}
	for _, r := range mod.Rules {
		fqn := append(ast.Ref{}, mod.Package.Path...)
		fqn = append(fqn, &ast.Term{Value: ast.String(r.Name)})
		if err := p.uninstallRule(txn, fqn, r); err != nil {
			for r, fqn := range uninstalled {
				if err := p.installRule(txn, fqn, r); err != nil {
					return err
				}
			}
			return err
		}
		uninstalled[r] = fqn
	}
	return nil
}

// uninstallRule removes the rule located at the path. If the path is not found
// or the rule does not exist in the ruleset, this function returns nil (no error).
func (p *policyStore) uninstallRule(txn Transaction, fqn ast.Ref, rule *ast.Rule) error {

	node, err := p.store.Read(txn, fqn)

	if IsNotFound(err) {
		return nil
	}

	rs, ok := node.([]*ast.Rule)
	if !ok {
		return fmt.Errorf("unable to remove rule: path refers to base document")
	}

	found := false

	for i := range rs {
		if rs[i].Equal(rule) {
			rs = append(rs[:i], rs[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return nil
	}

	if len(rs) == 0 {
		return p.store.Write(txn, RemoveOp, fqn, nil)
	}

	return p.store.Write(txn, ReplaceOp, fqn, rs)
}

func makePath(store Store, txn Transaction, path ast.Ref) error {
	var tmp ast.Ref
	for _, p := range path {
		tmp = append(tmp, p)
		node, err := store.Read(txn, tmp)
		if err != nil {
			switch err := err.(type) {
			case *Error:
				if err.Code == NotFoundErr {
					err := store.Write(txn, AddOp, tmp, map[string]interface{}{})
					if err != nil {
						return err
					}
					continue
				}
			}
			return err
		}
		switch node.(type) {
		case map[string]interface{}:
		case []interface{}:
		default:
			return fmt.Errorf("non-collection document: %v", path)
		}
	}
	return nil
}
