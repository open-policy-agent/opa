// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

// TODO(tsandall): refactor these tests cases to belong to Storage as the
// policyStore is now an implementation detail of the storage layer and these
// are essentially integration tests.

func TestPolicyStoreDefaultOpen(t *testing.T) {

	dir, err := ioutil.TempDir("", "policyDir")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(dir)

	filename := filepath.Join(dir, "testMod1")

	err = ioutil.WriteFile(filename, []byte(testMod1), 0644)
	if err != nil {
		panic(err)
	}

	policyStore := newPolicyStore(dir)

	err = policyStore.Open(invalidTXN, loadPolicies)
	if err != nil {
		t.Errorf("Unexpected error on Open(): %v", err)
		return
	}

	c := ast.NewCompiler()
	mod := ast.MustParseModule(testMod1)
	if c.Compile(map[string]*ast.Module{"testMod1": mod}); c.Failed() {
		panic(c.Errors)
	}

	stored, err := policyStore.Get("testMod1")
	if err != nil {
		t.Errorf("Unexpected error on Get(): %v", err)
		return
	}

	if !mod.Equal(stored) {
		t.Fatalf("Expected %v from policy store but got: %v", mod, stored)
	}
}

func TestPolicyStoreAdd(t *testing.T) {

	f := newFixture()
	defer f.cleanup()

	mod1 := f.compile1(testMod1)
	mod2 := f.compile1(testMod2)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod2", mod2, []byte(testMod2), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	r, err := f.policyStore.Get("testMod1")
	if err != nil {
		t.Errorf("Unexpected error on Get(): %v", err)
		return
	}

	if !mod1.Equal(r) {
		t.Errorf("Expected %v for Get() but got: %v", mod1, r)
		return
	}

	raw, err := f.policyStore.GetRaw("testMod1")
	if err != nil {
		t.Errorf("Unexpected error on GetRaw(): %v", err)
		return
	}

	if string(raw) != testMod1 {
		t.Errorf("Expected %v for GetRaw() but got: %v", testMod1, raw)
	}

	mods := f.policyStore.List()

	if len(mods) != 2 {
		t.Errorf("Expected a single module from List() but got: %v", mods)
		return
	}

	if !mods["testMod1"].Equal(mod1) {
		t.Errorf("Expected List() result to equal %v but got %v", mod1, mods["testMod1"])
		return
	}
}

func TestPolicyStoreAddIdempotent(t *testing.T) {

	f := newFixture()
	defer f.cleanup()

	mod1 := f.compile1(testMod1)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod1", mod1, []byte(testMod1), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

}

func TestPolicyStoreRemove(t *testing.T) {

	f := newFixture()
	defer f.cleanup()

	mod1 := f.compile1(testMod1)
	mod2 := f.compile1(testMod2)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod2", mod2, []byte(testMod2), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	if err := f.policyStore.Remove("testMod1"); err != nil {
		t.Errorf("Unexpected error on Remove(): %v", err)
		return
	}

	mods := f.policyStore.List()

	if len(mods) != 1 {
		t.Errorf("Expected one module to remain after Remove(): %v", mods)
		return
	}

	if _, err := f.policyStore.Get("testMod2"); err != nil {
		t.Errorf("Expected testMod2 to remain after Remove(): %v", mods)
		return
	}

	_, err = os.Stat(f.policyStore.getFilename("testMod1"))
	if !os.IsNotExist(err) {
		info, err := ioutil.ReadDir(f.policyStore.policyDir)
		if err != nil {
			panic(err)
		}
		files := []string{}
		for _, i := range info {
			files = append(files, i.Name())
		}
		t.Errorf("Expected testMod1 to be removed from disk but %v contains: %v", f.policyStore.policyDir, files)
		return
	}
}

func TestPolicyStoreUpdate(t *testing.T) {
	f := newFixture()
	defer f.cleanup()

	mod1 := f.compile1(testMod1)
	mod2 := f.compile1(testMod2)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod1", mod2, []byte(testMod2), true)
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

}

const (
	testMod1 = `package a.b

p = true { q }
q = true { true }`

	testMod2 = `package a.b

p = true { false }`
)

type fixture struct {
	policyStore *policyStore
}

func newFixture() *fixture {

	dir, err := ioutil.TempDir("", "policyDir")
	if err != nil {
		panic(err)
	}

	policyStore := newPolicyStore(dir)
	err = policyStore.Open(invalidTXN, func(map[string][]byte) (map[string]*ast.Module, error) {
		return nil, nil
	})
	if err != nil {
		panic(err)
	}

	f := &fixture{
		policyStore: policyStore,
	}

	return f
}

func (f *fixture) cleanup() {
	os.RemoveAll(f.policyStore.policyDir)
}

func (f *fixture) compile1(m string) *ast.Module {

	mods := f.policyStore.List()
	mod := ast.MustParseModule(m)
	mods[""] = mod

	c := ast.NewCompiler()
	if c.Compile(mods); c.Failed() {
		panic(c.Errors)
	}

	return c.Modules[""]
}
