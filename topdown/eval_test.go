// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/metrics"
	"github.com/open-policy-agent/opa/storage"
	inmem "github.com/open-policy-agent/opa/storage/inmem/test"
)

func TestQueryIDFactory(t *testing.T) {
	f := &queryIDFactory{}
	for i := 0; i < 10; i++ {
		if n := f.Next(); n != uint64(i) {
			t.Errorf("expected %d, got %d", i, n)
		}
	}
}

func TestMergeNonOverlappingKeys(t *testing.T) {
	realData := ast.MustParseTerm(`{"foo": "bar"}`).Value.(ast.Object)
	mockData := ast.MustParseTerm(`{"baz": "blah"}`).Value.(ast.Object)

	result, ok := merge(mockData, realData)
	if !ok {
		t.Fatal("Unexpected error occurred")
	}

	expected := ast.MustParseTerm(`{"foo": "bar", "baz": "blah"}`).Value

	if expected.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}
}

func TestMergeOverlappingKeys(t *testing.T) {
	realData := ast.MustParseTerm(`{"foo": "bar"}`).Value.(ast.Object)
	mockData := ast.MustParseTerm(`{"foo": "blah"}`).Value.(ast.Object)

	result, ok := merge(mockData, realData)
	if !ok {
		t.Fatal("Unexpected error occurred")
	}

	expected := ast.MustParseTerm(`{"foo": "blah"}`).Value
	if expected.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}

	realData = ast.MustParseTerm(`{"foo": {"foo1": {"foo11": [1,2,3], "foo12": "hello"}}, "bar": "baz"}`).Value.(ast.Object)
	mockData = ast.MustParseTerm(`{"foo": {"foo1": [1,2,3], "foo12": "world", "foo13": 123}, "baz": "bar"}`).Value.(ast.Object)

	result, ok = merge(mockData, realData)
	if !ok {
		t.Fatal("Unexpected error occurred")
	}

	expected = ast.MustParseTerm(`{"foo": {"foo1": [1,2,3], "foo12": "world", "foo13": 123}, "bar": "baz", "baz": "bar"}`).Value
	if expected.Compare(result) != 0 {
		t.Fatalf("Expected %v but got %v", expected, result)
	}

}

func TestMergeWhenHittingNonObject(t *testing.T) {
	cases := []struct {
		note            string
		real, mock, exp *ast.Term
	}{
		{
			note: "real object, mock string",
			real: ast.MustParseTerm(`{"foo": "bar"}`),
			mock: ast.StringTerm("foo"),
			exp:  ast.StringTerm("foo"),
		},
		{
			note: "real string, mock object",
			real: ast.StringTerm("foo"),
			mock: ast.MustParseTerm(`{"foo": "bar"}`),
			exp:  ast.MustParseTerm(`{"foo": "bar"}`),
		},
		{
			note: "real object with string value, where mock has object-value",
			real: ast.MustParseTerm(`{"foo": ["bar"], "quz": false}`),
			mock: ast.MustParseTerm(`{"foo": {"bar": 123}}`),
			exp:  ast.MustParseTerm(`{"foo": {"bar": 123}, "quz": false}`),
		},
		{
			note: "real object with deeply-nested object value, where mock has number-value",
			real: ast.MustParseTerm(`{"foo": {"bar": {"baz": "quz"}, "quz": true}}`),
			mock: ast.MustParseTerm(`{"foo": {"bar": 10}}`),
			exp:  ast.MustParseTerm(`{"foo": {"bar": 10, "quz": true}}`),
		},
		{
			note: "real object with deeply-nested string value, where mock has object-value",
			real: ast.MustParseTerm(`{"foo": {"bar": {"baz": "quz"}, "quz": true}}`),
			mock: ast.MustParseTerm(`{"foo": {"bar": {"baz": {"foo": "bar"}}}}`),
			exp:  ast.MustParseTerm(`{"foo": {"bar": {"baz": {"foo": "bar"}}, "quz": true}}`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			merged, ok := merge(tc.mock.Value, tc.real.Value)
			if !ok {
				t.Fatal("expected no error")
			}
			if tc.exp.Value.Compare(merged) != 0 {
				t.Errorf("Expected %v but got %v", tc.exp, merged)
			}
		})
	}
}

func TestRefContainsNonScalar(t *testing.T) {
	cases := []struct {
		note     string
		ref      ast.Ref
		expected bool
	}{
		{
			note:     "empty ref",
			ref:      ast.MustParseRef("data"),
			expected: false,
		},
		{
			note:     "string ref",
			ref:      ast.MustParseRef(`data.foo["bar"]`),
			expected: false,
		},
		{
			note:     "number ref",
			ref:      ast.MustParseRef("data.foo[1]"),
			expected: false,
		},
		{
			note:     "set ref",
			ref:      ast.MustParseRef("data.foo[{0}]"),
			expected: true,
		},
		{
			note:     "array ref",
			ref:      ast.MustParseRef(`data.foo[["bar"]]`),
			expected: true,
		},
		{
			note:     "object ref",
			ref:      ast.MustParseRef(`data.foo[{"bar": 1}]`),
			expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			actual := refContainsNonScalar(tc.ref)

			if actual != tc.expected {
				t.Errorf("Expected %t for %s", tc.expected, tc.ref)
			}
		})
	}

}

func TestContainsNestedRefOrCall(t *testing.T) {

	tests := []struct {
		note  string
		input string
		want  bool
	}{
		{
			note:  "single term - negative",
			input: "p[x]",
			want:  false,
		},
		{
			note:  "single term - positive ref",
			input: "p[q[x]]",
			want:  true,
		},
		{
			note:  "single term - positive composite ref",
			input: "[q[x]]",
			want:  true,
		},
		{
			note:  "single term - positive composite call",
			input: "[f(x)]",
			want:  true,
		},
		{
			note:  "call expr - negative",
			input: "f(x)",
			want:  false,
		},
		{
			note:  "call expr - positive ref",
			input: "f(p[x])",
			want:  true,
		},
		{
			note:  "call expr - positive call",
			input: "f(g(x))",
			want:  true,
		},
		{
			note:  "call expr - positive composite",
			input: "f([g(x)])",
			want:  true,
		},
		{
			note:  "unify expr - negative",
			input: "p[x] = q[y]",
			want:  false,
		},
		{
			note:  "unify expr - positive ref",
			input: "p[x] = q[r[y]]",
			want:  true,
		},
		{
			note:  "unify expr - positive call",
			input: "f(x) = g(h(y))",
			want:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			vis := newNestedCheckVisitor()
			expr := ast.MustParseExpr(tc.input)
			result := containsNestedRefOrCall(vis, expr)
			if result != tc.want {
				t.Fatal("Expected", tc.want, "but got", result)
			}
		})
	}
}

func TestTopdownVirtualCache(t *testing.T) {
	ctx := context.Background()
	store := inmem.New()

	tests := []struct {
		note      string
		module    string
		query     string
		hit, miss uint64
		exp       interface{} // if non-nil, check var `x`
	}{
		{
			note: "different args",
			module: `package p
					f(0) = 1
					f(x) = 12 { x > 0 }`,
			query: `data.p.f(0); data.p.f(1)`,
			hit:   0,
			miss:  2,
		},
		{
			note: "same args",
			module: `package p
					f(0) = 1
					f(x) = 12 { x > 0 }`,
			query: `data.p.f(1); data.p.f(1)`,
			hit:   1,
			miss:  1,
		},
		{
			note: "captured output",
			module: `package p
					f(0) = 1
					f(x) = 12 { x > 0 }`,
			query: `data.p.f(0); data.p.f(0, x)`,
			hit:   1,
			miss:  1,
			exp:   1,
		},
		{
			note: "captured output, bool(false) result",
			module: `package p
					g(x) = true { x > 0 }
					g(x) = false { x <= 0 }`,
			query: `data.p.g(-1, x); data.p.g(-1, y)`,
			hit:   1,
			miss:  1,
			exp:   false,
		},
		{
			note: "same args, iteration case",
			module: `package p
			f(0) = 1
			f(x) = 12 { x > 0 }
			q = y {
				x := f(1)
				y := f(1)
				x == y
			}`,
			query: `x = data.p.q`,
			hit:   1,
			miss:  2, // one for q, one for f(1)
			exp:   12,
		},
		{
			note: "cache invalidation",
			module: `package p
					f(x) = y { x+input = y }`,
			query: `data.p.f(1, z) with input as 7; data.p.f(1, z2) with input as 8`,
			hit:   0,
			miss:  2,
		},
		{
			note: "partial object: simple",
			module: `package p
			s["foo"] = true { true }
			s["bar"] = true { true }`,
			query: `data.p.s["foo"]; data.p.s["foo"]`,
			hit:   1,
			miss:  1,
		},
		{
			note: "partial set: simple",
			module: `package p
			s["foo"] { true }
			s["bar"] { true }`,
			query: `data.p.s["foo"]; data.p.s["foo"]`,
			hit:   1,
			miss:  1,
		},
		{
			note: "partial set: object",
			module: `package p
				s[z] { z := {"foo": "bar"} }`,
			query: `x = {"foo": "bar"}; data.p.s[x]; data.p.s[x]`,
			hit:   1,
			miss:  1,
		},
		{
			note: "partial set: miss",
			module: `package p
				s[z] { z = true }`,
			query: `data.p.s[true]; not data.p.s[false]`,
			hit:   0,
			miss:  2,
		},
		{
			note: "partial set: full extent cached",
			module: `package test
				p[x] { x = 1 }
				p[x] { x = 2 }
			`,
			query: "data.test.p = x; data.test.p = y",
			hit:   1,
			miss:  1,
		},
		{
			note: "partial set: all rules + each rule (non-ground var) cached",
			module: `package test
				p = r { data.test.q = x; data.test.q[y] = z; data.test.q[a] = b; r := true }
				q[x] { x = 1 }
				q[x] { x = 2 }
			`,
			query: "data.test.p = true",
			hit:   3, // 'data.test.q[y] = z' + 2x 'data.test.q[a] = b'
			miss:  2, // 'data.test.p = true' + 'data.test.q = x'
		},
		{
			note: "partial set: all rules + each rule (non-ground composite) cached",
			module: `package test
				p { data.test.q = x; data.test.q[[y, 1]] = z; data.test.q[[a, 2]] = b }
				q[[x, x]] { x = 1 }
				q[[x, x]] { x = 2 }
			`,
			query: "data.test.p = true",
			hit:   2, // 'data.test.q[[y,1]] = z' + 'data.test.q[[a, 2]] = b'
			miss:  2, // 'data.test.p = true' + 'data.test.q = x'
		},
		{
			note: "partial set: each rule (non-ground var), full extent cached",
			module: `package test
				p = r { data.test.q[y] = z; data.test.q = x; r := true }
				q[x] { x = 1 }
				q[x] { x = 2 }
			`,
			query: "data.test.p = x",
			hit:   2, // 2x 'data.test.q = x'
			miss:  2, // 'data.test.p = true' + 'data.test.q[y] = z'
		},
		{
			note: "partial set: each rule (non-ground composite), full extent cached",
			module: `package test
				p = y { data.test.q[[y, 1]] = z; data.test.q = x }
				q[[x, x]] { x = 1 }
				q[[x, x]] { x = 2 }
			`,
			query: "data.test.p = x",
			hit:   0,
			miss:  3, // 'data.test.p = true' + 'data.test.q[[y, 1]] = z' + 'data.test.q = x'
			exp:   1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := compileModules([]string{tc.module})
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)
			m := metrics.New()

			query := NewQuery(ast.MustParseBody(tc.query)).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				WithInstrumentation(NewInstrumentation(m))
			qrs, err := query.Run(ctx)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if exp, act := 1, len(qrs); exp != act {
				t.Fatalf("expected %d query result, got %d query results: %+v", exp, act, qrs)
			}
			if tc.exp != nil {
				x := ast.Var("x")
				if exp, act := ast.NewTerm(ast.MustInterfaceToValue(tc.exp)), qrs[0][x]; !exp.Equal(act) {
					t.Errorf("unexpected query result: want = %v, got = %v", exp, act)
				}
			}

			// check metrics
			if exp, act := tc.hit, m.Counter(evalOpVirtualCacheHit).Value().(uint64); exp != act {
				t.Errorf("expected %d cache hits, got %d", exp, act)
			}
			if exp, act := tc.miss, m.Counter(evalOpVirtualCacheMiss).Value().(uint64); exp != act {
				t.Errorf("expected %d cache misses, got %d", exp, act)
			}
		})
	}
}
