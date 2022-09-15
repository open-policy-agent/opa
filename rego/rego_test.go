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
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/ast/location"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/topdown/builtins"
	"github.com/open-policy-agent/opa/topdown/cache"
	"github.com/open-policy-agent/opa/types"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func assertEval(t *testing.T, r *Rego, expected string) {
	t.Helper()
	rs, err := r.Eval(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
	assertResultSet(t, rs, expected)
}

func assertPreparedEvalQueryEval(t *testing.T, pq PreparedEvalQuery, options []EvalOption, expected string) {
	t.Helper()
	rs, err := pq.Eval(context.Background(), options...)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}
	assertResultSet(t, rs, expected)
}

func assertResultSet(t *testing.T, rs ResultSet, expected string) {
	t.Helper()
	result := []interface{}{}

	for i := range rs {
		values := []interface{}{}
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
		input    interface{}
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
			func() interface{} {
				a := &struct {
					Foo string `json:"baz"`
				}{"bar"}
				return &a
			}(), `[[{"baz":"bar"}]]`},
		"slice":              {[]string{"a", "b"}, `[[["a", "b"]]]`},
		"nil":                {nil, `[[null]]`},
		"slice of interface": {[]interface{}{"a", 2, true}, `[[["a", 2, true]]]`},
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

	ctx := context.Background()

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

	ctx := context.Background()

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
			types.NewNull(),
		),
	})

	topdown.RegisterFunctionalBuiltin1("test.sleep", func(a ast.Value) (ast.Value, error) {
		d, _ := time.ParseDuration(string(a.(ast.String)))
		time.Sleep(d)
		return ast.Null{}, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	r := New(Query(`test.sleep("1s")`))
	rs, err := r.Eval(ctx)
	cancel()

	if err == nil {
		t.Fatalf("Expected cancellation error but got: %v", rs)
	}
	exp := topdown.Error{Code: topdown.CancelErr, Message: "caller cancelled query execution"}
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
				types.NewNull(),
			),
		},
		func(BuiltinContext, *ast.Term) (*ast.Term, error) {
			return nil, NewHaltError(fmt.Errorf("stop"))
		},
	)
	r := New(Query(`halt_func("")`), funOpt)
	rs, err := r.Eval(context.Background())
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
	ctx := context.Background()
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
		Input(map[string]interface{}{"x": 10})).PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	_, err = pq.Eval(context.Background()) // no EvalTracer option
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
		Input(map[string]interface{}{"x": 10})).PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	_, err = pq.Eval(context.Background()) // no EvalQueryTracer option
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

	p {
		input.x = 1
	}

	p {
		input.y = 1
	}
	`
	pq, err := New(
		Query("data"),
		Module("foo.rego", mod),
	).PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	_, err = pq.Eval(
		context.Background(),
		EvalQueryTracer(tracer),
		EvalRuleIndexing(false),
		EvalInput(map[string]interface{}{"x": 10}),
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
		found := false
		for _, actual := range evalNodes {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("Missing expected eval node in trace: %q\nGot: %q\n", expected, evalNodes)
		}
	}
}

func TestRegoCatchPathConflicts(t *testing.T) {
	r := New(
		Query("data"),
		Module("test.rego", "package x\np=1"),
		Store(inmem.NewFromObject(map[string]interface{}{
			"x": map[string]interface{}{"p": 1},
		})),
	)

	ctx := context.Background()
	_, err := r.Eval(ctx)

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPartialRewriteEquals(t *testing.T) {
	mod := `
	package test
	default p = false
	p {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	ctx := context.Background()
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
			p[{"x":"y"}]`,
			exp: `[[[{"x":"y"}]]]`,
		},
		{
			note: "set",
			module: `package test
			p[{"x"}]`,
			exp: `[[[["x"]]]]`,
		},
		{
			note: "array",
			module: `package test
			p[["x"]]`,
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

			pq, err := r.PrepareForEval(context.Background())
			if err != nil {
				t.Fatalf("Unexpected error: %s", err.Error())
			}

			// run this 1000 times concurrently
			var wg sync.WaitGroup
			wg.Add(1000)
			for i := 0; i < 1000; i++ {
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

	pq, err := r.PrepareForEval(context.Background())
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

	pq, err := r.PrepareForEval(context.Background())
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
	ctx := context.Background()
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

	err = store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{"y": 1})
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
	err = store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{"y": 2})
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
	err = store.Write(ctx, txn, storage.AddOp, path, map[string]interface{}{"y": 3})
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

	pq, err := r.PrepareForEval(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err.Error())
	}

	// Expect evaluating the same thing >1 time gives the same
	// results each time.
	for i := 0; i < 5; i++ {
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

	pq, err := r.PrepareForEval(context.Background())
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

func TestPrepareAndEvalNewPrintHook(t *testing.T) {
	module := `
	package test
	x { print(input) }
	`

	r := New(
		Query("data.test.x"),
		Module("", module),
		Package("foo"),
		EnablePrintStatements(true),
	)

	pq, err := r.PrepareForEval(context.Background())
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

	ctx := context.Background()

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

	ctx := context.Background()

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
	default p = false
	p {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	ctx := context.Background()

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

func TestPartialNamespace(t *testing.T) {

	r := New(
		PartialNamespace("foo"),
		Query("data.test.p = x"),
		Module("test.rego", `
			package test

			default p = false

			p { input.x = 1 }
		`),
	)

	pq, err := r.Partial(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	expQuery := ast.MustParseBody(`data.foo.test.p = x`)

	if len(pq.Queries) != 1 || !pq.Queries[0].Equal(expQuery) {
		t.Fatalf("Expected exactly one query %v but got: %v", expQuery, pq.Queries)
	}

	expSupport := ast.MustParseModule(`
		package foo.test

		default p = false

		p { input.x = 1 }
	`)

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

	ctx := context.Background()

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
	default p = false
	p {
		input.x == 1
	}
	`
	r := New(
		Query("data.test.p"),
		Module("test.rego", mod),
	)

	ctx := context.Background()
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
	p {
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

	ctx := context.Background()
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
	default p = false
	p {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	tracer := topdown.NewBufferTracer()

	ctx := context.Background()
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
	default p = false
	p {
		input.x = 1
	}
	`
	r := New(
		Query("data.test.p == true"),
		Module("test.rego", mod),
	)

	tracer := topdown.NewBufferTracer()

	ctx := context.Background()
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
	p {
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

	ctx := context.Background()
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
	rs, err := r.Eval(context.Background())

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

	res, err := r.Eval(context.Background())

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

	rs, err := r.Eval(context.Background())
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

	ctx := context.Background()

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
			deny {
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
			deny {
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
			deny {
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

	t.Run("ignore if given compiler", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler()),
			UnsafeBuiltins(map[string]struct{}{"count": {}}),
			Query("data.test.p = 0"),
			Module("test.rego", `package test

			p = count([])`),
		)
		rs, err := r.Eval(context.Background())
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

	ctx := context.Background()
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
		ctx := context.Background()

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
		ctx := context.Background()

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
			if exp := filepath.Join(path, "x/x.rego"); exp != act {
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
		ctx := context.Background()
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
			if exp := filepath.Join(path, "x/x.rego"); exp != act {
				t.Errorf("expected module name %q, got %q", exp, act)
			}
		}
	})
}

func TestRegoEvalPoliciesInStore(t *testing.T) {
	store := mock.New()
	ctx := context.Background()
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

	ctx := context.Background()

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

func TestRegoLoadFilesWithProvidedStore(t *testing.T) {
	ctx := context.Background()
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
	ctx := context.Background()
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
	p {
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

	pr, err := originalRego.PartialResult(context.Background())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	rs, err := pr.Rego(
		Input(map[string]interface{}{"foo": "/foo/bar/baz/"}),
	).Eval(context.Background())

	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	assertResultSet(t, rs, `[[true]]`)

}

func TestRegoPartialResultRecursiveRefs(t *testing.T) {

	r := New(Query("data"), Module("test.rego", `package foo.bar

	default p = false

	p { input.x = 1 }`))

	_, err := r.PartialResult(context.Background())
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

		default p = false

		p = true { input }
	`), SkipPartialNamespace(true))

	pq, err := r.Partial(context.Background())
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
	r := New(Query("data.test.p = true"), Module("example.rego", `
		package test

		p {
			q = true
		}

		q {
			input.x = r
		}

		r = 7
	`), ShallowInlining(true))

	pq, err := r.Partial(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(pq.Queries) != 1 || !pq.Queries[0].Equal(ast.MustParseBody("data.partial.test.p = true")) {
		t.Fatal("expected exactly one query and ref to be rewritten but got:", pq.Queries)
	}

	exp := ast.MustParseModule(`
		package partial.test

		p { data.partial.test.q = true }
		q { 7 = input.x }
	`)

	if len(pq.Support) != 1 || !pq.Support[0].Equal(exp) {
		t.Fatal("expected module:", exp, "\n\ngot module:", pq.Support[0])
	}
}

func TestRegoPartialResultSortedRules(t *testing.T) {
	r := New(Query("data.test.p"), Module("example.rego", `
		package test

		default p = false

		p {
			r = (input.d * input.a) + input.c
			r < s
		}

		p {
			r = (input.d * input.b) + input.c
			r < s
		}

		s = 100

	`))

	pq, err := r.Partial(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Without sorting of support rules, the output of the above partial evaluation
	// resulted in a random order of the support rules (in this case two different possible outputs)
	exp := ast.MustParseModule(
		`package partial.test

		default p = false

		p = true { lt(plus(mul(input.d, input.a), input.c), 100) }
		p = true { lt(plus(mul(input.d, input.b), input.c), 100) }
		`,
	)

	if len(pq.Support) != 1 || !pq.Support[0].Equal(exp) {
		t.Fatal("expected module:", exp, "\n\ngot module:", pq.Support[0])
	}

}

func TestPrepareWithEmptyModule(t *testing.T) {
	_, err := New(
		Query("d"),
		Module("example.rego", ""),
	).PrepareForEval(context.Background())

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
		ctx := context.Background()

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
	ctx := context.Background()
	_, err := New(Query(query), NDBuiltinCache(ndBC)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Check and make sure we got exactly 2x items back in the ND builtin cache.
	// NDBuiltinsCache always has the structure: map[ast.String]map[ast.Array]ast.Value
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
	ctx := context.Background()
	rs, err := New(Query(query), NDBuiltinCache(ndBC)).Eval(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Check that we got the correct time value in the result set.
	assertResultSet(t, rs, "[[1451311705000000000]]")
}

func TestNDBCacheWithRuleBody(t *testing.T) {
	ctx := context.Background()
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer ts.Close()

	ndBC := builtins.NDBCache{}
	query := "data.foo.p = x"
	_, err := New(
		Query(query),
		NDBuiltinCache(ndBC),
		Module("test.rego", fmt.Sprintf(`package foo
p {
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
	ctx := context.Background()
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
	}))
	defer ts.Close()

	ndBC := builtins.NDBCache{}
	query := "data.foo.results = x"
	_, err := New(
		Query(query),
		NDBuiltinCache(ndBC),
		Module("test.rego", fmt.Sprintf(`package foo

import future.keywords

urls := [
	"%[1]s/headers",
	"%[1]s/ip",
	"%[1]s/user-agent"
]

results[response] {
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
	_, err := New(Query("1/0"), StrictBuiltinErrors(true)).Eval(context.Background())
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

func TestTimeSeedingOptions(t *testing.T) {

	ctx := context.Background()
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

	var schema interface{}
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

	ctx := context.Background()

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

func TestGenerateJSON(t *testing.T) {
	r := New(
		Query("input"),
		Input("original-input"),
		GenerateJSON(func(t *ast.Term, ectx *EvalContext) (interface{}, error) {
			return "converted-input", nil
		}),
	)
	assertEval(t, r, `[["converted-input"]]`)
}
