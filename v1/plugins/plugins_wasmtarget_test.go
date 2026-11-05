// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm

package plugins

import (
	"io"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/compile"
	"github.com/open-policy-agent/opa/v1/resolver"
	"github.com/open-policy-agent/opa/v1/resolver/wasm"
	"github.com/open-policy-agent/opa/v1/storage/inmem"

	_ "github.com/open-policy-agent/opa/v1/features/wasm"
)

func TestSetWasmResolversClosesReplacedResolvers(t *testing.T) {
	c := compile.New().
		WithTarget(compile.TargetWasm).
		WithEntrypoints("test/p").
		WithBundle(&bundle.Bundle{
			Modules: []bundle.ModuleFile{{
				URL:    "p.rego",
				Path:   "p.rego",
				Raw:    []byte("package test\n\np := 1\n"),
				Parsed: ast.MustParseModule("package test\n\np := 1\n"),
			}},
		}).
		WithOutput(io.Discard)
	if err := c.Build(t.Context()); err != nil {
		t.Fatal(err)
	}

	b := c.Bundle()

	newResolver := func() *wasm.Resolver {
		r, err := wasm.New([]ast.Ref{ast.MustParseRef("data.test.p")}, b.WasmModules[0].Raw, map[string]any{})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}

	m, err := New([]byte{}, "test", inmem.New())
	if err != nil {
		t.Fatal(err)
	}

	first, second := newResolver(), newResolver()

	m.setWasmResolvers([]*wasm.Resolver{first})
	m.setWasmResolvers([]*wasm.Resolver{first})

	input := resolver.Input{Ref: ast.MustParseRef("data.test.p")}
	if _, err := first.Eval(t.Context(), input); err != nil {
		t.Fatalf("a resolver that was set again must stay open: %v", err)
	}

	m.setWasmResolvers([]*wasm.Resolver{second})

	if _, err := first.Eval(t.Context(), input); err == nil {
		t.Fatal("expected an error evaluating a replaced resolver, which should be closed")
	}
	if _, err := second.Eval(t.Context(), input); err != nil {
		t.Fatalf("the current resolver must stay open: %v", err)
	}
}
