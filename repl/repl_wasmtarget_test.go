// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package repl

import (
	"bytes"
	"context"
	"testing"
)

func TestReplWasmTarget(t *testing.T) {
	ctx := context.Background()
	store := newTestStore()
	var buffer bytes.Buffer
	repl := newRepl(store, &buffer)

	err := repl.OneShot(ctx, "target foo bar")

	expected := "code bad arguments: target <mode>: expects exactly one argument"
	if err == nil || err.Error() != expected {
		t.Fatalf("Expected error %s, got %s", expected, err)
	}

	err = repl.OneShot(ctx, "target foo")

	expected = "invalid target \"foo\":must be one of {rego,wasm}"
	if err == nil || err.Error() != expected {
		t.Fatalf("Expected error %s, got %s", expected, err)
	}

	buffer.Reset()
	err = repl.OneShot(ctx, "target wasm")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	repl.OneShot(ctx, `p = true { input.foo = "bar" }`)
	buffer.Reset()
	repl.OneShot(ctx, "p")

	if buffer.String() != "undefined\n" {
		t.Fatalf("Expected undefined but got: %v", buffer.String())
	}

	buffer.Reset()
	repl.OneShot(ctx, `p with input as {"foo": "bar"}`)

	result := buffer.String()
	expected = "true\n"

	if result != expected {
		t.Fatalf("Expected true but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(ctx, `p with input.foo as "bar"`)

	result = buffer.String()

	if result != expected {
		t.Fatalf("Expected true but got: %v", result)
	}

	buffer.Reset()
	repl.OneShot(ctx, `trace`)

	result = buffer.String()

	expected = "warning: trace mode \"full\" is not supported with wasm target\n"
	if result != expected {
		t.Fatalf("Expected true but got: %v", result)
	}
}
