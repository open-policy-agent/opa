// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package storage

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

// TODO(tsandall): refactor these tests cases to belong to Storage as the
// policyStore is now an implementation detail of the storage layer and these
// are essentially integration tests.

func TestPolicyStoreAdd(t *testing.T) {

	f := newFixture()

	mod1 := f.compile1(testMod1)
	mod2 := f.compile1(testMod2)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1))
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod2", mod2, []byte(testMod2))
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

	mod1 := f.compile1(testMod1)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1))
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod1", mod1, []byte(testMod1))
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

}

func TestPolicyStoreRemove(t *testing.T) {

	f := newFixture()

	mod1 := f.compile1(testMod1)
	mod2 := f.compile1(testMod2)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1))
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod2", mod2, []byte(testMod2))
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
}

func TestPolicyStoreUpdate(t *testing.T) {
	f := newFixture()

	mod1 := f.compile1(testMod1)
	mod2 := f.compile1(testMod2)

	err := f.policyStore.Add("testMod1", mod1, []byte(testMod1))
	if err != nil {
		t.Errorf("Unexpected error on Add(): %v", err)
		return
	}

	err = f.policyStore.Add("testMod1", mod2, []byte(testMod2))
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
	policyStore := newPolicyStore()
	f := &fixture{
		policyStore: policyStore,
	}
	return f
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
