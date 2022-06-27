// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/bundle"
)

func TestHTTPLoader(t *testing.T) {
	// Start loader, without having the HTTP content in place.

	var pd testPolicyData
	loader, err := newLoader(&pd).WithURL("http://localhost:0").WithInterval(10*time.Millisecond, 20*time.Millisecond).Init()
	if err != nil {
		t.Fatal(err.Error())
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	if err := loader.Start(ctx); err != context.Canceled {
		t.Fatalf("missing file not resulting in a correct error: %v", err)
	}

	// Start again, with the HTTP content in place.

	var mutex sync.Mutex
	policy := "wasm-policy"
	var data interface{} = map[string]interface{}{
		"foo": "bar",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()

		if err := bundle.Write(w, bundle.Bundle{
			Data: data.(map[string]interface{}),
			Wasm: []byte(policy),
		}); err != nil {
			panic(err)
		}
	}))
	defer ts.Close()

	loader, err = newLoader(&pd).WithURL(ts.URL).WithInterval(10*time.Millisecond, 20*time.Millisecond).Init()
	if err != nil {
		t.Fatal(err.Error())
	}

	ctx = context.Background()
	if err := loader.Start(ctx); err != nil {
		t.Fatalf("unable to start loader: %v", err)
	}

	pd.CheckEqual(t, policy, &data)

	// Reload with updated contents.

	mutex.Lock()
	policy = "wasm-policy-modified"
	data = map[string]interface{}{
		"bar": "foo",
	}
	mutex.Unlock()

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
