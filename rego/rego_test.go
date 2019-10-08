// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"context"
	"encoding/json"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/storage/mock"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
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
	} else if topdownErr, ok := err.(*topdown.Error); !ok || topdownErr.Code != topdown.CancelErr {
		t.Fatalf("Got unexpected error: %v", err)
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
		if _, ok := all[name]; !ok {
			t.Errorf("expected to find %v but did not", name)
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
		EvalTracer(tracer),
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
	expectedQuery := "input.x = 1"
	if len(partialQuery.Queries) != 1 {
		t.Errorf("expected 1 query but found %d: %+v", len(partialQuery.Queries), pq)
	}
	if partialQuery.Queries[0].String() != expectedQuery {
		t.Errorf("unexpected query in result, expected='%s' found='%s'",
			expectedQuery, partialQuery.Queries[0].String())
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

	t.Run("unsafe query", func(t *testing.T) {
		r := New(
			Query(`count([1, 2, 3])`),
			UnsafeBuiltins(map[string]struct{}{"count": struct{}{}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExpr) {
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
			UnsafeBuiltins(map[string]struct{}{"count": struct{}{}}),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExpr) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("inherit in query", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": struct{}{}})),
			Query("count([])"),
		)
		if _, err := r.Eval(ctx); err == nil || !strings.Contains(err.Error(), unsafeCountExpr) {
			t.Fatalf("Expected unsafe built-in error but got %v", err)
		}
	})

	t.Run("override/disable in query", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": struct{}{}})),
			UnsafeBuiltins(map[string]struct{}{}),
			Query("count([])"),
		)
		if _, err := r.Eval(ctx); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("override/change in query", func(t *testing.T) {
		r := New(
			Compiler(ast.NewCompiler().WithUnsafeBuiltins(map[string]struct{}{"count": struct{}{}})),
			UnsafeBuiltins(map[string]struct{}{"max": struct{}{}}),
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
			UnsafeBuiltins(map[string]struct{}{"count": struct{}{}}),
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

	var regoArgs []func(r *Rego)

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
	})
}

func TestRegoEvalPoliciesinStore(t *testing.T) {
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
