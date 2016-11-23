// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

func TestInit(t *testing.T) {
	tmp1, err := ioutil.TempFile("", "docFile")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp1.Name())

	doc1 := `{"foo": "bar", "x": {"y": {"z": [1]}}}`
	if _, err := tmp1.Write([]byte(doc1)); err != nil {
		panic(err)
	}
	if err := tmp1.Close(); err != nil {
		panic(err)
	}

	tmp2, err := ioutil.TempFile("", "policyFile")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmp2.Name())
	mod1 := `
	package a.b.c
	import data.foo
	p = true :- foo = "bar"
	p = true :- 1 = 2
	`
	if _, err := tmp2.Write([]byte(mod1)); err != nil {
		panic(err)
	}
	if err := tmp2.Close(); err != nil {
		panic(err)
	}

	tmp3, err := ioutil.TempDir("", "policyDir")
	if err != nil {
		panic(err)
	}

	defer os.RemoveAll(tmp3)

	tmp4 := filepath.Join(tmp3, "existingPolicy")

	err = ioutil.WriteFile(tmp4, []byte(`
	package a.b.c
	q = true :- false
	`), 0644)
	if err != nil {
		panic(err)
	}

	rt := Runtime{}

	err = rt.init(&Params{
		Paths:     []string{tmp1.Name(), tmp2.Name()},
		PolicyDir: tmp3,
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	txn := storage.NewTransactionOrDie(rt.Store)

	node, err := rt.Store.Read(txn, ast.MustParseRef("data.foo"))
	if util.Compare(node, "bar") != 0 || err != nil {
		t.Errorf("Expected %v but got %v (err: %v)", "bar", node, err)
		return
	}

	modules := rt.Store.ListPolicies(txn)
	expected := ast.MustParseModule(mod1)

	if !expected.Equal(modules[tmp2.Name()]) {
		t.Fatalf("Expected %v but got: %v", expected, modules[tmp2.Name()])
	}

}
