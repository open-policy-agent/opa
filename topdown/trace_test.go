// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
)

func TestEventEqual(t *testing.T) {

	a := ast.NewValueMap()
	a.Put(ast.String("foo"), ast.Number("1"))
	b := ast.NewValueMap()
	b.Put(ast.String("foo"), ast.Number("2"))

	tests := []struct {
		a     *Event
		b     *Event
		equal bool
	}{
		{&Event{}, &Event{}, true},
		{&Event{Op: EvalOp}, &Event{Op: EnterOp}, false},
		{&Event{QueryID: 1}, &Event{QueryID: 2}, false},
		{&Event{ParentID: 1}, &Event{ParentID: 2}, false},
		{&Event{Node: ast.MustParseBody("true")}, &Event{Node: ast.MustParseBody("false")}, false},
		{&Event{Node: ast.MustParseBody("true")[0]}, &Event{Node: ast.MustParseBody("false")[0]}, false},
		{&Event{Node: ast.MustParseRule(`p = true { true }`)}, &Event{Node: ast.MustParseRule(`p = true { false }`)}, false},
	}

	for _, tc := range tests {
		if tc.a.Equal(tc.b) != tc.equal {
			var s string
			if tc.equal {
				s = "=="
			} else {
				s = "!="
			}
			t.Errorf("Expected %v %v %v", tc.a, s, tc.b)
		}
	}

}

func TestPrettyTrace(t *testing.T) {
	module := `package test

	p = true { q[x]; plus(x, 1, n) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `Enter data.test.p = _
| Eval data.test.p = _
| Index data.test.p = _ (matched 1 rule)
| Enter data.test.p
| | Eval data.test.q[x]
| | Index data.test.q[x] (matched 1 rule)
| | Enter data.test.q
| | | Eval x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Exit data.test.p
| Exit data.test.p = _
Redo data.test.p = _
| Redo data.test.p = _
| Redo data.test.p
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Exit data.test.p
| Redo data.test.p
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Exit data.test.p
| Redo data.test.p
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Exit data.test.p
| Redo data.test.p
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTrace(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestPrettyTraceWithLocation(t *testing.T) {
	module := `package test

	p = true { q[x]; plus(x, 1, n) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `query:1             Enter data.test.p = _
query:1             | Eval data.test.p = _
query:1             | Index data.test.p = _ (matched 1 rule)
query:3             | Enter data.test.p
query:3             | | Eval data.test.q[x]
query:3             | | Index data.test.q[x] (matched 1 rule)
query:4             | | Enter data.test.q
query:4             | | | Eval x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Exit data.test.p
query:1             | Exit data.test.p = _
query:1             Redo data.test.p = _
query:1             | Redo data.test.p = _
query:3             | Redo data.test.p
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Exit data.test.p
query:3             | Redo data.test.p
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Exit data.test.p
query:3             | Redo data.test.p
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Exit data.test.p
query:3             | Redo data.test.p
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTraceWithLocation(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestTraceNote(t *testing.T) {
	module := `package test

	p = true { q[x]; plus(x, 1, n); trace(sprintf("n= %v", [n])) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `Enter data.test.p = _
| Eval data.test.p = _
| Index data.test.p = _ (matched 1 rule)
| Enter data.test.p
| | Eval data.test.q[x]
| | Index data.test.q[x] (matched 1 rule)
| | Enter data.test.q
| | | Eval x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Eval sprintf("n= %v", [n], __local0__)
| | Eval trace(__local0__)
| | Note "n= 2"
| | Exit data.test.p
| Exit data.test.p = _
Redo data.test.p = _
| Redo data.test.p = _
| Redo data.test.p
| | Redo trace(__local0__)
| | Redo sprintf("n= %v", [n], __local0__)
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Eval sprintf("n= %v", [n], __local0__)
| | Eval trace(__local0__)
| | Note "n= 3"
| | Exit data.test.p
| Redo data.test.p
| | Redo trace(__local0__)
| | Redo sprintf("n= %v", [n], __local0__)
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Eval sprintf("n= %v", [n], __local0__)
| | Eval trace(__local0__)
| | Note "n= 4"
| | Exit data.test.p
| Redo data.test.p
| | Redo trace(__local0__)
| | Redo sprintf("n= %v", [n], __local0__)
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
| | | Exit data.test.q
| | Eval plus(x, 1, n)
| | Eval sprintf("n= %v", [n], __local0__)
| | Eval trace(__local0__)
| | Note "n= 5"
| | Exit data.test.p
| Redo data.test.p
| | Redo trace(__local0__)
| | Redo sprintf("n= %v", [n], __local0__)
| | Redo plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo data.test.q
| | | Redo x = data.a[_]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTrace(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")

	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestTraceNoteWithLocation(t *testing.T) {
	module := `package test

	p = true { q[x]; plus(x, 1, n); trace(sprintf("n= %v", [n])) }
	q[x] { x = data.a[_] }`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	tracer := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = _")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err := query.Run(ctx)
	if err != nil {
		panic(err)
	}

	expected := `query:1             Enter data.test.p = _
query:1             | Eval data.test.p = _
query:1             | Index data.test.p = _ (matched 1 rule)
query:3             | Enter data.test.p
query:3             | | Eval data.test.q[x]
query:3             | | Index data.test.q[x] (matched 1 rule)
query:4             | | Enter data.test.q
query:4             | | | Eval x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Eval sprintf("n= %v", [n], __local0__)
query:3             | | Eval trace(__local0__)
note                | | Note "n= 2"
query:3             | | Exit data.test.p
query:1             | Exit data.test.p = _
query:1             Redo data.test.p = _
query:1             | Redo data.test.p = _
query:3             | Redo data.test.p
query:3             | | Redo trace(__local0__)
query:3             | | Redo sprintf("n= %v", [n], __local0__)
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Eval sprintf("n= %v", [n], __local0__)
query:3             | | Eval trace(__local0__)
note                | | Note "n= 3"
query:3             | | Exit data.test.p
query:3             | Redo data.test.p
query:3             | | Redo trace(__local0__)
query:3             | | Redo sprintf("n= %v", [n], __local0__)
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Eval sprintf("n= %v", [n], __local0__)
query:3             | | Eval trace(__local0__)
note                | | Note "n= 4"
query:3             | | Exit data.test.p
query:3             | Redo data.test.p
query:3             | | Redo trace(__local0__)
query:3             | | Redo sprintf("n= %v", [n], __local0__)
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
query:4             | | | Exit data.test.q
query:3             | | Eval plus(x, 1, n)
query:3             | | Eval sprintf("n= %v", [n], __local0__)
query:3             | | Eval trace(__local0__)
note                | | Note "n= 5"
query:3             | | Exit data.test.p
query:3             | Redo data.test.p
query:3             | | Redo trace(__local0__)
query:3             | | Redo sprintf("n= %v", [n], __local0__)
query:3             | | Redo plus(x, 1, n)
query:3             | | Redo data.test.q[x]
query:4             | | Redo data.test.q
query:4             | | | Redo x = data.a[_]
`

	a := strings.Split(expected, "\n")
	var buf bytes.Buffer
	PrettyTraceWithLocation(&buf, *tracer)
	b := strings.Split(buf.String(), "\n")
	min := len(a)
	if min > len(b) {
		min = len(b)
	}

	for i := 0; i < min; i++ {
		if a[i] != b[i] {
			t.Errorf("Line %v in trace is incorrect. Expected %v but got: %v", i+1, a[i], b[i])
		}
	}

	if len(a) < len(b) {
		t.Fatalf("Extra lines in trace:\n%v", strings.Join(b[min:], "\n"))
	} else if len(b) < len(a) {
		t.Fatalf("Missing lines in trace:\n%v", strings.Join(a[min:], "\n"))
	}
}

func TestMultipleTracers(t *testing.T) {

	ctx := context.Background()

	buf1 := NewBufferTracer()
	buf2 := NewBufferTracer()
	q := NewQuery(ast.MustParseBody("a = 1")).
		WithTracer(buf1).
		WithTracer(buf2)

	_, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(*buf1) != len(*buf2) {
		t.Fatalf("Expected buffer lengths to be equal but got: %d and %d", len(*buf1), len(*buf2))
	}

	for i := range *buf1 {
		if !(*buf1)[i].Equal((*buf2)[i]) {
			t.Fatalf("Expected all events to be equal but at index %d got %v and %v", i, (*buf1)[i], (*buf2)[i])
		}
	}

}

func TestTraceRewrittenQueryVars(t *testing.T) {
	module := `package test

	y = [1, 2, 3]`

	ctx := context.Background()
	compiler := compileModules([]string{module})
	queryCompiler := compiler.QueryCompiler()
	data := loadSmallTestData()
	store := inmem.NewFromObject(data)
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	compiledQuery, err := queryCompiler.Compile(ast.MustParseBody("z := {a | a := data.y[_]}"))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	tracer := NewBufferTracer()
	query := NewQuery(compiledQuery).
		WithQueryCompiler(queryCompiler).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithTracer(tracer)

	_, err = query.Run(ctx)
	if err != nil {
		panic(err)
	}

	foundQueryVar := false

	for _, event := range *tracer {
		if event.LocalMetadata != nil {
			name, ok := event.LocalMetadata["__localq1__"]
			if ok && name.Name == "z" {
				foundQueryVar = true
				break
			}
		}
	}

	if !foundQueryVar {
		t.Error("Expected to find trace with rewritten var 'z' -> '__localq__")
	}

	// Rewrite the vars in the first event (which is a query) and verify that
	// that vars have been mapped to user-provided names.
	cpy := rewrite((*tracer)[0])
	node := cpy.Node.(ast.Body)
	exp := ast.MustParseBody("z = {a | a = data.y[_]}")

	if !node.Equal(exp) {
		t.Errorf("Expected %v but got %v", exp, node)
	}
}

func TestTraceRewrittenVarsIssue2022(t *testing.T) {

	input := &Event{
		Node: &ast.Expr{
			Terms: ast.VarTerm("foo"),
		},
		LocalMetadata: map[ast.Var]VarMetadata{
			ast.Var("foo"): VarMetadata{Name: ast.Var("bar")},
		},
	}

	output := rewrite(input)

	if input.Node == output.Node {
		t.Fatal("expected node to have been copied")
	} else if !output.Node.(*ast.Expr).Equal(ast.NewExpr(ast.VarTerm("bar"))) {
		t.Fatal("expected copy to contain rewritten var")
	}
}
