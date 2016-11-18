// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
)

func TestEventEqual(t *testing.T) {

	a := ast.NewValueMap()
	a.Put(ast.String("foo"), ast.Number(1))
	b := ast.NewValueMap()
	b.Put(ast.String("foo"), ast.Number(2))

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
		{&Event{Node: ast.MustParseRule("p :- true")}, &Event{Node: ast.MustParseRule("p :- false")}, false},
		{&Event{Node: "foo"}, &Event{Node: "foo"}, false}, // test some unsupported node type
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
	module := `
	package test
	p :- q[x], plus(x, 1, n)
	q[x] :- x = data.a[_]
	`

	compiler := compileModules([]string{module})
	data := loadSmallTestData()
	store := storage.New(storage.InMemoryWithJSONConfig(data))
	txn := storage.NewTransactionOrDie(store)

	params := NewQueryParams(compiler, store, txn, nil, ast.MustParseRef("data.test.p"))
	tracer := NewBufferTracer()
	params.Tracer = tracer

	_, err := Query(params)
	if err != nil {
		panic(err)
	}

	expected := `Enter eq(data.test.p, _)
| Eval eq(data.test.p, _)
| Enter p = true :- data.test.q[x], plus(x, 1, n)
| | Eval data.test.q[x]
| | Enter q[x] :- eq(x, data.a[_])
| | | Eval eq(x, data.a[_])
| | | Exit q[x] :- eq(x, data.a[_])
| | Eval plus(x, 1, n)
| | Exit p = true :- data.test.q[x], plus(x, 1, n)
| Redo p = true :- data.test.q[x], plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo q[x] :- eq(x, data.a[_])
| | | Redo eq(x, data.a[_])
| | | Exit q[x] :- eq(x, data.a[_])
| | Eval plus(x, 1, n)
| | Exit p = true :- data.test.q[x], plus(x, 1, n)
| Redo p = true :- data.test.q[x], plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo q[x] :- eq(x, data.a[_])
| | | Redo eq(x, data.a[_])
| | | Exit q[x] :- eq(x, data.a[_])
| | Eval plus(x, 1, n)
| | Exit p = true :- data.test.q[x], plus(x, 1, n)
| Redo p = true :- data.test.q[x], plus(x, 1, n)
| | Redo data.test.q[x]
| | Redo q[x] :- eq(x, data.a[_])
| | | Redo eq(x, data.a[_])
| | | Exit q[x] :- eq(x, data.a[_])
| | Eval plus(x, 1, n)
| | Exit p = true :- data.test.q[x], plus(x, 1, n)
| Exit eq(data.test.p, _)
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
