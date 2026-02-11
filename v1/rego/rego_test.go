// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package rego

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/bundle"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestRegoEval_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note      string
		module    string
		expResult any
		expErrs   []string
	}{
		{
			note: "v0 module", // v0 in NOT the default version
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "import rego.v1",
			module: `package test
import rego.v1

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note: "v1 module ", // v1 is the default version
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note: "v1 module, v1 compile-time violations", // v1 is the default version
			module: `package test
import data.foo
import data.bar as foo

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"test.rego:3: rego_compile_error: import must not shadow import data.foo",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(root string) {
				ctx := t.Context()

				pq, err := New(
					Load([]string{root}, nil),
					Query("data.test.p"),
				).PrepareForEval(ctx)

				if tc.expErrs != nil {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain %q but got: %v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					rs, err := pq.Eval(ctx)
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					if len(rs) != 1 {
						t.Fatalf("Expected exactly one result but got: %v", rs)
					}

					if reflect.DeepEqual(rs[0].Expressions[0].Value, tc.expResult) {
						t.Fatalf("Expected %v but got: %v", tc.expResult, rs[0].Expressions[0].Value)
					}
				}
			})
		})
	}
}

func TestRegoEval_Capabilities(t *testing.T) {
	tests := []struct {
		note         string
		regoVersion  ast.RegoVersion
		capabilities *ast.Capabilities
		module       string
		expResult    any
		expErrs      []string
	}{
		{
			note:        "v0 module, rego-v0, no capabilities",
			regoVersion: ast.RegoV0,
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note:         "v0 module, rego-v0, v0 capabilities",
			regoVersion:  ast.RegoV0,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note:         "v0 module, rego-v0, v1 capabilities",
			regoVersion:  ast.RegoV0,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expResult: []string{"a", "b", "c"},
		},

		{
			note:        "v0 module, rego-v1, no capabilities",
			regoVersion: ast.RegoV1,
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v0 module, rego-v1, v0 capabilities",
			regoVersion:  ast.RegoV1,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:        "v0 module, rego-v1, v0 capabilities without rego_v1 feature",
			regoVersion: ast.RegoV1,
			capabilities: func() *ast.Capabilities {
				caps := ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0))

				feats := make([]string, 0, len(caps.Features))
				for _, feat := range caps.Features {
					if feat != ast.FeatureRegoV1 {
						feats = append(feats, feat)
					}
				}
				caps.Features = feats

				return caps
			}(),
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v0 module, rego-v1, v1 capabilities",
			regoVersion:  ast.RegoV1,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:3: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		{
			note:        "v1 module, rego-v0, no capabilities",
			regoVersion: ast.RegoV0,
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"test.rego:4: rego_parse_error: unexpected identifier token",
			},
		},
		{
			note:         "v1 module, rego-v0, v0 capabilities",
			regoVersion:  ast.RegoV0,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"test.rego:4: rego_parse_error: unexpected identifier token",
			},
		},
		{
			note:         "v1 module, rego-v0, v1 capabilities",
			regoVersion:  ast.RegoV0,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"test.rego:4: rego_parse_error: unexpected identifier token",
			},
		},

		{
			note:        "v1 module, rego-v1, no capabilities",
			regoVersion: ast.RegoV1,
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note:         "v1 module, rego-v1, v0 capabilities",
			regoVersion:  ast.RegoV1,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
		},
		{
			note:        "v1 module, rego-v1, v0 capabilities without rego_v1 feature",
			regoVersion: ast.RegoV1,
			capabilities: func() *ast.Capabilities {
				caps := ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0))

				feats := make([]string, 0, len(caps.Features))
				for _, feat := range caps.Features {
					if feat != ast.FeatureRegoV1 {
						feats = append(feats, feat)
					}
				}
				caps.Features = feats

				return caps
			}(),
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v1 module, rego-v1, v1 capabilities",
			regoVersion:  ast.RegoV1,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expResult: []string{"a", "b", "c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(root string) {
				ctx := t.Context()

				pq, err := New(
					SetRegoVersion(tc.regoVersion),
					Capabilities(tc.capabilities),
					Load([]string{root}, nil),
					Query("data.test.p"),
				).PrepareForEval(ctx)

				if tc.expErrs != nil {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					rs, err := pq.Eval(ctx)
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					if len(rs) != 1 {
						t.Fatalf("Expected exactly one result but got:\n\n%v", rs)
					}

					if reflect.DeepEqual(rs[0].Expressions[0].Value, tc.expResult) {
						t.Fatalf("Expected %v but got: %v", tc.expResult, rs[0].Expressions[0].Value)
					}
				}
			})
		})
	}
}

func assertEval(t *testing.T, r *Rego, expected string) {
	t.Helper()
	rs, err := r.Eval(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
	assertResultSet(t, rs, expected)
}

func assertPreparedEvalQueryEval(t *testing.T, pq PreparedEvalQuery, options []EvalOption, expected string) {
	t.Helper()
	rs, err := pq.Eval(t.Context(), options...)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
	assertResultSet(t, rs, expected)
}

func assertResultSet(t *testing.T, rs ResultSet, expected string) {
	t.Helper()
	result := make([]any, 0, len(rs))

	for i := range rs {
		values := make([]any, 0, len(rs[i].Expressions))
		for j := range rs[i].Expressions {
			values = append(values, rs[i].Expressions[j].Value)
		}
		result = append(result, values)
	}

	if !reflect.DeepEqual(result, util.MustUnmarshalJSON([]byte(expected))) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", expected, result)
	}
}

func TestRegoEvalExpressionValue(t *testing.T) {

	module := `package test

	arr = [1,false,true]
	f(x) = x
	g(x, y) = x + y
	h(x) = false`

	tests := []struct {
		query    string
		expected string
	}{
		{
			query:    "1",
			expected: "[[1]]",
		},
		{
			query:    "1+2",
			expected: "[[3]]",
		},
		{
			query:    "1+(2*3)",
			expected: "[[7]]",
		},
		{
			query:    "data.test.arr[0]",
			expected: "[[1]]",
		},
		{
			query:    "data.test.arr[1]",
			expected: "[[false]]",
		},
		{
			query:    "data.test.f(1)",
			expected: "[[1]]",
		},
		{
			query:    "data.test.f(1,x)",
			expected: "[[true]]",
		},
		{
			query:    "data.test.g(1,2)",
			expected: "[[3]]",
		},
		{
			query:    "data.test.g(1,2,x)",
			expected: "[[true]]",
		},
		{
			query:    "false",
			expected: "[[false]]",
		},
		{
			query:    "1 == 2",
			expected: "[[false]]",
		},
		{
			query:    "data.test.h(1)",
			expected: "[[false]]",
		},
		{
			query:    "data.test.g(1,2) != 3",
			expected: "[[false]]",
		},
		{
			query:    "data.test.arr[i]",
			expected: "[[1], [true]]",
		},
		{
			query:    "[x | data.test.arr[_] = x]",
			expected: "[[[1, false, true]]]",
		},
		{
			query:    "a = 1; b = 2; a > b",
			expected: `[]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			r := New(
				Query(tc.query),
				Module("", module),
			)
			assertEval(t, r, tc.expected)
		})
	}
}

func TestRegoInputs(t *testing.T) {
	tests := map[string]struct {
		input    any
		expected string
	}{
		"map":  {map[string]bool{"foo": true}, `[[{"foo": true}]]`},
		"int":  {1, `[[1]]`},
		"bool": {false, `[[false]]`},
		"struct": {struct {
			Foo string `json:"baz"`
		}{"bar"}, `[[{"baz":"bar"}]]`},
		"pointer to struct": {&struct {
			Foo string `json:"baz"`
		}{"bar"}, `[[{"baz":"bar"}]]`},
		"pointer to pointer to struct": {
			func() any {
				a := &struct {
					Foo string `json:"baz"`
				}{"bar"}
				return &a
			}(), `[[{"baz":"bar"}]]`},
		"slice":              {[]string{"a", "b"}, `[[["a", "b"]]]`},
		"nil":                {nil, `[[null]]`},
		"slice of interface": {[]any{"a", 2, true}, `[[["a", 2, true]]]`},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			r := New(
				Query("input"),
				Input(tc.input),
				Schemas(nil),
			)
			assertEval(t, r, tc.expected)
		})
	}
}

func TestRegoRewrittenVarsCapture(t *testing.T) {

	ctx := t.Context()

	r := New(
		Query("a := 1; a != 0; a"),
	)

	rs, err := r.Eval(ctx)
	if err != nil || len(rs) != 1 {
		t.Fatalf("Unexpected result: %v (err: %v)", rs, err)
	}

	if !reflect.DeepEqual(rs[0].Bindings["a"], json.Number("1")) {
		t.Fatal("Expected a to be 1 but got:", rs[0].Bindings["a"])
	}

}

func TestRegoDoNotCaptureVoidCalls(t *testing.T) {

	ctx := t.Context()

	r := New(Query("print(1)"))

	rs, err := r.Eval(ctx)
	if err != nil || len(rs) != 1 {
		t.Fatal(err, "rs:", rs)
	}

	if !rs[0].Expressions[0].Value.(bool) {
		t.Fatal("expected expression value to be true")
	}
}

func TestRegoCancellation(t *testing.T) {

	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.sleep",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.Nl,
		),
	})

	topdown.RegisterBuiltinFunc("test.sleep", func(_ topdown.BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		d, _ := time.ParseDuration(string(operands[0].Value.(ast.String)))
		time.Sleep(d)
		return iter(ast.NullTerm())
	})

	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond*10)
	r := New(Query(`test.sleep("1s")`))
	rs, err := r.Eval(ctx)
	cancel()

	if err == nil {
		t.Fatalf("Expected cancellation error but got: %v", rs)
	}
	exp := topdown.Error{Code: topdown.CancelErr, Message: context.DeadlineExceeded.Error()}
	if !errors.Is(err, &exp) {
		t.Errorf("error: expected %v, got: %v", exp, err)
	}
}

func TestRegoCustomBuiltinHalt(t *testing.T) {

	funOpt := Function1(
		&Function{
			Name: "halt_func",
			Decl: types.NewFunction(
				types.Args(types.S),
				types.Nl,
			),
		},
		func(BuiltinContext, *ast.Term) (*ast.Term, error) {
			return nil, NewHaltError(errors.New("stop"))
		},
	)
	r := New(Query(`halt_func("")`), funOpt)
	rs, err := r.Eval(t.Context())
	if err == nil {
		t.Fatalf("Expected halt error but got: %v", rs)
	}
	// exp is the error topdown returns after unwrapping the Halt
	exp := topdown.Error{Code: topdown.BuiltinErr, Message: "halt_func: stop",
		Location: location.NewLocation([]byte(`halt_func("")`), "", 1, 1)}
	if !errors.Is(err, &exp) {
		t.Fatalf("error: expected %v, got: %v", exp, err)
	}
}

func TestRegoMetrics(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m))
	ctx := t.Context()
	_, err := r.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	validateRegoMetrics(t, m, []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_eval_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_module_parse_ns",
		"timer_rego_module_compile_ns",
	})
}

func TestPreparedRegoMetrics(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m))
	ctx := t.Context()
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pq.Eval(ctx, EvalMetrics(m))
	if err != nil {
		t.Fatal(err)
	}

	validateRegoMetrics(t, m, []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_eval_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_module_parse_ns",
		"timer_rego_module_compile_ns",
	})
}

func TestPreparedRegoMetricsPrepareOnly(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m))
	ctx := t.Context()
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pq.Eval(ctx) // No EvalMetrics() passed in
	if err != nil {
		t.Fatal(err)
	}

	validateRegoMetrics(t, m, []string{
		"timer_rego_query_parse_ns",
		"timer_rego_query_compile_ns",
		"timer_rego_module_parse_ns",
		"timer_rego_module_compile_ns",
	})
}

func TestPreparedRegoMetricsEvalOnly(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x")) // No Metrics() passed in
	ctx := t.Context()
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = pq.Eval(ctx, EvalMetrics(m))
	if err != nil {
		t.Fatal(err)
	}

	validateRegoMetrics(t, m, []string{
		"timer_rego_query_eval_ns",
	})
}

func validateRegoMetrics(t *testing.T, m metrics.Metrics, expectedFields []string) {
	t.Helper()

	all := m.All()

	for _, name := range expectedFields {
		value, ok := all[name]
		if !ok {
			t.Errorf("expected to find %v but did not", name)
		}
		if value.(int64) == 0 {
			t.Errorf("expected metric %v to have some non-zero value, but found 0", name)
		}
	}
}

func TestRegoInstrumentExtraEvalCompilerStage(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m), Instrument(true))
	ctx := t.Context()
	_, err := r.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	exp := []string{
		"timer_query_compile_stage_rewrite_to_capture_value_ns",
	}

	all := m.All()

	for _, name := range exp {
		if _, ok := all[name]; !ok {
			t.Errorf("expected to find %v but did not", name)
		}
	}
}

func TestPreparedRegoInstrumentExtraEvalCompilerStage(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m), Instrument(true))
	ctx := t.Context()
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// No metrics flag is passed in, should not affect results for compiler stage
	// but expect to turn off instrumentation for evaluation.
	_, err = pq.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	exp := []string{
		"timer_query_compile_stage_rewrite_to_capture_value_ns",
	}

	nExp := []string{
		"timer_eval_op_plug_ns", // We should *not* see the eval timers
	}

	all := m.All()

	for _, name := range exp {
		if _, ok := all[name]; !ok {
			t.Errorf("expected to find %v but did not", name)
		}
	}

	for _, name := range nExp {
		if _, ok := all[name]; ok {
			t.Errorf("did not expect to find %v", name)
		}
	}
}

func TestRegoInstrumentExtraPartialCompilerStage(t *testing.T) {
	m := metrics.New()
	r := New(Query("foo = 1"), Module("foo.rego", "package x"), Metrics(m), Instrument(true))
	ctx := t.Context()
	_, err := r.Partial(ctx)
	if err != nil {
		t.Fatal(err)
	}

	exp := []string{
		"timer_query_compile_stage_rewrite_equals_ns",
	}

	all := m.All()

	for _, name := range exp {
		if _, ok := all[name]; !ok {
			t.Errorf("Expected to find %v but did not", name)
		}
	}
}

func TestRegoInstrumentExtraPartialResultCompilerStage(t *testing.T) {
	m := metrics.New()
	r := New(Query("input.x"), Module("foo.rego", "package x"), Metrics(m), Instrument(true))
	ctx := t.Context()
	_, err := r.PartialResult(ctx)
	if err != nil {
		t.Fatal(err)
	}

	exp := []string{
		"timer_query_compile_stage_rewrite_for_partial_eval_ns",
	}

	all := m.All()

	for _, name := range exp {
		if _, ok := all[name]; !ok {
			t.Errorf("Expected to find '%v' in metrics\n\nActual:\n %+v", name, all)
		}
	}
}

func TestPreparedRegoTracerNoPropagate(t *testing.T) {
	tracer := topdown.NewBufferTracer()
	mod := `
	package test

	p = {
		input.x == 10
	}
	`
	pq, err := New(
		Query("data"),
		Module("foo.rego", mod),
		Tracer(tracer),
		Input(map[string]any{"x": 10})).PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	_, err = pq.Eval(t.Context()) // no EvalTracer option
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	if len(*tracer) > 0 {
		t.Fatal("expected 0 traces to be collected")
	}
}

func TestPreparedRegoQueryTracerNoPropagate(t *testing.T) {
	tracer := topdown.NewBufferTracer()
	mod := `
	package test

	p = {
		input.x == 10
	}
	`
	pq, err := New(
		Query("data"),
		Module("foo.rego", mod),
		QueryTracer(tracer),
		Input(map[string]any{"x": 10})).PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	_, err = pq.Eval(t.Context()) // no EvalQueryTracer option
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	if len(*tracer) > 0 {
		t.Fatal("expected 0 traces to be collected")
	}
}

func TestRegoDisableIndexing(t *testing.T) {
	tracer := topdown.NewBufferTracer()
	mod := `
	package test
	import rego.v1

	p if {
		input.x = 1
	}

	p if {
		input.y = 1
	}
	`
	pq, err := New(
		Query("data"),
		Module("foo.rego", mod),
	).PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	_, err = pq.Eval(
		t.Context(),
		EvalQueryTracer(tracer),
		EvalRuleIndexing(false),
		EvalInput(map[string]any{"x": 10}),
	)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	var evalNodes []string
	for _, e := range *tracer {
		if e.Op == topdown.EvalOp {
			evalNodes = append(evalNodes, string(e.Node.Loc().Text))
		}
	}

	expectedEvalNodes := []string{
		"input.x = 1",
		"input.y = 1",
	}

	for _, expected := range expectedEvalNodes {
		found := slices.Contains(evalNodes, expected)
		if !found {
			t.Fatalf("Missing expected eval node in trace: %q\nGot: %q\n", expected, evalNodes)
		}
	}
}

func TestRegoDisableIndexingWithMatch(t *testing.T) {
	tracer := topdown.NewBufferTracer()
	mod := `
	package test
	import rego.v1

	p if {
		input.x = 1
	}

	p if {
		input.y = 1
	}
	`
	pq, err := New(
		Query("data"),
		Module("foo.rego", mod),
	).PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	rs, err := pq.Eval(
		t.Context(),
		EvalQueryTracer(tracer),
		EvalRuleIndexing(false),
		EvalInput(map[string]any{"x": 1}),
	)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	assertResultSet(t, rs, `[[{"test": {"p": true}}]]`)

	var evalNodes []string
	for _, e := range *tracer {
		if e.Op == topdown.EvalOp {
			evalNodes = append(evalNodes, string(e.Node.Loc().Text))
		}
	}

	expectedEvalNodes := []string{
		"input.x = 1",
		"input.y = 1",
	}

	for _, expected := range expectedEvalNodes {
		found := slices.Contains(evalNodes, expected)
		if !found {
			t.Fatalf("Missing expected eval node in trace: %q\nGot: %q\n", expected, evalNodes)
		}
	}
}

func TestRegoCatchPathConflicts(t *testing.T) {
	r := New(
		Query("data"),
		Module("test.rego", "package x\np=1"),
		Store(inmem.NewFromObject(map[string]any{
			"x": map[string]any{"p": 1},
		})),
	)

	ctx := t.Context()
	_, err := r.Eval(ctx)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPartialRewriteEquals(t *testing.T) {
	mod := `
	package test
	import rego.v1

	default p = false
	p if {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	ctx := t.Context()
	pq, err := r.Partial(ctx)

	if err != nil {
		t.Fatalf("unexpected error from Rego.Partial(): %s", err.Error())
	}

	// Expect to not have any "support" in the resulting queries
	if len(pq.Support) > 0 {
		t.Errorf("expected to not have any Support in PartialQueries: %+v", pq)
	}

	expectedQuery := "input.x = 1"
	if len(pq.Queries) != 1 {
		t.Errorf("expected 1 query but found %d: %+v", len(pq.Queries), pq)
	}
	if pq.Queries[0].String() != expectedQuery {
		t.Errorf("unexpected query in result, expected='%s' found='%s'",
			expectedQuery, pq.Queries[0].String())
	}
}

// NOTE(sr): https://github.com/open-policy-agent/opa/issues/4345
func TestPrepareAndEvalRaceConditions(t *testing.T) {
	tests := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "object",
			module: `package test
			import rego.v1
			p contains {"x":"y"}`,
			exp: `[[[{"x":"y"}]]]`,
		},
		{
			note: "set",
			module: `package test
			import rego.v1
			p contains {"x"}`,
			exp: `[[[["x"]]]]`,
		},
		{
			note: "array",
			module: `package test
			import rego.v1
			p contains ["x"]`,
			exp: `[[[["x"]]]]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			r := New(
				Query("data.test.p"),
				Module("", tc.module),
				Package("foo"),
			)

			pq, err := r.PrepareForEval(t.Context())
			if err != nil {
				t.Fatalf("Unexpected error: %s", err.Error())
			}

			// run this 1000 times concurrently
			var wg sync.WaitGroup
			wg.Add(1000)
			for range 1000 {
				go func(t *testing.T) {
					t.Helper()
					assertPreparedEvalQueryEval(t, pq, []EvalOption{}, tc.exp)
					wg.Done()
				}(t)
			}
			wg.Wait()
		})
	}
}

func TestPrepareAndEvalNewInput(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
	)

	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[1]]")
}

func TestPrepareAndEvalNewMetrics(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	originalMetrics := metrics.New()

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		Metrics(originalMetrics),
	)

	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	if len(originalMetrics.All()) == 0 {
		t.Errorf("Expected metrics stored on 'originalMetrics' after Prepare()")
	}

	// Reset the original ones (for testing)
	// and make a new one for the Eval
	originalMetrics.Clear()
	newMetrics := metrics.New()

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
		EvalMetrics(newMetrics),
	}, "[[1]]")

	if len(originalMetrics.All()) > 0 {
		t.Errorf("Expected no metrics stored on original Rego object metrics but found: %s",
			originalMetrics.All())
	}

	if len(newMetrics.All()) == 0 {
		t.Errorf("Expected metrics stored on 'newMetrics' after Prepare()")
	}
}

func TestPrepareAndEvalTransaction(t *testing.T) {
	module := `
	package test
	x = data.foo.y
	`
	ctx := t.Context()
	store := mock.New()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	path, ok := storage.ParsePath("/foo")
	if !ok {
		t.Fatalf("Unexpected error parsing path")
	}

	err := storage.MakeDir(ctx, store, txn, path)
	if err != nil {
		t.Fatalf("Unexpected error writing to store: %s", err.Error())
	}

	err = store.Write(ctx, txn, storage.AddOp, path, map[string]any{"y": 1})
	if err != nil {
		t.Fatalf("Unexpected error writing to store: %s", err.Error())
	}

	r := New(
		Query("data.test.x"),
		Module("", module),
		Store(store),
		Transaction(txn),
	)

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	// Base case, expect it to use the transaction provided
	assertPreparedEvalQueryEval(t, pq, []EvalOption{EvalTransaction(txn)}, "[[1]]")

	mockTxn := store.GetTransaction(txn.ID())
	for _, read := range store.Reads {
		if read.Transaction != mockTxn {
			t.Errorf("Found read operation with an invalid transaction, expected: %d, found: %d", mockTxn.ID(), read.Transaction.ID())
		}
	}

	store.AssertValid(t)
	store.Reset()

	// Case with an update to the store and a new transaction
	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	err = store.Write(ctx, txn, storage.AddOp, path, map[string]any{"y": 2})
	if err != nil {
		t.Fatalf("Unexpected error writing to store: %s", err.Error())
	}

	// Expect the new result from the updated value on this transaction
	assertPreparedEvalQueryEval(t, pq, []EvalOption{EvalTransaction(txn)}, "[[2]]")

	err = store.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("Unexpected error committing to store: %s", err)
	}

	newMockTxn := store.GetTransaction(txn.ID())
	for _, read := range store.Reads {
		if read.Transaction != newMockTxn {
			t.Errorf("Found read operation with an invalid transaction, expected: %d, found: %d", mockTxn.ID(), read.Transaction.ID())
		}
	}

	store.AssertValid(t)
	store.Reset()

	// Case with no transaction provided, should create a new one and see the latest value
	txn = storage.NewTransactionOrDie(ctx, store, storage.WriteParams)
	err = store.Write(ctx, txn, storage.AddOp, path, map[string]any{"y": 3})
	if err != nil {
		t.Fatalf("Unexpected error writing to store: %s", err.Error())
	}
	err = store.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("Unexpected error committing to store: %s", err)
	}

	assertPreparedEvalQueryEval(t, pq, nil, "[[3]]")

	if len(store.Transactions) != 2 {
		t.Fatalf("Expected only two transactions on store, found %d", len(store.Transactions))
	}

	autoTxn := store.Transactions[1]
	for _, read := range store.Reads {
		if read.Transaction != autoTxn {
			t.Errorf("Found read operation with an invalid transaction, expected: %d, found: %d", autoTxn, read.Transaction.ID())
		}
	}
	store.AssertValid(t)

}

func TestPrepareAndEvalIdempotent(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
	)

	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	// Expect evaluating the same thing >1 time gives the same
	// results each time.
	for range 5 {
		assertPreparedEvalQueryEval(t, pq, []EvalOption{
			EvalInput(map[string]int{"y": 1}),
		}, "[[1]]")
	}
}

func TestPrepareAndEvalOriginal(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		Input(map[string]int{"y": 2}),
	)

	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[1]]")

	// Even after prepare and eval with different input
	// expect that the original Rego object behaves
	// as expected for Eval.

	assertEval(t, r, "[[2]]")
}

func TestPrepareAndEvalOnlyOneErrorOccurredPrintOnce(t *testing.T) {
	module := `
	package test
    package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		Input(map[string]int{"y": 2}),
	)

	_, err := r.PrepareForEval(t.Context())
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
	if strings.Count(err.Error(), "1 error occurred") > 1 {
		t.Fatalf("Expected to print '1 error occurred' only once")
	}
}

func TestPrepareAndEvalNewPrintHook(t *testing.T) {
	module := `
	package test
	import rego.v1
	x if { print(input) }
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		EnablePrintStatements(true),
	)

	pq, err := r.PrepareForEval(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	var buf0 bytes.Buffer
	ph0 := topdown.NewPrintHook(&buf0)
	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput("hello"),
		EvalPrintHook(ph0),
	}, "[[true]]")

	if exp, act := "hello\n", buf0.String(); exp != act {
		t.Fatalf("print hook, expected %q, got %q", exp, act)
	}

	// repeat
	var buf1 bytes.Buffer
	ph1 := topdown.NewPrintHook(&buf1)
	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput("world"),
		EvalPrintHook(ph1),
	}, "[[true]]")

	if exp, act := "world\n", buf1.String(); exp != act {
		t.Fatalf("print hook, expected %q, got %q", exp, act)
	}
}

func TestPrepareAndPartialResult(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		Input(map[string]int{"y": 2}),
	)

	ctx := t.Context()

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[1]]")

	// Even after prepare and eval with different input
	// expect that the original Rego object behaves
	// as expected for PartialResult.

	partial, err := r.PartialResult(ctx)
	if err != nil {
		t.Fatal(err)
	}

	r2 := partial.Rego(
		Input(map[string]int{"y": 7}),
	)
	assertEval(t, r2, "[[7]]")
}

func TestPrepareWithPartialEval(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
	)

	ctx := t.Context()

	// Prepare the query and partially evaluate it
	pq, err := r.PrepareForEval(ctx, WithPartialEval())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[1]]")
}

func TestPrepareAndPartial(t *testing.T) {
	mod := `
	package test
	import rego.v1

	default p = false
	p if {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	ctx := t.Context()

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"x": 1}),
	}, "[[true]]")

	// Even after prepare and eval with different input
	// expect that the original Rego object behaves
	// as expected for Partial.

	partialQuery, err := r.Partial(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expectedQuery := "input.x = 1"
	if len(partialQuery.Queries) != 1 {
		t.Errorf("expected 1 query but found %d: %+v", len(partialQuery.Queries), pq)
	}
	if partialQuery.Queries[0].String() != expectedQuery {
		t.Errorf("unexpected query in result, expected='%s' found='%s'",
			expectedQuery, partialQuery.Queries[0].String())
	}
}

func TestPartialWithRegoV1(t *testing.T) {
	tests := []struct {
		note       string
		module     string
		expQuery   string
		expSupport string
	}{
		{
			note: "No imports",
			module: `package test
				p[k] contains v if {
					k := "foo"
					v := input.v
				}`,
			expQuery: `data.partial.test.p = x`,
			expSupport: `package partial.test.p

foo contains __local1__1 if { __local1__1 = input.v }`,
		},
		{
			note: "rego.v1 imported",
			module: `package test
				import rego.v1
				p[k] contains v if {
					k := "foo"
					v := input.v
				}`,
			expQuery: `data.partial.test.p = x`,
			expSupport: `package partial.test.p

foo contains __local1__1 if { __local1__1 = input.v }`,
		},
		{
			note: "future.keywords imported",
			module: `package test
				import future.keywords
				p[k] contains v if {
					k := "foo"
					v := input.v
				}`,
			expQuery: `data.partial.test.p = x`,
			expSupport: `package partial.test.p

foo contains __local1__1 if { __local1__1 = input.v }`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			r := New(
				Query("data.test.p = x"),
				Module("test.rego", tc.module),
				SetRegoVersion(ast.RegoV1),
			)

			ctx := t.Context()

			partialQuery, err := r.Partial(ctx)
			if err != nil {
				t.Fatal(err)
			}

			actualQuery := partialQuery.Queries[0].String()
			if tc.expQuery != actualQuery {
				t.Fatalf("Expected partial query to be:\n\n%s\n\nbut got:\n\n%s", tc.expQuery, actualQuery)
			}

			actualSupport := partialQuery.Support[0].String()
			if tc.expSupport != actualSupport {
				t.Fatalf("Expected support module to be:\n\n%s\n\nbut got:\n\n%s", tc.expSupport, actualSupport)
			}
		})
	}
}

func TestPartialNamespace(t *testing.T) {

	r := New(
		PartialNamespace("foo"),
		Query("data.test.p = x"),
		SetRegoVersion(ast.RegoV1),
		Module("test.rego", `
			package test

			default p = false

			p if { input.x = 1 }
		`),
	)

	pq, err := r.Partial(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	expQuery := ast.MustParseBody(`data.foo.test.p = x`)

	if len(pq.Queries) != 1 || !pq.Queries[0].Equal(expQuery) {
		t.Fatalf("Expected exactly one query %v but got: %v", expQuery, pq.Queries)
	}

	expSupport := ast.MustParseModuleWithOpts(`
		package foo.test

		default p = false

		p if { input.x = 1 }
	`, ast.ParserOptions{RegoVersion: ast.RegoV1})

	if len(pq.Support) != 1 || !pq.Support[0].Equal(expSupport) {
		t.Fatalf("Expected exactly one support:\n\n%v\n\nGot:\n\n%v", expSupport, pq.Support[0])
	}
}

func TestPrepareAndCompile(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
	)

	ctx := t.Context()

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[1]]")

	// Ensure that Compile still works after Prepare
	// and its Eval has been called.
	_, err = r.Compile(ctx)
	if err != nil {
		t.Errorf("Unexpected error when compiling: %s", err.Error())
	}
}

func TestPartialResultWithInput(t *testing.T) {
	mod := `
	package test
	import rego.v1

	default p = false
	p if {
		input.x == 1
	}
	`
	r := New(
		Query("data.test.p"),
		Module("test.rego", mod),
	)

	ctx := t.Context()
	pr, err := r.PartialResult(ctx)

	if err != nil {
		t.Fatalf("unexpected error from Rego.PartialResult(): %s", err.Error())
	}

	r2 := pr.Rego(
		Input(map[string]int{"x": 1}),
	)

	assertEval(t, r2, "[[true]]")
}

func TestPartialResultWithNamespace(t *testing.T) {
	mod := `
	package test
	import rego.v1

	p if {
		true
	}
	`
	c := ast.NewCompiler()
	r := New(
		Query("data.test.p"),
		Module("test.rego", mod),
		PartialNamespace("test_ns1"),
		Compiler(c),
	)

	ctx := t.Context()
	pr, err := r.PartialResult(ctx)

	if err != nil {
		t.Fatalf("unexpected error from Rego.PartialResult(): %s", err.Error())
	}

	expectedQuery := "data.test_ns1.__result__"
	if pr.body.String() != expectedQuery {
		t.Fatalf("Expected partial result query %s got %s", expectedQuery, pr.body)
	}

	r2 := pr.Rego()

	assertEval(t, r2, "[[true]]")

	if len(c.Modules) != 2 {
		t.Fatalf("Expected two modules on the compiler, got: %v", c.Modules)
	}

	expectedModuleID := "__partialresult__test_ns1__"
	if _, ok := c.Modules[expectedModuleID]; !ok {
		t.Fatalf("Expected to find module %s in compiler Modules, got: %v", expectedModuleID, c.Modules)
	}
}

func TestPreparedPartialResultWithTracer(t *testing.T) {
	mod := `
	package test
	import rego.v1

	default p = false
	p if {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	tracer := topdown.NewBufferTracer()

	ctx := t.Context()
	pq, err := r.PrepareForPartial(ctx)
	if err != nil {
		t.Fatalf("unexpected error from Rego.PrepareForPartial(): %s", err.Error())
	}

	pqs, err := pq.Partial(ctx, EvalTracer(tracer))
	if err != nil {
		t.Fatalf("unexpected error from PreparedEvalQuery.Partial(): %s", err.Error())
	}

	expectedQuery := "input.x = 1"
	if len(pqs.Queries) != 1 {
		t.Errorf("expected 1 query but found %d: %+v", len(pqs.Queries), pqs)
	}
	if pqs.Queries[0].String() != expectedQuery {
		t.Errorf("unexpected query in result, expected='%s' found='%s'",
			expectedQuery, pqs.Queries[0].String())
	}

	if len(*tracer) == 0 {
		t.Errorf("Expected buffer tracer to contain > 0 traces")
	}
}

func TestPreparedPartialResultWithQueryTracer(t *testing.T) {
	mod := `
	package test
	import rego.v1

	default p = false
	p if {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	tracer := topdown.NewBufferTracer()

	ctx := t.Context()
	pq, err := r.PrepareForPartial(ctx)
	if err != nil {
		t.Fatalf("unexpected error from Rego.PrepareForPartial(): %s", err.Error())
	}

	pqs, err := pq.Partial(ctx, EvalQueryTracer(tracer))
	if err != nil {
		t.Fatalf("unexpected error from PreparedEvalQuery.Partial(): %s", err.Error())
	}

	expectedQuery := "input.x = 1"
	if len(pqs.Queries) != 1 {
		t.Errorf("expected 1 query but found %d: %+v", len(pqs.Queries), pqs)
	}
	if pqs.Queries[0].String() != expectedQuery {
		t.Errorf("unexpected query in result, expected='%s' found='%s'",
			expectedQuery, pqs.Queries[0].String())
	}

	if len(*tracer) == 0 {
		t.Errorf("Expected buffer tracer to contain > 0 traces")
	}
}

func TestPartialResultSetsValidConflictChecker(t *testing.T) {
	mod := `
	package test
	import rego.v1

	p if {
		true
	}
	`

	c := ast.NewCompiler().WithPathConflictsCheck(func(_ []string) (bool, error) {
		t.Fatal("Conflict check should not have been called")
		return false, nil
	})

	r := New(
		Query("data.test.p"),
		Module("test.rego", mod),
		PartialNamespace("test_ns1"),
		Compiler(c),
	)

	ctx := t.Context()
	pr, err := r.PartialResult(ctx)

	if err != nil {
		t.Fatalf("unexpected error from Rego.PartialResult(): %s", err.Error())
	}

	r2 := pr.Rego()

	assertEval(t, r2, "[[true]]")
}

func TestMissingLocation(t *testing.T) {

	// Create a query programmatically and evaluate it. The Location information
	// is not set so the resulting expression value will not have it.
	r := New(ParsedQuery(ast.NewBody(ast.NewExpr(ast.BooleanTerm(true)))))
	rs, err := r.Eval(t.Context())

	if err != nil {
		t.Fatal(err)
	} else if len(rs) == 0 || !rs[0].Expressions[0].Value.(bool) {
		t.Fatal("Unexpected result set:", rs)
	}

	if rs[0].Expressions[0].Location != nil {
		t.Fatal("Expected location data to be unset.")
	}
}

func TestBundlePassing(t *testing.T) {

	opaBundle := bundle.Bundle{
		Modules: []bundle.ModuleFile{
			{
				Path: "policy.rego",
				Parsed: ast.MustParseModule(`package foo
                         allow = true`),
				Raw: []byte(`package foo
                         allow = true`),
			},
		},
		Manifest: bundle.Manifest{Revision: "test", Roots: &[]string{"/"}},
	}

	// Pass a bundle
	r := New(
		ParsedBundle("123", &opaBundle),
		Query("x = data.foo.allow"),
	)

	res, err := r.Eval(t.Context())

	if err != nil {
		t.Fatal(err)
	}
	assertResultSet(t, res, `[[true]]`)
}

func TestModulePassing(t *testing.T) {

	// This module will not be loaded since it has the same filename as the
	// file2.rego below and the raw modules override parsed modules.
	module1, err := ast.ParseModule("file2.rego", `package file2

	p = "deadbeef"
	`)
	if err != nil {
		t.Fatal(err)
	}

	r := New(
		Query("data"),
		Module("file1.rego", `package file1

		p = 1
		`),
		Module("file2.rego", `package file2

		p = 2`),
		ParsedModule(module1),
		ParsedModule(ast.MustParseModule(`package file4

		p = 4`)),
	)

	rs, err := r.Eval(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	exp := util.MustUnmarshalJSON([]byte(`
	{
		"file1": {
			"p": 1
		},
		"file2": {
			"p": 2
		},
		"file4": {
			"p": 4
		}
	}
	`))

	if !reflect.DeepEqual(rs[0].Expressions[0].Value, exp) {
		t.Fatalf("Expected %v but got %v", exp, rs[0].Expressions[0].Value)
	}
}

func TestUnsafeBuiltins(t *testing.T) {

	ctx := t.Context()

	unsafeCountExpr := "unsafe built-in function calls in expression: count"
	unsafeCountExprWith := `with keyword replacing built-in function: target must not be unsafe: "count"`

	t.Run("unsafe query", func(t *testing.T) {
		r := New(
			Query(`count([1, 2, 3])`),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExpr) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("unsafe query, 'with' replacement", func(t *testing.T) {
		r := New(
			Query(`is_array([1, 2, 3]) with is_array as count`),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExprWith) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("unsafe module", func(t *testing.T) {
		r := New(
			Query(`data.pkg.deny`),
			Module("pkg.rego", `package pkg
			import rego.v1
			deny if{
				count(input.requests) > 10
			}
			`),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExpr) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("unsafe module, 'with' replacement in query", func(t *testing.T) {
		r := New(
			Query(`data.pkg.deny with is_array as count`),
			Module("pkg.rego", `package pkg
			import rego.v1
			deny if {
				is_array(input.requests) > 10
			}
			`),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExprWith) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("unsafe module, 'with' replacement in module", func(t *testing.T) {
		r := New(
			Query(`data.pkg.deny`),
			Module("pkg.rego", `package pkg
			import rego.v1
			deny if {
				is_array(input.requests) > 10 with is_array as count
			}
			`),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExprWith) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("inherit in query", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": {}})),
			Query("count([])"),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExpr) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("inherit in query, 'with' replacement", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": {}})),
			Query("is_array([]) with is_array as count"),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExprWith) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("override/disable in query", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": {}})),
			UnsafeBuiltins(map[string]struct{}{}),
			Query("count([])"),
		)
		if _, err := r.Eval(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("override/change in query", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": {}})),
			UnsafeBuiltins(map[string]struct{}{"max": {}}),
			Query("count([]); max([1,2])"),
		)

		_, err := r.Eval(ctx)
		if err == nil || err.Error() != "1 error occurred: 1:12: rego_type_error: unsafe built-in function calls in expression: max" {
			t.Fatalf("expected error for max but got: %v", err)
		}
	})

	t.Run("ignore if given compiler", func(_ *testing.T) {
		r := New(
			Compiler(ast.NewCompiler()),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
			Query("data.test.p = 0"),
			Module("test.rego", `package test

			p = count([])`),
		)
		rs, err := r.Eval(t.Context())
		if err != nil || len(rs) != 1 {
			log.Fatalf("Unexpected error or result. Result: %v. Error: %v", rs, err)
		}
	})
}

func TestPreparedQueryGetModules(t *testing.T) {
	mods := map[string]string{
		"a.rego": "package a\np = 1",
		"b.rego": "package b\nq = 1",
		"c.rego": "package c\nr = 1",
	}

	regoArgs := make([]func(r *Rego), 0, len(mods)+1)

	for name, mod := range mods {
		regoArgs = append(regoArgs, Module(name, mod))
	}

	regoArgs = append(regoArgs, Query("data"))

	ctx := t.Context()
	pq, err := New(regoArgs...).PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	actualMods := pq.Modules()

	if len(actualMods) != len(mods) {
		t.Fatalf("Expected %d modules, got %d", len(mods), len(actualMods))
	}

	for name, actualMod := range actualMods {
		expectedMod, found := mods[name]
		if !found {
			t.Fatalf("Unexpected module %s", name)
		}
		if actualMod.String() != ast.MustParseModule(expectedMod).String() {
			t.Fatalf("Modules for %s do not match.\n\nExpected:\n%s\n\nActual:\n%s\n\n",
				name, actualMod.String(), expectedMod)
		}
	}
}

func TestRegoEvalWithFile(t *testing.T) {
	files := map[string]string{
		"x/x.rego": "package x\np = 1",
		"x/x.json": `{"y": "foo"}`,
	}

	test.WithTempFS(files, func(path string) {
		ctx := t.Context()

		pq, err := New(
			Load([]string{path}, nil),
			Query("data"),
		).PrepareForEval(ctx)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		rs, err := pq.Eval(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		assertResultSet(t, rs, `[[{"x":{"p":1,"y":"foo"}}]]`)
	})
}

func TestRegoEvalWithBundle(t *testing.T) {
	files := map[string]string{
		"x/x.rego":            "package x\np = data.x.b",
		"x/data.json":         `{"b": "bar"}`,
		"other/not-data.json": `{"ignored": "data"}`,
	}

	test.WithTempFS(files, func(path string) {
		ctx := t.Context()

		pq, err := New(
			LoadBundle(path),
			Query("data.x.p"),
		).PrepareForEval(ctx)

		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		rs, err := pq.Eval(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		assertResultSet(t, rs, `[["bar"]]`)

		mods := pq.Modules()
		if exp, act := 1, len(mods); exp != act {
			t.Fatalf("expected %d modules, found %d", exp, act)
		}
		for act := range mods {
			if exp := filepath.Join(path, "x", "x.rego"); exp != act {
				t.Errorf("expected module name %q, got %q", exp, act)
			}
		}
	})
}

func TestRegoEvalWithBundleURL(t *testing.T) {
	files := map[string]string{
		"x/x.rego": "package x\np = data.x.b",
	}

	test.WithTempFS(files, func(path string) {
		ctx := t.Context()
		pq, err := New(
			LoadBundle("file://"+path),
			Query("data.x.p"),
		).PrepareForEval(ctx)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		mods := pq.Modules()
		if exp, act := 1, len(mods); exp != act {
			t.Fatalf("expected %d modules, found %d", exp, act)
		}
		for act := range mods {
			if exp := filepath.Join(path, "x", "x.rego"); exp != act {
				t.Errorf("expected module name %q, got %q", exp, act)
			}
		}
	})
}

func TestRegoEvalPoliciesInStore(t *testing.T) {
	store := mock.New()
	ctx := t.Context()
	txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

	err := store.UpsertPolicy(ctx, txn, "a.rego", []byte("package a\np=1"))
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	err = store.Commit(ctx, txn)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	pq, err := New(
		Store(store),
		Module("b.rego", "package b\np = data.a.p"),
		Query("data.b.p"),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	rs, err := pq.Eval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertResultSet(t, rs, `[[1]]`)
}

func TestRegoEvalModulesOnCompiler(t *testing.T) {
	compiler := ast.NewCompiler()

	compiler.Compile(map[string]*ast.Module{
		"a.rego": ast.MustParseModule("package a\np = 1"),
	})

	if len(compiler.Errors) > 0 {
		t.Fatalf("Unexpected compile errors: %s", compiler.Errors)
	}

	ctx := t.Context()

	pq, err := New(
		Compiler(compiler),
		Query("data.a.p"),
		Schemas(nil),
	).PrepareForEval(ctx)

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	rs, err := pq.Eval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	assertResultSet(t, rs, `[[1]]`)
}

func TestRegoEvalWithRegoV1(t *testing.T) {
	tests := []struct {
		note           string
		regoVersion    ast.RegoVersion
		policies       map[string]string
		query          string
		expectedResult string
		expectedErr    string
	}{
		{
			note:        "Rego v0",
			regoVersion: ast.RegoV0,
			policies: map[string]string{
				"policy.rego": `package test
				x[y] { y := 1 }`,
			},
			expectedResult: `[[{"x": [1]}]]`,
		},
		{
			note:        "Rego v0, forced v1 compatibility",
			regoVersion: ast.RegoV0CompatV1,
			policies: map[string]string{
				"policy.rego": `package test
				import rego.v1
				x contains y if { y := 1 }`,
			},
			expectedResult: `[[{"x": [1]}]]`,
		},
		{
			note:        "Rego v0, forced v1 compatibility, invalid rule head",
			regoVersion: ast.RegoV0CompatV1,
			policies: map[string]string{
				"policy.rego": `package test
				import future.keywords.contains
				x contains y { y := 1 }`,
			},
			expectedErr: "rego_parse_error: `if` keyword is required before rule body",
		},
		{
			note:        "Rego v0, forced v1 compatibility, missing required imports",
			regoVersion: ast.RegoV0CompatV1,
			policies: map[string]string{
				"policy.rego": `package test
				x contains y if { y := 1 }`,
			},
			expectedErr: "rego_parse_error: var cannot be used for rule name", // FIXME: Improve error message
		},
		{
			note:        "Rego v1",
			regoVersion: ast.RegoV1,
			policies: map[string]string{
				"policy.rego": `package test
				x contains y if { y := 1 }`,
			},
			expectedResult: `[[{"x": [1]}]]`,
		},
		{
			note:        "Rego v1, invalid rule head",
			regoVersion: ast.RegoV1,
			policies: map[string]string{
				"policy.rego": `package test
				x contains y { y := 1 }`,
			},
			expectedErr: "rego_parse_error: `if` keyword is required before rule body",
		},
		{
			note:        "Rego v1, multiple files",
			regoVersion: ast.RegoV1,
			policies: map[string]string{
				"one.rego": `package test
				x contains v if { v := 1 }`,
				"two.rego": `package test
				import rego.v1
				y contains v if { v := 1 }`,
				"three.rego": `package test
				import future.keywords
				z contains v if { v := 1 }`,
			},
			expectedResult: `[[{"x": [1], "y": [1], "z": [1]}]]`,
		},
	}

	setup := []struct {
		name    string
		options func(path string, policies map[string]string, t *testing.T, ctx context.Context) []func(*Rego)
	}{
		{
			name: "File",
			options: func(path string, _ map[string]string, _ *testing.T, _ context.Context) []func(*Rego) {
				return []func(*Rego){
					Load([]string{path}, nil),
				}
			},
		},
		{
			name: "Bundle",
			options: func(path string, _ map[string]string, _ *testing.T, _ context.Context) []func(*Rego) {
				return []func(*Rego){
					LoadBundle(path),
				}
			},
		},
		{
			name: "Bundle URL",
			options: func(path string, _ map[string]string, _ *testing.T, _ context.Context) []func(*Rego) {
				return []func(*Rego){
					LoadBundle("file://" + path),
				}
			},
		},
		{
			name: "Store",
			options: func(_ string, policies map[string]string, t *testing.T, ctx context.Context) []func(*Rego) {
				t.Helper()
				store := mock.New()
				txn := storage.NewTransactionOrDie(ctx, store, storage.WriteParams)

				for name, policy := range policies {
					err := store.UpsertPolicy(ctx, txn, name, []byte(policy))
					if err != nil {
						t.Fatalf("Unexpected error: %s", err)
					}
				}

				err := store.Commit(ctx, txn)
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				return []func(*Rego){
					// This extra module is required for modules in the store to be parsed
					Module("extra.rego", "package extra\np = 1"),
					Store(store),
				}
			},
		},
	}

	for _, s := range setup {
		for _, tc := range tests {
			t.Run(fmt.Sprintf("%s: %s", s.name, tc.note), func(t *testing.T) {
				test.WithTempFS(tc.policies, func(path string) {
					ctx := t.Context()

					options := append(s.options(path, tc.policies, t, ctx),
						Query("data.test"),
						func(r *Rego) {
							SetRegoVersion(tc.regoVersion)(r)
						},
					)

					pq, err := New(
						options...,
					).PrepareForEval(ctx)

					if tc.expectedErr != "" {
						if err == nil {
							t.Fatal("Expected error, got none")
						}
						if !strings.Contains(err.Error(), tc.expectedErr) {
							t.Fatalf("Expected error:\n\n%s\n\ngot:\n\n%s", err, tc.expectedErr)
						}
					} else {
						if err != nil {
							t.Fatalf("Unexpected error: %s", err)
						}

						rs, err := pq.Eval(ctx)
						if err != nil {
							t.Fatalf("Unexpected error: %s", err)
						}

						if tc.expectedResult != "" {
							assertResultSet(t, rs, tc.expectedResult)
						}
					}
				})
			})
		}
	}
}

func TestRegoLoadFilesWithProvidedStore(t *testing.T) {
	ctx := t.Context()
	store := mock.New()

	files := map[string]string{
		"x.rego": "package x\np = data.x.b",
	}

	test.WithTempFS(files, func(path string) {
		pq, err := New(
			Store(store),
			Query("data"),
			Load([]string{path}, nil),
		).PrepareForEval(ctx)

		if err == nil {
			t.Fatal("Expected an error but err == nil")
		}

		if pq.r != nil {
			t.Fatalf("Expected pq.r == nil, got: %+v", pq)
		}
	})
}

func TestRegoLoadBundleWithProvidedStore(t *testing.T) {
	ctx := t.Context()
	store := mock.New()

	files := map[string]string{
		"x/x.rego": "package x\np = data.x.b",
	}

	test.WithTempFS(files, func(path string) {
		pq, err := New(
			Store(store),
			Query("data"),
			LoadBundle(path),
		).PrepareForEval(ctx)

		if err == nil {
			t.Fatal("Expected an error but err == nil")
		}

		if pq.r != nil {
			t.Fatalf("Expected pq.r == nil, got: %+v", pq)
		}
	})
}

func TestRegoCustomBuiltinPartialPropagate(t *testing.T) {
	mod := `package test
	import rego.v1

	p if {
		x = trim_and_split(input.foo, "/")
		x == ["foo", "bar", "baz"]
	}
	`

	originalRego := New(
		Module("test.rego", mod),
		Query(`data.test.p`),
		Function2(
			&Function{
				Name: "trim_and_split",
				Decl: types.NewFunction(
					types.Args(types.S, types.S), // two string inputs
					types.NewArray(nil, types.S), // variable-length string array output
				),
			},
			func(_ BuiltinContext, a, b *ast.Term) (*ast.Term, error) {

				str, ok1 := a.Value.(ast.String)
				delim, ok2 := b.Value.(ast.String)

				// The function is undefined for non-string inputs. Built-in
				// functions should only return errors in unrecoverable cases.
				if !ok1 || !ok2 {
					return nil, nil
				}

				result := strings.Split(strings.Trim(string(str), string(delim)), string(delim))

				arr := make([]*ast.Term, len(result))
				for i := range result {
					arr[i] = ast.StringTerm(result[i])
				}

				return ast.ArrayTerm(arr...), nil
			},
		),
	)

	pr, err := originalRego.PartialResult(t.Context())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	rs, err := pr.Rego(
		Input(map[string]any{"foo": "/foo/bar/baz/"}),
	).Eval(t.Context())

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	assertResultSet(t, rs, `[[true]]`)

}

func TestRegoPartialResultRecursiveRefs(t *testing.T) {
	r := New(Query("data"), Module("test.rego", `package foo.bar
	import rego.v1

	default p = false

	p if { input.x = 1 }`))

	_, err := r.PartialResult(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}

	if !IsPartialEvaluationNotEffectiveErr(err) {
		t.Fatal("expected ineffective partial eval error")
	}

}

func TestSkipPartialNamespaceOption(t *testing.T) {
	r := New(Query("data.test.p"), Module("example.rego", `
		package test
		import rego.v1

		default p = false

		p = true if { input }
	`), SkipPartialNamespace(true))

	pq, err := r.Partial(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(pq.Queries) != 1 || !pq.Queries[0].Equal(ast.MustParseBody("data.test.p")) {
		t.Fatal("expected exactly one query and for reference to not have been rewritten but got:", pq.Queries)
	}

	if len(pq.Support) != 1 || !pq.Support[0].Package.Equal(ast.MustParsePackage("package test")) {
		t.Fatal("expected exactly one support and for package to be same as input but got:", pq.Support)
	}
}

func TestShallowInliningOption(t *testing.T) {
	r := New(Query("data.test.p = true"),
		SetRegoVersion(ast.RegoV1),
		Module("example.rego", `
			package test

			p if {
				q = true
			}

			q if {
				input.x = r
			}

			r = 7
		`),
		ShallowInlining(true))

	pq, err := r.Partial(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if len(pq.Queries) != 1 || !pq.Queries[0].Equal(ast.MustParseBody("data.partial.test.p = true")) {
		t.Fatal("expected exactly one query and ref to be rewritten but got:", pq.Queries)
	}

	exp := ast.MustParseModuleWithOpts(`
		package partial.test

		p if { data.partial.test.q = true }
		q if { 7 = input.x }
	`, ast.ParserOptions{RegoVersion: ast.RegoV1})

	if len(pq.Support) != 1 || !pq.Support[0].Equal(exp) {
		t.Fatal("expected module:", exp, "\n\ngot module:", pq.Support[0])
	}
}

func TestRegoPartialResultSortedRules(t *testing.T) {
	r := New(Query("data.test.p"),
		SetRegoVersion(ast.RegoV1),
		Module("example.rego", `
			package test

			default p = false

			p if {
				r = (input.d * input.a) + input.c
				r < s
			}

			p if {
				r = (input.d * input.b) + input.c
				r < s
			}

			s = 100
	`))

	pq, err := r.Partial(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	// Without sorting of support rules, the output of the above partial evaluation
	// resulted in a random order of the support rules (in this case two different possible outputs)
	exp := ast.MustParseModuleWithOpts(
		`package partial.test

		default p = false

		p = true if { lt(plus(mul(input.d, input.a), input.c), 100) }
		p = true if { lt(plus(mul(input.d, input.b), input.c), 100) }
		`,
		ast.ParserOptions{RegoVersion: ast.RegoV1})

	if len(pq.Support) != 1 || !pq.Support[0].Equal(exp) {
		t.Fatal("expected module:", exp, "\n\ngot module:", pq.Support[0])
	}

}

func TestPrepareWithEmptyModule(t *testing.T) {
	_, err := New(
		Query("d"),
		Module("example.rego", ""),
	).PrepareForEval(t.Context())

	expected := "1 error occurred: example.rego:0: rego_parse_error: empty module"
	if err == nil || err.Error() != expected {
		t.Fatalf("Expected error %s, got %s", expected, err)
	}
}

func TestPrepareWithWasmTargetNotSupported(t *testing.T) {
	files := map[string]string{
		"x/x.rego":     "package x\np = data.x.b",
		"x/data.json":  `{"b": "bar"}`,
		"/policy.wasm": `modules-compiled-as-wasm-binary`,
	}

	test.WithTempFS(files, func(path string) {
		ctx := t.Context()

		_, err := New(
			LoadBundle(path),
			Query("data.x.p"),
			Target("wasm"),
		).PrepareForEval(ctx)

		expected := "wasm target not supported"
		if err == nil || err.Error() != expected {
			t.Fatalf("Expected error %s, got %s", expected, err)
		}
	})
}

func TestEvalWithInterQueryCache(t *testing.T) {
	newHeaders := map[string][]string{"Cache-Control": {"max-age=290304000, public"}}

	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		headers := w.Header()

		maps.Copy(headers, newHeaders)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"x": 1}`))
	}))
	defer ts.Close()
	query := fmt.Sprintf(`http.send({"method": "get", "url": "%s", "force_json_decode": true, "cache": true})`, ts.URL)

	// add an inter-query cache
	config, _ := cache.ParseCachingConfig(nil)
	interQueryCache := cache.NewInterQueryCache(config)

	ctx := t.Context()
	_, err := New(Query(query), InterQueryBuiltinCache(interQueryCache)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// eval again with same query
	// this request should be served by the cache
	_, err = New(Query(query), InterQueryBuiltinCache(interQueryCache)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(requests) != 1 {
		t.Fatal("Expected server to be called only once")
	}
}

func TestEvalWithInterQueryValueCache(t *testing.T) {
	ctx := t.Context()

	// add an inter-query value cache
	config, _ := cache.ParseCachingConfig(nil)
	interQueryValueCache := cache.NewInterQueryValueCache(ctx, config)

	m := metrics.New()

	query := `regex.match("foo.*", "foobar")`
	_, err := New(Query(query), InterQueryBuiltinValueCache(interQueryValueCache), Metrics(m)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// eval again with same query
	// this request should be served by the cache
	_, err = New(Query(query), InterQueryBuiltinValueCache(interQueryValueCache), Metrics(m)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if exp, act := uint64(1), m.Counter("rego_builtin_regex_interquery_value_cache_hits").Value(); exp != act {
		t.Fatalf("expected %d cache hits, got %d", exp, act)
	}

	query = `glob.match("*.example.com", ["."], "api.example.com")`
	_, err = New(Query(query), InterQueryBuiltinValueCache(interQueryValueCache), Metrics(m)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// eval again with same query
	// this request should be served by the cache
	_, err = New(Query(query), InterQueryBuiltinValueCache(interQueryValueCache), Metrics(m)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = New(Query(query), InterQueryBuiltinValueCache(interQueryValueCache), Metrics(m)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if exp, act := uint64(2), m.Counter("rego_builtin_glob_interquery_value_cache_hits").Value(); exp != act {
		t.Fatalf("expected %d cache hits, got %d", exp, act)
	}
}

// We use http.send to ensure the NDBuiltinCache is involved.
func TestEvalWithNDCache(t *testing.T) {
	var requests []*http.Request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r)
		_, _ = w.Write([]byte(`{"x": 1}`))
	}))
	defer ts.Close()
	query := fmt.Sprintf(`http.send({"method": "get", "url": "%s", "force_json_decode": true})`, ts.URL)

	// Set up the ND cache, and put in some arbitrary constants for the first K/V pair.
	arbitraryKey := ast.Number(strconv.Itoa(2015))
	arbitraryValue := ast.String("First commit year")
	ndBC := builtins.NDBCache{}
	ndBC.Put("arbitrary_experiment", arbitraryKey, arbitraryValue)

	// Query execution of http.send should add an entry to the NDBuiltinCache.
	ctx := t.Context()
	_, err := New(Query(query), NDBuiltinCache(ndBC)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Check and make sure we got exactly 2x items back in the ND builtin cache.
	// NDBuiltinCache always has the structure: map[ast.String]map[ast.Array]ast.Value
	if len(ndBC) != 2 {
		t.Fatalf("Expected exactly 2 items in non-deterministic builtin cache. Found %d items.\n", len(ndBC))
	}
	// Check the cached k/v types for the HTTP section of the cache.
	if cachedResults, ok := ndBC["http.send"]; ok {
		err := cachedResults.Iter(func(k, v *ast.Term) error {
			if _, ok := k.Value.(*ast.Array); !ok {
				t.Fatalf("http.send failed to store Object key in the ND builtins cache")
			}
			if _, ok := v.Value.(ast.Object); !ok {
				t.Fatalf("http.send failed to store Object value in the ND builtins cache")
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// Ensure our original arbitrary data in the cache was preserved.
	if v, ok := ndBC.Get("arbitrary_experiment", arbitraryKey); ok {
		if v != arbitraryValue {
			t.Fatalf("Non-deterministic builtins cache value was mangled. Expected: %v, got: %v\n", arbitraryValue, v)
		}
	} else {
		t.Fatal("Non-deterministic builtins cache lookup failed.")
	}
}

func TestEvalWithPrebuiltNDCache(t *testing.T) {
	query := "time.now_ns()"
	ndBC := builtins.NDBCache{}

	// Populate the cache for time.now_ns with an arbitrary timestamp.
	timeValue, err := time.Parse("2006-01-02T15:04:05Z", "2015-12-28T14:08:25Z")
	if err != nil {
		t.Fatal(err)
	}

	// Timestamp ns value will be: 1451311705000000000
	ndBC.Put("time.now_ns", ast.NewArray(), ast.Number(json.Number(strconv.FormatInt(timeValue.UnixNano(), 10))))
	// time.now_ns should use the cached entry instead of the current time.
	ctx := t.Context()
	rs, err := New(Query(query), NDBuiltinCache(ndBC)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Check that we got the correct time value in the result set.
	assertResultSet(t, rs, "[[1451311705000000000]]")
}

func TestNDBCacheWithRuleBody(t *testing.T) {
	ctx := t.Context()
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()

	ndBC := builtins.NDBCache{}
	query := "data.foo.p = x"
	_, err := New(
		Query(query),
		NDBuiltinCache(ndBC),
		Module("test.rego", fmt.Sprintf(`package foo
import rego.v1
p if {
	http.send({"url": "%s", "method":"get"})
}`, ts.URL)),
	).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, ok := ndBC["http.send"]
	if !ok {
		t.Fatalf("expected http.send cache entry")
	}
}

// Catches issues around iteration with ND builtins.
func TestNDBCacheWithRuleBodyAndIteration(t *testing.T) {
	ctx := t.Context()
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
	}))
	defer ts.Close()

	ndBC := builtins.NDBCache{}
	query := "data.foo.results = x"
	_, err := New(
		Query(query),
		NDBuiltinCache(ndBC),
		Module("test.rego", fmt.Sprintf(`package foo

import rego.v1

urls := [
	"%[1]s/headers",
	"%[1]s/ip",
	"%[1]s/user-agent"
]

results contains response if {
	some url in urls
	response := http.send({
		"method": "GET",
		"url": url
	})
}`, ts.URL)),
	).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that the cache exists, and has exactly 3 entries.
	entries, ok := ndBC["http.send"]
	if !ok {
		t.Fatalf("expected http.send cache entry")
	}
	if entries.Len() != 3 {
		t.Fatalf("expected 3 http.send cache entries, received:\n%v", ndBC)
	}
}

// This test ensures that the NDBCache correctly serializes/deserializes.
func TestNDBCacheMarshalUnmarshalJSON(t *testing.T) {
	original := builtins.NDBCache{}

	// Populate the cache for time.now_ns with an arbitrary timestamp.
	original.Put("time.now_ns", ast.NewArray(), ast.Number(json.Number(strconv.FormatInt(1451311705000000000, 10))))
	jOriginal, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var other builtins.NDBCache
	err = json.Unmarshal(jOriginal, &other)
	if err != nil {
		t.Fatal(err)
	}

	jOther, err := json.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the two NDBCache value's JSONified forms match exactly.
	if !bytes.Equal(jOriginal, jOther) {
		t.Fatalf("JSONified values of NDBCaches do not match; expected %s, got %s", string(jOriginal), string(jOther))
	}
}

func TestStrictBuiltinErrors(t *testing.T) {
	_, err := New(Query("1/0"), StrictBuiltinErrors(true)).Eval(t.Context())
	if err == nil {
		t.Fatal("expected error")
	}
	topdownErr, ok := err.(*topdown.Error)
	if !ok {
		t.Fatal("expected topdown error but got:", err)
	}

	if topdownErr.Code != topdown.BuiltinErr {
		t.Fatal("expected builtin error code but got:", topdownErr.Code)
	}

	if topdownErr.Message != "div: divide by zero" {
		t.Fatal("expected divide by zero error but got:", topdownErr.Message)
	}
}

func TestBuiltinErrorList(t *testing.T) {
	var buf []topdown.Error

	_, err := New(Query("1/0"), BuiltinErrorList(&buf)).Eval(t.Context())
	if err != nil {
		t.Fatal("unexpected error")
	}

	if len(buf) != 1 {
		t.Fatal("expected 1 error in buffer")
	}

	if buf[0].Error() != "1/0: eval_builtin_error: div: divide by zero" {
		t.Fatal("expected divide by zero error but got:", buf[0].Error())
	}
}

func TestTimeSeedingOptions(t *testing.T) {

	ctx := t.Context()
	clock := time.Now()

	// Check expected time is returned.
	rs, err := New(Query("time.now_ns(x)"), Time(clock)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs) != 1 || !reflect.DeepEqual(rs[0].Bindings["x"], int64ToJSONNumber(clock.UnixNano())) {
		t.Fatal("unexpected wall clock value")
	}

	// Check that time is not propagated to prepared query.
	eval, err := New(Query("time.now_ns(x)"), Time(clock)).PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	rs2, err := eval.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(rs2) != 1 || reflect.DeepEqual(rs[0].Bindings["x"], rs2[0].Bindings["x"]) {
		t.Fatal("expected new wall clock value")
	}

	// Check that prepared query returns provided time.
	rs3, err := eval.Eval(ctx, EvalTime(clock))
	if err != nil {
		t.Fatal(err)
	} else if len(rs2) != 1 || !reflect.DeepEqual(rs[0].Bindings["x"], rs3[0].Bindings["x"]) {
		t.Fatal("expected old wall clock value")
	}

}

func int64ToJSONNumber(i int64) json.Number {
	return json.Number(strconv.FormatInt(i, 10))
}

func TestPrepareAndCompileWithSchema(t *testing.T) {
	module := `
	package test
	x = input.y
	`

	schemaBytes := `{
		"$schema": "http://json-schema.org/draft-07/schema",
		"$id": "http://example.com/example.json",
		"type": "object",
		"title": "The root schema",
		"description": "The root schema comprises the entire JSON document.",
		"required": [],
		"properties": {
			"y": {
				"$id": "#/properties/y",
				"type": "integer",
				"title": "The y schema",
				"description": "An explanation about the purpose of this instance."
			}
		},
		"additionalProperties": false
	}`

	var schema any
	err := util.Unmarshal([]byte(schemaBytes), &schema)
	if err != nil {
		t.Fatal(err)
	}

	schemaSet := ast.NewSchemaSet()
	schemaSet.Put(ast.InputRootRef, schema)

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		Schemas(schemaSet),
	)

	ctx := t.Context()

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[1]]")

	// Ensure that Compile still works after Prepare
	// and its Eval has been called.
	_, err = r.Compile(ctx)
	if err != nil {
		t.Errorf("Unexpected error when compiling: %s", err.Error())
	}
}

func TestPrepareAndCompileWithRegoV1(t *testing.T) {
	module := `package test
x contains v if {
	v := input.y
}`

	r := New(
		Query("data.test.x"),
		Module("", module),
		SetRegoVersion(ast.RegoV1),
	)

	ctx := t.Context()

	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	assertPreparedEvalQueryEval(t, pq, []EvalOption{
		EvalInput(map[string]int{"y": 1}),
	}, "[[[1]]]")

	_, err = r.Compile(ctx)
	if err != nil {
		t.Errorf("Unexpected error when compiling: %s", err.Error())
	}
}

func TestGenerateJSON(t *testing.T) {
	r := New(
		Query("input"),
		Input("original-input"),
		GenerateJSON(func(*ast.Term, *EvalContext) (any, error) {
			return "converted-input", nil
		}),
	)
	assertEval(t, r, `[["converted-input"]]`)
}

func TestRegoLazyObjDefault(t *testing.T) {
	foo := map[string]any{"foo": "bar", "other": 1}
	store := inmem.NewFromObjectWithOpts(map[string]any{
		"stored": foo,
	})
	r := New(
		Query("x = data.stored"),
		Store(store),
	)

	ctx := t.Context()
	rs, err := r.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	act, ok := rs[0].Bindings["x"]
	if !ok {
		t.Fatalf("expected binding for \"x\", got %v", rs[0].Bindings)
	}
	m, ok := act.(map[string]any)
	if !ok {
		t.Fatalf("expected %T, got %T: %[2]v", m, act)
	}
	m["fox"] = true

	if _, ok := foo["fox"]; ok {
		t.Errorf("expected no change in foo, found one: %v", foo)
	}
}

func TestRegoLazyObjNoRoundTripOnWrite(t *testing.T) {
	foo := map[string]any{"foo": "bar", "other": 1}
	store := inmem.NewFromObjectWithOpts(map[string]any{
		"stored": foo,
	}, inmem.OptRoundTripOnWrite(false))
	r := New(
		Query("x = data.stored"),
		Store(store),
	)

	ctx := t.Context()
	rs, err := r.Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	act, ok := rs[0].Bindings["x"]
	if !ok {
		t.Fatalf("expected binding for \"x\", got %v", rs[0].Bindings)
	}
	m, ok := act.(map[string]any)
	if !ok {
		t.Fatalf("expected %T, got %T: %[2]v", m, act)
	}
	m["fox"] = true

	if v, ok := foo["fox"]; !ok || !v.(bool) {
		t.Errorf("expected change in foo, found none: %v", foo)
	}
}

func TestRegoLazyObjCopyMaps(t *testing.T) {
	foo := map[string]any{"foo": "bar", "other": 1}
	store := inmem.NewFromObjectWithOpts(map[string]any{
		"stored": foo,
	}, inmem.OptRoundTripOnWrite(false))
	r := New(
		Query("x = data.stored"),
		Store(store),
	)

	ctx := t.Context()
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rs, err := pq.Eval(ctx, EvalCopyMaps(true))
	if err != nil {
		t.Fatal(err)
	}
	act, ok := rs[0].Bindings["x"]
	if !ok {
		t.Fatalf("expected binding for \"x\", got %v", rs[0].Bindings)
	}
	m, ok := act.(map[string]any)
	if !ok {
		t.Fatalf("expected %T, got %T: %[2]v", m, act)
	}
	m["fox"] = true

	if _, ok := foo["fox"]; ok {
		t.Errorf("expected no change in foo, found one: %v", foo)
	}
}

func TestDescriptionRegisterBuiltin1(t *testing.T) {
	description := "custom-arity-1"

	decl := &Function{
		Name:        "foo",
		Description: description,
		Decl: types.NewFunction(
			types.Args(types.S),
			types.S,
		),
	}

	RegisterBuiltin1(decl, func(_ BuiltinContext, _ *ast.Term) (*ast.Term, error) {
		return ast.StringTerm("bar"), nil
	})
	defer unregisterBuiltin("foo")

	got := ast.Builtins[len(ast.Builtins)-1].Description
	if got != description {
		t.Fatalf("expected %q, got %q", description, got)
	}
}

func TestDescriptionRegisterBuiltin2(t *testing.T) {
	description := "custom-arity-2"

	decl := &Function{
		Name:        "foo",
		Description: description,
		Decl: types.NewFunction(
			types.Args(types.S, types.S),
			types.S,
		),
	}

	RegisterBuiltin2(decl, func(_ BuiltinContext, _, _ *ast.Term) (*ast.Term, error) {
		return ast.StringTerm("bar"), nil
	})
	defer unregisterBuiltin("foo")

	got := ast.Builtins[len(ast.Builtins)-1].Description
	if got != description {
		t.Fatalf("expected %q, got %q", description, got)
	}
}

func TestDescriptionRegisterBuiltin3(t *testing.T) {
	description := "custom-arity-3"

	decl := &Function{
		Name:        "foo",
		Description: description,
		Decl: types.NewFunction(
			types.Args(types.S, types.S, types.S),
			types.S,
		),
	}

	RegisterBuiltin3(decl, func(_ BuiltinContext, _, _, _ *ast.Term) (*ast.Term, error) {
		return ast.StringTerm("bar"), nil
	})
	defer unregisterBuiltin("foo")

	got := ast.Builtins[len(ast.Builtins)-1].Description
	if got != description {
		t.Fatalf("expected %q, got %q", description, got)
	}
}

func TestDescriptionRegisterBuiltin4(t *testing.T) {
	description := "custom-arity-4"

	decl := &Function{
		Name:        "foo",
		Description: description,
		Decl: types.NewFunction(
			types.Args(types.S, types.S, types.S, types.S),
			types.S,
		),
	}

	RegisterBuiltin4(decl, func(_ BuiltinContext, _, _, _, _ *ast.Term) (*ast.Term, error) {
		return ast.StringTerm("bar"), nil
	})
	defer unregisterBuiltin("foo")

	got := ast.Builtins[len(ast.Builtins)-1].Description
	if got != description {
		t.Fatalf("expected %q, got %q", description, got)
	}
}

func TestDescriptionRegisterBuiltinDyn(t *testing.T) {
	description := "custom-arity-dyn"

	decl := &Function{
		Name:        "foo",
		Description: description,
		Decl: types.NewFunction(
			types.Args(types.S),
			types.S,
		),
	}

	RegisterBuiltinDyn(decl, func(BuiltinContext, []*ast.Term) (*ast.Term, error) {
		return ast.StringTerm("bar"), nil
	})
	defer unregisterBuiltin("foo")

	got := ast.Builtins[len(ast.Builtins)-1].Description
	if got != description {
		t.Fatalf("expected %q, got %q", description, got)
	}
}

// unregisterBuiltin removes the builtin of the given name from ast.Builtins. This assists in
// cleaning up custom functions added as part of certain test cases.
func unregisterBuiltin(name string) {
	ast.Builtins = slices.DeleteFunc(ast.Builtins, func(b *ast.Builtin) bool { return b.Name == name })
}

func TestCompilerContextViaRegoModuleBuiltin(t *testing.T) {
	moduleSource := `package test

result := test.module("policy.rego")
`

	t.Run("compiler not passed", func(t *testing.T) {
		ctx := t.Context()
		r := New(
			Query("data.test.result"),
			CompilerHook(func(c *ast.Compiler) { ctx = ast.WithCompiler(ctx, c) }),
			Module("policy.rego", moduleSource),
			Function1(&Function{
				Name: "test.module",
				Decl: types.NewFunction(types.Args(types.S), types.S),
			}, func(bctx BuiltinContext, a *ast.Term) (*ast.Term, error) {
				moduleName, ok := a.Value.(ast.String)
				if !ok {
					return nil, fmt.Errorf("bad arg type: %T", a.Value)
				}

				comp, ok := ast.CompilerFromContext(bctx.Context)
				if !ok {
					return nil, errors.New("no compiler on context")
				}

				return ast.StringTerm(comp.Modules[string(moduleName)].String()), nil
			}),
		)

		rs, err := r.Eval(ctx)
		if err != nil {
			t.Fatalf("rego Eval error: %v", err)
		}
		if len(rs) == 0 || len(rs[0].Expressions) == 0 {
			t.Fatalf("No results")
		}
		got := rs[0].Expressions[0].Value
		want := "package test\n\nresult := __local0__ if { test.module(\"policy.rego\", __local0__) }"
		if got != want {
			t.Errorf("Expected %q, got %q", want, got)
		}
	})

	t.Run("compiler passed in", func(t *testing.T) { // when the compiler is passed, no hook is run
		ctx := t.Context()
		r := New(
			Compiler(ast.NewCompiler()),
			Query("data.test.result"),
			CompilerHook(func(*ast.Compiler) { t.Fatal("unexpected hook call") }),
			Module("policy.rego", "package test\nresult:=true"),
		)
		rs, err := r.Eval(ctx)
		if err != nil {
			t.Fatalf("rego Eval error: %v", err)
		}
		if act, exp := rs.Allowed(), true; exp != act {
			t.Errorf("expected %v, got %v", exp, act)
		}
	})
}

func TestRegoData(t *testing.T) {
	ctx := t.Context()

	r := New(
		Query("data.x.y"),
		Data(map[string]any{
			"x": map[string]any{
				"y": "hello",
			},
		}),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Fatalf("Expected one result with one expression but got: %v", rs)
	}

	if rs[0].Expressions[0].Value != "hello" {
		t.Fatalf("Expected 'hello' but got: %v", rs[0].Expressions[0].Value)
	}
}

func TestRegoDataWithModule(t *testing.T) {
	ctx := t.Context()

	mod := `
	package test
	import rego.v1

	result := data.users[input.user_id].role
	`

	r := New(
		Query("data.test.result"),
		Module("test.rego", mod),
		Data(map[string]any{
			"users": map[string]any{
				"alice": map[string]any{
					"role": "admin",
				},
				"bob": map[string]any{
					"role": "viewer",
				},
			},
		}),
		Input(map[string]any{
			"user_id": "alice",
		}),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(rs) != 1 || len(rs[0].Expressions) != 1 {
		t.Fatalf("Expected one result with one expression but got: %v", rs)
	}

	if rs[0].Expressions[0].Value != "admin" {
		t.Fatalf("Expected 'admin' but got: %v", rs[0].Expressions[0].Value)
	}
}
