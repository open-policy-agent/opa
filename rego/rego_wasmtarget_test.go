// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package rego

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/open-policy-agent/opa/ast"
	sdk_errors "github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
)

func TestPrepareAndEvalWithWasmTarget(t *testing.T) {
	mod := `
	package test
	default p = false
	p {
		input.x == 1
	}
	`

	ctx := context.Background()

	pq, err := New(
		Query("data.test.p = x"),
		Target("wasm"),
		Module("a.rego", mod),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"x": 1}),
	}, "[[true]]")

	pq, err = New(
		Query("a = [1,2]; x = a[i]"),
		Target("wasm"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{}, "[[true, true],[true, true]]")

	pq, err = New(
		Query("foo(100)"),
		Target("wasm"),
	).PrepareForEval(ctx)

	if err == nil {
		t.Fatal("Expected error")
	}
}

func TestPrepareAndEvalWithWasmTargetModulesOnCompiler(t *testing.T) {
	mod := `
	package test
	default p = false
	p {
		input.x == data.x.p
	}
	`

	compiler := ast.NewCompiler()

	compiler.Compile(map[string]*ast.Module{
		"a.rego": ast.MustParseModule(mod),
	})

	if len(compiler.Errors) > 0 {
		t.Fatalf("Unexpected compile errors: %s", compiler.Errors)
	}

	ctx := context.Background()

	pq, err := New(
		Compiler(compiler),
		Query("data.test.p"),
		Target("wasm"),
		Store(inmem.NewFromObject(map[string]interface{}{
			"x": map[string]interface{}{"p": 1},
		})),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"x": 1}),
	}, "[[true]]")
}

func TestWasmTimeOfDay(t *testing.T) {

	ctx := context.Background()
	pq, err := New(Query("time.now_ns()"), Target("wasm")).PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Unix(1615397269, 0)

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalTime(now),
	}, "[[1615397269000000000]]")
}

func TestEvalWithContextTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			// Without this, our test execution would hang waiting for this server to have
			// served all requests to the end -- unrelated to the behaviour in the client,
			// so the test would still pass.
			return
		case <-time.After(5 * time.Second):
			return
		}
	}))
	defer ts.Close()

	// This is host function, i.e. it's not implemented natively in wasm,
	// but calls the topdown function from the wasm instance's execution.
	// Also, it uses the topdown.Cancel mechanism for cancellation.
	cidrExpand := `package p
allow {
	net.cidr_expand("1.0.0.0/1")
}`

	// Also a host function, but uses context.Context for cancellation.
	httpSend := fmt.Sprintf(`package p
allow {
	http.send({"method": "get", "url": "%s", "raise_error": true})
}`,
		ts.URL)
	httpSendTopdownError := fmt.Sprintf(`http.send: Get "%s": context deadline exceeded`, ts.URL)

	// This is a natively-implemented (for the wasm target) function that
	// takes long.
	numbersRange := `package p
allow {
	numbers.range(1, 1e8)[_] == 1e8
}`

	for _, tc := range []struct {
		note, target, policy string
		errorCheck           func(error) bool
	}{
		{
			note:       "net.cidr_expand",
			target:     "rego",
			policy:     cidrExpand,
			errorCheck: topdown.IsCancel,
		},
		{
			note:   "http.send",
			target: "rego",
			policy: httpSend,
			errorCheck: func(err error) bool {
				var te *topdown.Error
				return errors.As(err, &te) && te.Message == httpSendTopdownError
			},
		},
		{
			note:       "numbers.range",
			target:     "rego",
			policy:     numbersRange,
			errorCheck: topdown.IsCancel,
		},
		{
			note:   "net.cidr_expand",
			target: "wasm",
			policy: cidrExpand,
			errorCheck: func(err error) bool {
				return errors.Is(err, sdk_errors.ErrCancelled)
			},
		},
		{
			note:   "http.send",
			target: "wasm",
			policy: httpSend,
			errorCheck: func(err error) bool {
				return errors.Is(err, sdk_errors.ErrCancelled)
			},
		},
		{
			note:   "numbers.range",
			target: "wasm",
			policy: numbersRange,
			errorCheck: func(err error) bool {
				return errors.Is(err, sdk_errors.ErrCancelled)
			}},
	} {
		t.Run(tc.target+"/"+tc.note, func(t *testing.T) {
			defer leaktest.Check(t)()
			before := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			pq, err := New(
				Query("data.p.allow"),
				Module("p.rego", tc.policy),
				Target(tc.target),
				StrictBuiltinErrors(true), // ignored for wasm target (always non-strict)
			).PrepareForEval(ctx)
			if err != nil {
				t.Fatal(err)
			}

			_, err = pq.Eval(ctx)
			if !tc.errorCheck(err) {
				t.Errorf("failed checking error, got %[1]v (%[1]T)", err)
			}
			if time.Since(before) > 2*time.Second {
				// if the cancelled execution took so long, it wasn't really cancelled
				t.Errorf("expected cancellation, but test ran %s", time.Since(before))
			}
		})
	}
}
