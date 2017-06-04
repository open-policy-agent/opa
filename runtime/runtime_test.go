// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/util"
)

func TestEval(t *testing.T) {
	params := NewParams()
	var buffer bytes.Buffer
	params.Output = &buffer
	params.OutputFormat = "json"
	params.Eval = `a = b; a = 1; c = 2; c > b`
	rt := &Runtime{}
	rt.Start(params)
	expected := parseJSON(`[{"a": 1, "b": 1, "c": 2}]`)
	result := parseJSON(buffer.String())
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Expected %v but got: %v", expected, result)
	}
}

func TestInit(t *testing.T) {
	ctx := context.Background()

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
	mod1 := `package a.b.c

import data.foo

p = true { foo = "bar" }
p = true { 1 = 2 }`
	if _, err := tmp2.Write([]byte(mod1)); err != nil {
		panic(err)
	}
	if err := tmp2.Close(); err != nil {
		panic(err)
	}

	rt := Runtime{}

	err = rt.init(ctx, &Params{
		Paths: []string{tmp1.Name(), tmp2.Name()},
	})

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
		return
	}

	txn := storage.NewTransactionOrDie(ctx, rt.Store)

	node, err := rt.Store.Read(ctx, txn, storage.MustParsePath("/foo"))
	if util.Compare(node, "bar") != 0 || err != nil {
		t.Errorf("Expected %v but got %v (err: %v)", "bar", node, err)
		return
	}

	id := normalizeModuleID(tmp2.Name())

	result, err := rt.Store.GetPolicy(ctx, txn, id)
	if err != nil || string(result) != mod1 {
		t.Fatalf("Expected %v but got: %v (err: %v)", mod1, result, err)
	}
}

func TestWatchPaths(t *testing.T) {

	fs := map[string]string{
		"/foo/bar/baz.json": "true",
	}

	expected := []string{
		"/foo", "/foo/bar", "/foo/bar/baz.json",
	}

	withTempFS(fs, func(rootDir string) {
		paths, err := getWatchPaths([]string{"prefix:" + rootDir + "/foo"})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		result := []string{}
		for _, path := range paths {
			result = append(result, strings.TrimPrefix(path, rootDir))
		}
		if !reflect.DeepEqual(expected, result) {
			t.Fatalf("Expected %v but got: %v", expected, result)
		}
	})
}

func parseJSON(s string) interface{} {
	var x interface{}
	if err := util.UnmarshalJSON([]byte(s), &x); err != nil {
		panic(err)
	}
	return x
}
