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
