// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

//go:build opa_wasm
// +build opa_wasm

package rego

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"

	"github.com/open-policy-agent/opa/ast"
	sdk_errors "github.com/open-policy-agent/opa/internal/wasm/sdk/opa/errors"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/util/test"

	_ "github.com/open-policy-agent/opa/features/wasm"
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
	test.Skip(t)

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
			note:       "http.send",
			target:     "rego",
			policy:     httpSend,
			errorCheck: topdown.IsCancel,
		},
		{
			note:       "numbers.range",
			target:     "rego",
			policy:     numbersRange,
			errorCheck: topdown.IsCancel,
		},
		{
			note:       "net.cidr_expand",
			target:     "wasm",
			policy:     cidrExpand,
			errorCheck: sdk_errors.IsCancel,
		},
		{
			note:       "http.send",
			target:     "wasm",
			policy:     httpSend,
			errorCheck: sdk_errors.IsCancel,
		},
		{
			note:       "numbers.range",
			target:     "wasm",
			policy:     numbersRange,
			errorCheck: sdk_errors.IsCancel,
		},
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
			if testing.Verbose() {
				t.Log(err)
			}
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

func TestRandSeedingOptions(t *testing.T) {

	ctx := context.Background()

	exp := "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8"

	for _, tgt := range []string{targetWasm, targetRego} {
		t.Run(tgt, func(t *testing.T) {
			seed := rand.New(rand.NewSource(0))

			// Check expected uuid is returned.
			rs, err := New(Query(`uuid.rfc4122("", x)`), Seed(seed), Target(tgt)).Eval(ctx)
			if err != nil {
				t.Fatal(err)
			} else if rs[0].Bindings["x"] != exp {
				t.Fatalf("expected %q but got %q", exp, rs[0].Bindings["x"])
			}

			// Check that seed does not propagate to prepared query.
			eval, err := New(Query(`uuid.rfc4122("", x)`), Seed(seed)).PrepareForEval(ctx)
			if err != nil {
				t.Fatal(err)
			}

			rs2, err := eval.Eval(ctx)
			if err != nil {
				t.Fatal(err)
			} else if rs2[0].Bindings["x"] == exp {
				t.Fatal("expected new uuid")
			}

			exp3 := "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75"

			// Check that prepared query uses explicitly provided seed.
			rs3, err := eval.Eval(ctx, EvalSeed(seed))
			if err != nil {
				t.Fatal(err)
			} else if rs3[0].Bindings["x"] != exp3 {
				t.Fatalf("expected %q but got %q", exp, rs3[0].Bindings["x"])
			}
		})
	}
}

func TestCompatWithABIMinorVersion1(t *testing.T) {
	ctx := context.Background()

	pq, err := New(
		LoadBundle("testdata/bundle.tar.gz"),
		Query("data.test.allow"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	rs, err := pq.Eval(ctx, EvalInput(map[string]interface{}{"x": "x"}))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertResultSet(t, rs, `[[true]]`)
}

func TestEvalWasmWithInterQueryCache(t *testing.T) {
	newHeaders := map[string][]string{"Cache-Control": {"max-age=290304000, public"}}

	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		headers := w.Header()

		for k, v := range newHeaders {
			headers[k] = v
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"x": 1}`))
	}))
	defer ts.Close()

	query := fmt.Sprintf(`http.send({"method": "get", "url": "%s", "force_json_decode": true, "cache": true})`, ts.URL)

	// add an inter-query cache
	config, _ := cache.ParseCachingConfig(nil)
	interQueryCache := cache.NewInterQueryCache(config)

	ctx := context.Background()
	_, err := New(Target("wasm"), Query(query), InterQueryBuiltinCache(interQueryCache)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// eval again with same query
	// this request should be served by the cache
	_, err = New(Target("wasm"), Query(query), InterQueryBuiltinCache(interQueryCache)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatal("Expected server to be called only once")
	}
}

func TestEvalWasmWithHTTPAllowNet(t *testing.T) {
	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"x": 1}`))
	}))
	defer ts.Close()

	serverUrl, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	serverHost := strings.Split(serverUrl.Host, ":")[0]

	query := fmt.Sprintf(`http.send({"method": "get", "url": "%s", "force_json_decode": true, "cache": true})`, ts.URL)
	capabilities := ast.CapabilitiesForThisVersion()
	capabilities.AllowNet = []string{"example.com"}

	// add an inter-query cache
	config, _ := cache.ParseCachingConfig(nil)
	interQueryCache := cache.NewInterQueryCache(config)

	ctx := context.Background()
	// StrictBuiltinErrors(true) has no effect when target is 'wasm'
	// this request should be rejected by the allow_net allowlist
	_, err = New(Target("wasm"), Query(query), InterQueryBuiltinCache(interQueryCache), Capabilities(capabilities)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 0 {
		t.Fatal("Expected server to not be called")
	}

	capabilities.AllowNet = []string{serverHost}

	// eval again with same query
	// this request should not be rejected by the allow_net allowlist
	_, err = New(Target("wasm"), Query(query), InterQueryBuiltinCache(interQueryCache), Capabilities(capabilities)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatal("Expected server to never be called")
	}
}
