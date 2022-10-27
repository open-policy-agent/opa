// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package file

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/bundle"
)

func TestFileLoader(t *testing.T) {
	// Assign a temp file.

	f, err := os.CreateTemp("", "test-file-loader")
	if err != nil {
		panic(err)
	}

	defer os.Remove(f.Name())

	// Start loader, without having a file in place.

	var pd testPolicyData
	loader, err := new(&pd).WithFile(f.Name()).WithInterval(10 * time.Millisecond).Init()
	if err != nil {
		t.Fatal(err.Error())
	}

	ctx := context.Background()
	if err := loader.Start(ctx); err == nil {
		t.Fatal("missing file not resulting in an error")
	}

	policy := "wasm-policy"
	var data interface{} = map[string]interface{}{
		"foo": "bar",
	}

	// Start loader, with the file in place.

	writeBundle(f.Name(), policy, data)

	if err := loader.Start(ctx); err != nil {
		t.Fatalf("unable to start loader: %v", err)
	}

	pd.CheckEqual(t, policy, &data)

	// Reload with updated contents.

	policy = "wasm-policy-modified"
	data = map[string]interface{}{
		"bar": "foo",
	}

	writeBundle(f.Name(), policy, data)

	pd.WaitUpdate()
	pd.CheckEqual(t, policy, &data)

	loader.Close()
}

type testPolicyData struct {
	sync.Mutex
	policy  []byte
	data    *interface{}
	updated chan struct{}
}

func (pd *testPolicyData) SetPolicyData(_ context.Context, policy []byte, data *interface{}) error {
	pd.Lock()
	defer pd.Unlock()

	pd.policy = policy
	pd.data = data
	if pd.updated != nil {
		close(pd.updated)
	}

	return nil
}

func (pd *testPolicyData) CheckEqual(t *testing.T, policy string, data *interface{}) {
	pd.Lock()
	defer pd.Unlock()

	if !bytes.Equal([]byte(policy), pd.policy) && reflect.DeepEqual(data, pd.data) {
		t.Fatal("policy/data mismatch.")
	}
}

func (pd *testPolicyData) WaitUpdate() {
	pd.Lock()
	pd.updated = make(chan struct{})
	pd.Unlock()

	<-pd.updated

	pd.Lock()
	pd.updated = nil
	pd.Unlock()
}

func writeBundle(name string, policy string, data interface{}) {
	b := bundle.Bundle{
		Data: data.(map[string]interface{}),
		Wasm: []byte(policy),
	}

	var buf bytes.Buffer
	if err := bundle.Write(&buf, b); err != nil {
		panic(err)
	}

	if err := os.WriteFile(name, buf.Bytes(), 0644); err != nil {
		panic(err)
	}
}
