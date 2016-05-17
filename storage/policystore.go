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

// PolicyStore provides a storage abstraction for policy definitions and modules.
//
type PolicyStore struct {
	dataStore *DataStore
	policyDir string
	raw       map[string][]byte
	modules   map[string]*ast.Module
}

// LoadPolicies is the default callback function that will be used when
// opening the policy store.
func LoadPolicies(bufs map[string][]byte) (map[string]*ast.Module, error) {

	parsed := map[string]*ast.Module{}

	for id, bs := range bufs {
		mod, err := ast.ParseModule(string(bs))
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
func NewPolicyStore(store *DataStore, policyDir string) *PolicyStore {
	return &PolicyStore{
		dataStore: store,
		policyDir: policyDir,
		raw:       map[string][]byte{},
		modules:   map[string]*ast.Module{},
	}
}

// List returns all of the modules.
func (p *PolicyStore) List() map[string]*ast.Module {
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
//
func (p *PolicyStore) Open(f func(map[string][]byte) (map[string]*ast.Module, error)) error {

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
func (p *PolicyStore) Add(id string, mod *ast.Module, raw []byte, persist bool) error {

	if persist && len(p.policyDir) == 0 {
		return fmt.Errorf("cannot persist without --policy-dir set")
	}

	old := p.modules[id]

	if old != nil {
		if err := p.uninstallModule(old); err != nil {
			return errors.Wrapf(err, "failed to uninstall old version of module: %v", id)
		}
	}

	if err := p.installModule(mod); err != nil {
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
func (p *PolicyStore) Remove(id string) error {

	mod, err := p.Get(id)
	if err != nil {
		return err
	}

	if err := p.uninstallModule(mod); err != nil {
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
func (p *PolicyStore) Get(id string) (*ast.Module, error) {
	mod, ok := p.modules[id]
	if !ok {
		return nil, notFoundErrorf("module not found: %v", id)
	}
	return mod, nil
}

// GetRaw returns the raw content of the module for id.
func (p *PolicyStore) GetRaw(id string) ([]byte, error) {
	bs, ok := p.raw[id]
	if !ok {
		return nil, notFoundErrorf("definition not found: %v", id)
	}
	return bs, nil
}

func (p *PolicyStore) getFilename(id string) string {
	return filepath.Join(p.policyDir, id)
}

func (p *PolicyStore) getID(f string) string {
	return filepath.Base(f)
}

func (p *PolicyStore) installModule(mod *ast.Module) error {

	installed := map[*ast.Rule][]interface{}{}

	for _, r := range mod.Rules {
		fqn := append(ast.Ref{}, mod.Package.Path...)
		fqn = append(fqn, &ast.Term{Value: ast.String(r.Name)})
		path, _ := fqn.Underlying()
		path = path[1:]
		if err := p.installRule(path, r); err != nil {
			for r, path := range installed {
				if err := p.uninstallRule(path, r); err != nil {
					return err
				}
			}
			return err
		}
		installed[r] = path
	}

	return nil
}

func (p *PolicyStore) installRule(path []interface{}, rule *ast.Rule) error {

	err := p.dataStore.MakePath(path[:len(path)-1])
	if err != nil {
		return errors.Wrapf(err, "unable to make path for rule set")
	}

	node, err := p.dataStore.Get(path)
	if err != nil {
		switch err := err.(type) {
		case *Error:
			if err.Code == NotFoundErr {
				rules := []*ast.Rule{rule}
				if err := p.dataStore.Patch(AddOp, path, rules); err != nil {
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

	if err := p.dataStore.Patch(ReplaceOp, path, rs); err != nil {
		return errors.Wrapf(err, "unable to add rule to existing rule set")
	}

	return nil
}

func (p *PolicyStore) uninstallModule(mod *ast.Module) error {
	uninstalled := map[*ast.Rule][]interface{}{}
	for _, r := range mod.Rules {
		fqn := append(ast.Ref{}, mod.Package.Path...)
		fqn = append(fqn, &ast.Term{Value: ast.String(r.Name)})
		path, _ := fqn.Underlying()
		path = path[1:]
		if err := p.uninstallRule(path, r); err != nil {
			for r, path := range uninstalled {
				if err := p.installRule(path, r); err != nil {
					return err
				}
			}
			return err
		}
		uninstalled[r] = path
	}
	return nil
}

// uninstallRule removes the rule located at the path. If the path is not found
// or the rule does not exist in the ruleset, this function returns nil (no error).
func (p *PolicyStore) uninstallRule(path []interface{}, rule *ast.Rule) error {

	node, err := p.dataStore.Get(path)

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
		return p.dataStore.Patch(RemoveOp, path, nil)
	}

	return p.dataStore.Patch(ReplaceOp, path, rs)
}
