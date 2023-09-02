// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"encoding/json"
	"strings"
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
	// TODO: break out into separate tests
	t.Setenv("EXPERIMENTAL_GENERAL_RULE_REFS", "true")

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
			note: "partial object: query into object value",
			module: `package p
			s["foo"] = { "x": 42, "y": 43 } { true }
			s["bar"] = { "x": 42, "y": 43 } { true }`,
			query: `data.p.s["foo"].x = x; data.p.s["foo"].y`,
			hit:   1,
			miss:  1,
			exp:   42,
		},
		{
			note: "partial object: simple, general ref",
			module: `package p
			s.t[u].v = true { x = ["foo", "bar"]; u = x[_] }`,
			query: `data.p.s.t["foo"].v = x; data.p.s.t["foo"].v`,
			hit:   1,
			miss:  1,
			exp:   true,
		},
		{
			note: "partial object: simple, general ref, multiple vars",
			module: `package p
			s.t[u].v[w] = true { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[_] }`,
			query: `data.p.s.t = x; data.p.s.t`,
			hit:   1,
			miss:  1,
			exp: map[string]interface{}{
				"foo": map[string]interface{}{
					"v": map[string]interface{}{
						"do": true,
						"re": true,
					},
				},
				"bar": map[string]interface{}{
					"v": map[string]interface{}{
						"do": true,
						"re": true,
					},
				},
			},
		},
		{
			note: "partial object: simple, general ref, multiple vars (2)",
			module: `package p
			s.t[u].v[w] = true { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[_] }`,
			query: `data.p.s.t.foo = x; data.p.s.t["foo"]`,
			hit:   1,
			miss:  1,
			exp: map[string]interface{}{
				"v": map[string]interface{}{
					"do": true,
					"re": true,
				},
			},
		},
		{
			note: "partial object: simple, general ref, multiple vars (3)",
			module: `package p
			s.t[u].v[w] = true { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[_] }`,
			query: `data.p.s.t.foo.v = x; data.p.s.t["foo"].v`,
			hit:   1,
			miss:  1,
			exp: map[string]interface{}{
				"do": true,
				"re": true,
			},
		},
		{
			note: "partial object: simple, general ref, multiple vars (4)",
			module: `package p
			s.t[u].v[w] = true { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[_] }`,
			query: `data.p.s.t.foo.v.re = x; data.p.s.t["foo"].v["re"]`,
			hit:   1,
			miss:  1,
			exp:   true,
		},
		{
			note: "partial object: simple, general ref, miss",
			module: `package p
			s.t[u].v[w] = true { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[_] }`,
			query: `data.p.s.t.foo.v.re = x; data.p.s.t.foo.v.do`,
			hit:   0,
			miss:  2,
			exp:   true,
		},
		{
			note: "partial object: simple, general ref, miss (2)",
			module: `package p
			s.t[u].v[w] = i { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[i] }`,
			query: `data.p.s.t.foo.v.re = x; data.p.s.t.foo.v.do; data.p.s.t.foo.v.re`,
			hit:   1,
			miss:  2,
			exp:   1,
		},
		{
			note: "partial object: simple, general ref, miss (3)",
			module: `package p
			s.t[u].v[w] = i { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[i] }`,
			query: `data.p.s.t.foo.v.re = x; data.p.s.t.foo.v.do; data.p.s.t.bar.v.re`,
			hit:   0,
			miss:  3,
			exp:   1,
		},
		{
			note: "partial object: simple, general ref, miss (3)",
			module: `package p
			s.t[u].v[w] = i { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[i] }`,
			query: `data.p.s.t.foo.v.re = x; data.p.s.t.foo.v.do; data.p.s.t.bar.v.re; data.p.s.t.foo.v.do`,
			hit:   1,
			miss:  3,
			exp:   1,
		},
		{
			note: "partial object: simple, general ref, miss (4)",
			module: `package p
			s.t[u].v[w] = i { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[i] }`,
			query: `data.p.s.t.foo = x; data.p.s.t.foo.v.do`,
			hit:   1,
			miss:  1,
			exp: map[string]interface{}{
				"v": map[string]interface{}{
					"do": 0,
					"re": 1,
				},
			},
		},
		{
			note: "partial object: simple, general ref, miss (5)",
			module: `package p
			s.t[u].v[w] = i { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[i] }`,
			query: `data.p.s.t.foo; data.p.s.t.foo.v.do = x`,
			hit:   1,
			miss:  1,
			exp:   0,
		},
		{
			note: "partial object: simple, general ref, miss (6)",
			module: `package p
			s.t[u].v[w] = i { x = ["foo", "bar"]; u = x[_]; y = ["do", "re"]; w = y[i] }`,
			query: `data.p.s.t.foo.v.do = x; data.p.s.t.foo`,
			hit:   0, // Note: Could we be smart in query term eval order to gain an extra hit here?
			miss:  2,
			exp:   0,
		},
		{
			note: "partial object: simple, query into value",
			module: `package p
			s["foo"].t = { "x": 42, "y": 43 } { true }
			s["bar"].t = { "x": 42, "y": 43 } { true }`,
			query: `data.p.s["foo"].t.x = x; data.p.s["foo"].t.x`,
			hit:   1,
			miss:  1,
			exp:   42,
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

func TestPartialRule(t *testing.T) {
	t.Setenv("EXPERIMENTAL_GENERAL_RULE_REFS", "true")

	ctx := context.Background()
	store := inmem.New()

	tests := []struct {
		note   string
		module string
		query  string
		exp    string
		expErr string
	}{
		{
			note: "partial set",
			module: `package test
				p[v] {
					v := [1, 2, 3][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": [1, 2, 3]}}}]`,
		},
		{
			note: "partial object",
			module: `package test
				p[i] := v {
					v := [1, 2, 3][i]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"0": 1, "1": 2, "2": 3}}}}]`,
		},
		{
			note: "partial object (const key)",
			module: `package test
				p["foo"] := v {
					v := 42
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": 42}}}}]`,
		},
		{
			note: "ref head",
			module: `package test
				p.foo := v {
					v := 42
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": 42}}}}]`,
		},
		{
			note: "partial object (ref head)",
			module: `package test
				p.q.r[i] := v {
					v := ["a", "b", "c"][i]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"q": {"r": {"0": "a", "1": "b", "2": "c"}}}}}}]`,
		},
		{
			note: "partial object (ref head), query to obj root",
			module: `package test
				p.q.r[i] := v {
					v := ["a", "b", "c"][i]
				}
			`,
			query: `data.test.p.q.r = x`,
			exp:   `[{"x": {"0": "a", "1": "b", "2": "c"}}]`,
		},
		{
			note: "partial object (ref head), query to obj root, enumerating keys",
			module: `package test
				p.q.r[i] := v {
					v := ["a", "b", "c"][i]
				}
			`,
			query: `data.test.p.q.r[x]`,
			// NOTE: $_term_0_0 wildcard var is filtered from eval result output
			exp: `[{"x": 0, "$_term_0_0": "a"}, {"x": 1, "$_term_0_0": "b"}, {"x": 2, "$_term_0_0": "c"}]`,
		},
		{
			note: "partial object (ref head), implicit 'true' value",
			module: `package test
				p.q.r[v] {
					v := [1, 2, 3][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"q": {"r": {"1": true, "2": true, "3": true}}}}}}]`,
		},
		{
			note: "partial set (ref head)",
			module: `package test
				import future.keywords
				p.q contains v if {
					v := [1, 2, 3][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"q": [1, 2, 3]}}}}]`,
		},
		{
			note: "partial set (general ref head)",
			module: `package test
				import future.keywords
				p[j] contains v if {
					v := [1, 2, 3][_]
					j := ["a", "b", "c"][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"a": [1, 2, 3], "b": [1, 2, 3], "c": [1, 2, 3]}}}}]`,
		},
		{
			note: "partial set (general ref head, static suffix)",
			module: `package test
				import future.keywords
				p[q].r contains v if {
					q := "foo"
					v := [1, 2, 3][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": {"r": [1, 2, 3]}}}}}]`,
		},
		{
			note: "partial object (general ref head, multiple vars)",
			module: `package test
				p.q[x].r[i] := v {
					some i
					v := [1, 2, 3][i]
					x := ["a", "b", "c"][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"q": {"a": {"r": {"0": 1, "1": 2, "2": 3}}, "b": {"r": {"0": 1, "1": 2, "2": 3}}, "c": {"r": {"0": 1, "1": 2, "2": 3}}}}}}}]`,
		},
		{
			note: "partial object (general ref head, multiple vars) #2",
			module: `package test
				p[j].foo[i] := v {
					v := [1, 2, 3][i]
					j := ["a", "b", "c"][_]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"a": {"foo": {"0": 1, "1": 2, "2": 3}}, "b": {"foo": {"0": 1, "1": 2, "2": 3}}, "c": {"foo": {"0": 1, "1": 2, "2": 3}}}}}}]`,
		},
		{
			note: "partial set (multiple vars in general ref head)",
			module: `package test
				import future.keywords
				p[j][i] contains v if {
					v := [1, 2, 3][_]
					j := ["a", "b", "c"][_]
					i := "foo"
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"a": {"foo": [1, 2, 3]}, "b": {"foo": [1, 2, 3]}, "c": {"foo": [1, 2, 3]}}}}}]`,
		},
		// Overlapping rules
		{
			note: "partial object with overlapping rule (defining key/value in object)",
			module: `package test
				foo.bar[i] := v {
					v := ["a", "b", "c"][i]
				}
				foo.bar.baz := 42
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"foo": {"bar": {"0": "a", "1": "b", "2": "c", "baz": 42}}}}}]`,
		},
		{
			note: "partial object with overlapping rule (dee ref on overlap)",
			module: `package test
				p[k] := 1 {
					k := "foo"
				}
				p.q.r.s.t := 42
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": 1, "q": {"r": {"s": {"t": 42}}}}}}}]`,
		},
		{
			note: "partial object with overlapping rule (dee ref on overlap; conflict)",
			module: `package test
				p[k] := 1 {
					k := "q"
				}
				p.q.r.s.t := 42
			`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object with overlapping rule (key conflict)",
			module: `package test
				foo.bar[k] := v {
					k := "a"
					v := 43
				}
				foo.bar["a"] := 42
			`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object generating conflicting nested keys (different nested object depth)",
			module: `package test
				p.q.r {
					true
				}
				p.q[r].s.t {
					r := "foo"
				}`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"q": {"foo": {"s": {"t": true}}, "r": true}}}}}]`,
		},
		{
			note: "partial object generating conflicting nested keys (different nested object depth; key conflict)",
			module: `package test
				p.q[k].s := 1 {
					k := "r"
				}
				p.q[k].s.t := 1 {
					k := "r"
				}`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object (overlapping rules producing same values)",
			module: `package test
				p.foo.bar[i] := v {
					v := ["a", "b", "c"][i]
				}
				p.foo[i][j] := v {
					i := "bar"
					v := ["a", "b", "c"][j]
				}
				p[q][i][j] := v {
					q := "foo"
					i := "bar"
					v := ["a", "b", "c"][j]
				}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": {"bar": {"0": "a", "1": "b", "2": "c"}}}}}}]`,
		},
		{
			note: "partial object (overlapping rules, same depth, producing non-conflicting keys)",
			module: `package test
				p.foo[i].bar := v {
					v := ["a", "b", "c"][i]
				}
				p.foo.bar[i] := v {
					v := ["a", "b", "c"][i]
				}
			`,
			query: `data = x`,
			exp: `[{"x": {"test": {"p": {"foo": {
						"0": {"bar": "a"}, 
						"1": {"bar": "b"}, 
						"2": {"bar": "c"}, 
						"bar": {"0": "a", "1": "b", "2": "c"}}}}}}]`,
		},
		// Intersections with object values
		{
			note: "partial object NOT intersecting with object value of other rule",
			module: `package test
				p.foo := {"bar": {"baz": 1}}
				p[k] := 2 {k := "other"}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": {"bar": {"baz": 1}}, "other": 2}}}}]`,
		},
		{
			note: "partial object NOT intersecting with object value of other rule (nested object merge along rule refs)",
			module: `package test
				p.foo.bar := {"baz": 1}                        # p.foo.bar == {"baz": 1}
				p[k].bar2 := v {k := "foo"; v := {"other": 2}} # p.foo.bar2 == {"other": 2}
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"foo": {"bar": {"baz": 1}, "bar2": {"other": 2}}}}}}]`,
		},
		{
			note: "partial object intersecting with object value of other rule (not merging otherwise conflict-free obj values)",
			module: `package test
				p.foo := {"bar": {"baz": 1}}                       # p == {"foo": {"bar": {"baz": 1}}}
				p[k] := v {k := "foo"; v := {"bar": {"other": 2}}} # p == {"foo": {"bar": {"other": 2}}}
			`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique", // conflict on key "bar" which is inside rule values, which may not be modified by other rule
		},
		{
			note: "partial object rules with overlapping known ref vars (no eval-time conflict)",
			module: `package test
				p[k].r1 := 1 { k := "q" }
				p[k].r2 := 2 { k := "q" }
			`,
			query: `data = x`,
			exp:   `[{"x": {"test": {"p": {"q": {"r1": 1, "r2": 2}}}}}]`,
		},
		{
			note: "partial object rules with overlapping known ref vars (eval-time conflict)",
			module: `package test
				p[k].r := 1 { k := "q" }
				p[k].r := 2 { k := "q" }
			`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object rules with overlapping known ref vars, non-overlapping object type values (eval-time conflict)",
			module: `package test
				p[k].r := {"s1": 1} { k := "q" }
				p[k].r := {"s2": 2} { k := "q" }
			`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		// Deep queries
		{
			note: "deep query into partial object (ref head)",
			module: `package test
				p.q[r] := 1 { r := "foo" }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": 1}]`,
		},
		{
			note: "deep query into partial object (ref head) and object value",
			module: `package test
				p.q[r] := x { 
					r := "foo" 
					x := {"bar": {"baz": 1}}
				}
			`,
			query: `data.test.p.q.foo.bar = x`,
			exp:   `[{"x": {"baz": 1}}]`,
		},
		{
			note: "deep query into partial object starting-point (general ref head) up to array value",
			module: `package test
				p.q[r].s[t].u := x {
					obj := {
						"foo": {
							"do": ["a", "b", "c"],
							"re": ["d", "e", "f"],
						},
						"bar": {
							"mi": ["g", "h", "i"],
							"fa": ["j", "k", "l"],
						}
					}
					x := obj[r][t]
				}
			`,
			query: `data.test.p.q = x`,
			exp:   `[{"x": {"bar": {"s": {"fa": {"u": ["j", "k", "l"]}, "mi": {"u": ["g", "h", "i"]}}}, "foo": {"s": {"do": {"u": ["a", "b", "c"]}, "re": {"u": ["d", "e", "f"]}}}}}]`,
		},
		{
			note: "deep query into partial object mid-point (general ref head) up to array value",
			module: `package test
				p.q[r].s[t].u := x {
					obj := {
						"foo": {
							"do": ["a", "b", "c"],
							"re": ["d", "e", "f"],
						},
						"bar": {
							"mi": ["g", "h", "i"],
							"fa": ["j", "k", "l"],
						}
					}
					x := obj[r][t]
				}
			`,
			query: `data.test.p.q.bar.s = x`,
			exp:   `[{"x": {"fa": {"u": ["j", "k", "l"]}, "mi": {"u": ["g", "h", "i"]}}}]`,
		},
		{
			note: "deep query into partial object (general ref head) up to array value",
			module: `package test
				p.q[r].s[t].u := x {
					obj := {
						"foo": {
							"do": ["a", "b", "c"],
							"re": ["d", "e", "f"],
						},
						"bar": {
							"mi": ["g", "h", "i"],
							"fa": ["j", "k", "l"],
						}
					}
					x := obj[r][t]
				}
			`,
			query: `data.test.p.q.bar.s.mi.u = x`,
			exp:   `[{"x": ["g", "h", "i"]}]`,
		},
		{
			note: "deep query into partial object (general ref head) and array value",
			module: `package test
				p.q[r].s[t].u := x {
					obj := {
						"foo": {
							"do": ["a", "b", "c"],
							"re": ["d", "e", "f"],
						},
						"bar": {
							"mi": ["g", "h", "i"],
							"fa": ["j", "k", "l"],
						}
					}
					x := obj[r][t]
				}
			`,
			query: `data.test.p.q.foo.s.re.u[1] = x`,
			exp:   `[{"x": "e"}]`,
		},
		{
			note: "query up to (ref head), but not into partial set",
			module: `package test
				import future.keywords
				p.q.r contains s { {"foo", "bar", "bax"}[s] }
			`,
			query: `data.test.p = x`,
			exp:   `[{"x": {"q": {"r": ["bar", "bax", "foo"]}}}]`,
		},
		{
			note: "deep query up to (ref mid-point), but not into partial set",
			module: `package test
				import future.keywords
				p.q.r contains s { {"foo", "bar", "bax"}[s] }
			`,
			query: `data.test.p.q = x`,
			exp:   `[{"x": {"r": ["bar", "bax", "foo"]}}]`,
		},
		{
			note: "deep query up to (ref tail), but not into partial set",
			module: `package test
				import future.keywords
				p.q.r contains s { {"foo", "bar", "bax"}[s] }
			`,
			query: `data.test.p.q.r = x`,
			exp:   `[{"x": ["bar", "bax", "foo"]}]`,
		},
		{
			note: "deep query into partial set",
			module: `package test
				import future.keywords
				p.q contains r { {"foo", "bar", "bax"}[r] }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": "foo"}]`,
		},
		{ // enumeration
			note: "deep query into partial object and object value, full depth, enumeration on object value",
			module: `package test
				p.q[r] := x { 
					r := ["foo", "bar"][_]
					x := {"s": {"do": 0, "re": 1, "mi": 2}}
				}
			`,
			query: `data.test.p.q.bar.s[y] = z`,
			exp:   `[{"y": "do", "z": 0}, {"y": "re", "z": 1}, {"y": "mi", "z": 2}]`,
		},
		{ // enumeration
			note: "deep query into partial object and object value, full depth, enumeration on rule path and object value",
			module: `package test
				p.q[r] := x { 
					r := ["foo", "bar"][_]
					x := {"s": {"do": 0, "re": 1, "mi": 2}}
				}
			`,
			query: `data.test.p.q[x].s[y] = z`,
			exp:   `[{"x": "foo", "y": "do", "z": 0}, {"x": "foo", "y": "re", "z": 1}, {"x": "foo", "y": "mi", "z": 2}, {"x": "bar", "y": "do", "z": 0}, {"x": "bar", "y": "re", "z": 1}, {"x": "bar", "y": "mi", "z": 2}]`,
		},
		{
			note: "deep query into partial object (ref head) and set value",
			module: `package test
				import future.keywords
				p.q contains t {
					{"do", "re", "mi"}[t]
				}
			`,
			query: `data.test.p.q.re = x`,
			exp:   `[{"x": "re"}]`,
		},
		{
			note: "deep query into partial object (general ref head) and set value",
			module: `package test
				import future.keywords
				p.q[r] contains t { 
					r := ["foo", "bar"][_] 
					{"do", "re", "mi"}[t]
				}
			`,
			query: `data.test.p.q.foo.re = x`,
			exp:   `[{"x": "re"}]`,
		},
		{
			note: "deep query into partial object (general ref head, static tail) and set value",
			module: `package test
				import future.keywords
				p.q[r].s contains t { 
					r := ["foo", "bar"][_] 
					{"do", "re", "mi"}[t]
				}
			`,
			query: `data.test.p.q.foo.s.re = x`,
			exp:   `[{"x": "re"}]`,
		},
		{
			note: "deep query into general ref to set value",
			module: `package test
				import future.keywords
				p.q[r].s contains t { 
					r := ["foo", "bar"][_] 
					t := ["do", "re", "mi"][_]
				}
			`,
			query: `data.test.p.q.foo.s = x`,
			exp:   `[{"x": ["do", "mi", "re"]}]`, // FIXME: set ordering makes this test brittle
		},
		{
			note: "deep query into general ref to object value",
			module: `package test
				p.q[r].s[t] := u { 
					r := ["foo", "bar"][_] 
					t := ["do", "re", "mi"][u]
				}
			`,
			query: `data.test.p.q.foo.s = x`,
			exp:   `[{"x": {"do": 0, "re": 1, "mi": 2}}]`,
		},
		{
			note: "deep query into general ref enumerating set values",
			module: `package test
				import future.keywords
				p.q[r].s contains t { 
					r := ["foo", "bar"][_] 
					{"do", "re", "mi"}[t]
				}
			`,
			query: `data.test.p.q.foo.s[x]`,
			// NOTE: $_term_0_0 wildcard var is filtered from eval result output
			exp: `[{"$_term_0_0": "do", "x": "do"}, {"$_term_0_0": "re", "x": "re"}, {"$_term_0_0": "mi", "x": "mi"}]`,
		},
		{
			note: "deep query into partial object and object value, non-tail var",
			module: `package test
				p.q[r].s := x { 
					r := "foo" 
					x := {"bar": {"baz": 1}}
				}
			`,
			query: `data.test.p.q.foo.s.bar = x`,
			exp:   `[{"x": {"baz": 1}}]`,
		},
		{
			note: "deep query into partial object, on first var in ref",
			module: `package test
				p.q[r].s := 1 { r := "foo" }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": {"s": 1}}]`,
		},
		{
			note: "deep query into partial object, beyond first var in ref",
			module: `package test
				p.q[r].s := 1 { r := "foo" }
			`,
			query: `data.test.p.q.foo.s = x`,
			exp:   `[{"x": 1}]`,
		},
		{
			note: "deep query into partial object, shallow rule ref",
			module: `package test
				p.q[r][s] := 1 { r := "foo"; s := "bar" }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": {"bar": 1}}]`,
		},
		{
			note: "deep query into partial object, shallow rule ref, multiple keys",
			module: `package test
				p.q[r][s] := t { l := ["do", "re", "mi"]; r := "foo"; s := l[t] }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": {"do": 0, "re": 1, "mi": 2}}]`,
		},
		{
			note: "deep query into partial object, beyond first var in ref, multiple vars",
			module: `package test
				p.q[r][s] := 1 { r := "foo"; s := "bar" }
			`,
			query: `data.test.p.q.foo.bar = x`,
			exp:   `[{"x": 1}]`,
		},
		{
			note: "deep query into partial object, beyond first var in ref, multiple vars",
			module: `package test
				p.q[r][s].t := 1 { r := "foo"; s := "bar" }
			`,
			query: `data.test.p.q.foo.bar = x`,
			exp:   `[{"x": {"t": 1}}]`,
		},
		{
			note: "deep query to partial object, overlapping rules (key override), no dynamic ref",
			module: `package test
				p.q[r] := 1 { r := "foo" }
				p.q.r := 2
			`,
			query: `data.test.p.q = x`,
			exp:   `[{"x": {"foo": 1, "r": 2}}]`,
		},
		{
			note: "deep query into partial object, overlapping rules (key override), no dynamic ref",
			module: `package test
				p.q[r] := 1 { r := "foo" }
				p.q.r := 2
			`,
			query: `data.test.p.q.r = x`,
			exp:   `[{"x": 2}]`,
		},
		{
			note: "deep query into partial object, overlapping rules, no dynamic ref",
			module: `package test
				p.q[r] := 1 { r := "foo" }
				p.q[r] := 2 { r := "bar" }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": 1}]`,
		},
		{
			note: "deep query into partial object, overlapping rules with same key/value, no dynamic ref",
			module: `package test
				p.q[r] := 1 { r := "foo" }
				p.q[r] := 1 { r := "foo" }
			`,
			query: `data.test.p.q.foo = x`,
			exp:   `[{"x": 1}]`,
		},
		{
			note: "deep query into partial object, overlapping rules, dynamic ref",
			module: `package test
				p.q[r].s := 1 { r := "r" }
				p.q.r[s] := 2 { s := "foo" }
			`,
			query: `data.test.p.q.r = x`,
			exp:   `[{"x": {"s": 1, "foo": 2}}]`,
		},
		{
			note: "deep query into partial object, overlapping rules with same key/value, dynamic ref",
			module: `package test
				p.q[r].s := 1 { r := "r" }
				p.q.r[s] := 1 { s := "s" }
			`,
			query: `data.test.p.q.r = x`,
			exp:   `[{"x": {"s": 1}}]`,
		},
		// Multiple results (enumeration)
		{
			note: "shallow query into general ref, key enumeration",
			module: `package test
				p.q[r].s[t] := u {
					r := ["a", "b", "c"][_]
					t := ["d", "e", "f"][u]
				}`,
			query: `data.test.p.q[x] = y`,
			exp: `[{"x": "a", "y": {"s": {"d": 0, "e": 1, "f": 2}}}, 
					{"x": "b", "y": {"s": {"d": 0, "e": 1, "f": 2}}},
					{"x": "c", "y": {"s": {"d": 0, "e": 1, "f": 2}}}]`,
		},
		{
			note: "query to partial object, overlapping rules, dynamic ref, key enumeration",
			module: `package test
				p.q[r].s := 1 { r := "foo" }
				p.q[r].s := 2 { r := "bar" }
			`,
			query: `data.test.p.q[i] = x`,
			exp:   `[{"i": "bar", "x": {"s": 2}}, {"i": "foo", "x": {"s": 1}}]`,
		},
		{
			note: "deep query into partial object, overlapping rules, dynamic ref, key enumeration",
			module: `package test
				p.q[r].s := 1 { r := "foo" }
				p.q[r].s := 2 { r := "bar" }
			`,
			query: `data.test.p.q[i].s = x`,
			exp:   `[{"i": "bar", "x": 2}, {"i": "foo", "x": 1}]`,
		},
		// Errors
		{
			note: "partial object generating conflicting keys",
			module: `package test
				p[k] := x {
					k := "foo"
					x := [1, 2][_]
				}`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object (ref head) generating conflicting keys (dots in head)",
			module: `package test
				p.q[k] := x {
					k := "foo"
					x := [1, 2][_]
				}`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object (general ref head) generating conflicting nested keys",
			module: `package test
				p.q[k].s := x {
					k := "foo"
					x := [1, 2][_]
				}`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
		{
			note: "partial object (general ref head) generating conflicting ref vars",
			module: `package test
				p.q[k].s := x {
					k := ["foo", "foo"][x]
				}`,
			query:  `data = x`,
			expErr: "eval_conflict_error: object keys must be unique",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			compiler := compileModules([]string{tc.module})
			txn := storage.NewTransactionOrDie(ctx, store)
			defer store.Abort(ctx, txn)

			query := NewQuery(ast.MustParseBody(tc.query)).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn)

			qrs, err := query.Run(ctx)
			if tc.expErr != "" {
				if err == nil {
					t.Fatalf("Expected error %v but got result: %v", tc.expErr, qrs)
				}
				if exp, act := tc.expErr, err.Error(); !strings.Contains(act, exp) {
					t.Fatalf("Expected error %v but got: %v", exp, act)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}

				var exp []map[string]interface{}
				_ = json.Unmarshal([]byte(tc.exp), &exp)
				if expLen, act := len(exp), len(qrs); expLen != act {
					t.Fatalf("expected %d query result:\n\n%+v,\n\ngot %d query results:\n\n%+v", expLen, exp, act, qrs)
				}
				testAssertResultSet(t, exp, qrs, false)
			}
		})
	}
}

// TODO: Remove when general rule refs are enabled by default.
func TestGeneralRuleRefsFeatureFlag(t *testing.T) {
	module := ast.MustParseModule(`package test
		p[q].r { q := "q" }`)
	mods := map[string]*ast.Module{
		"": module,
	}
	c := ast.NewCompiler()
	c.Compile(mods)

	if !strings.Contains(c.Errors.Error(), "rego_type_error: rule head must only contain string terms (except for last)") {
		t.Fatal("Expected error but got:", c.Errors)
	}

	t.Setenv("EXPERIMENTAL_GENERAL_RULE_REFS", "true")

	c = ast.NewCompiler()
	c.Compile(mods)

	if c.Errors != nil {
		t.Fatal("Unexpected error:", c.Errors)
	}
}
