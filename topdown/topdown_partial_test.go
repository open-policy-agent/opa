// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestTopDownPartialEval(t *testing.T) {

	tests := []struct {
		note        string
		unknowns    []string
		query       string
		modules     []string
		data        string
		input       string
		wantQueries []string
		wantSupport []string
	}{
		{
			note:        "empty",
			query:       "true = true",
			wantQueries: []string{``},
		},
		{
			note:        "query vars",
			query:       "x = 1",
			wantQueries: []string{`x = 1`},
		},
		{
			note:        "trivial",
			query:       "input.x = 1",
			wantQueries: []string{`input.x = 1`},
		},
		{
			note:        "trivial reverse",
			query:       "1 = input.x",
			wantQueries: []string{`1 = input.x`},
		},
		{
			note:  "trivial both",
			query: "input.x = input.y",
			wantQueries: []string{
				`input.x = input.y`,
			},
		},
		{
			note:        "transitive",
			query:       "input.x = y; y[0] = z; z = 1; plus(z, 2, 3)",
			wantQueries: []string{`input.x = y; y[0] = z; z = 1; plus(z, 2, 3)`},
		},
		{
			note:  "vars",
			query: "x = 1; y = 2; input.x = x; y = input.y",
			wantQueries: []string{
				`input.x = 1; 2 = input.y; x = 1; y = 2`,
			},
		},
		{
			note:  "complete: substitute",
			query: "input.x = data.test.p; data.test.q = input.y",
			modules: []string{
				`package test
				p = x { x = "foo" }
				q = x { x = "bar" }`,
			},
			wantQueries: []string{
				`"foo" = input.x; "bar" = input.y`,
			},
		},
		{
			note:  "iterate vars",
			query: "a = [1,2]; a[i] = x",
			wantQueries: []string{
				`a = [1, 2]; i = 0; x = 1`,
				`a = [1, 2]; i = 1; x = 2`,
			},
		},
		{
			note:  "iterate data",
			query: "data.x[i] = input.x",
			data:  `{"x": [1,2,3]}`,
			wantQueries: []string{
				`input.x = 1; i = 0`,
				`input.x = 2; i = 1`,
				`input.x = 3; i = 2`,
			},
		},
		{
			note:  "iterate rules: partial object",
			query: `data.test.p[x] = input.x`,
			modules: []string{
				`package test
				p["a"] = "b"
				p["b"] = "c"
				p["c"] = "d"`,
			},
			wantQueries: []string{
				`"b" = input.x; x = "a"`,
				`"c" = input.x; x = "b"`,
				`"d" = input.x; x = "c"`,
			},
		},
		{
			note:  "iterate rules: partial set",
			query: `input.x = x; data.test.p[x]`,
			modules: []string{
				`package test
				p[1]
				p[2]
				p[3]`,
			},
			wantQueries: []string{
				`input.x = x; 1 = x; x_term_0_1 = 1`,
				`input.x = x; 2 = x; x_term_0_1 = 2`,
				`input.x = x; 3 = x; x_term_0_1 = 3`,
			},
		},
		{
			note:  "iterate keys: sets",
			query: `input = x; s = {1,2}; s[x] = y`,
			wantQueries: []string{
				`input = x; 1 = x; s = {1, 2}; y = 1`,
				`input = x; 2 = x; s = {1, 2}; y = 2`,
			},
		},
		{
			note:  "iterate keys: objects",
			query: `input = x; o = {"a": 1, "b": 2}; o[x] = y`,
			wantQueries: []string{
				`input = x; "a" = x; o = {"a": 1, "b": 2}; y = 1`,
				`input = x; "b" = x; o = {"a": 1, "b": 2}; y = 2`,
			},
		},
		{
			note:  "iterate keys: saved",
			query: `x = input; y = [x]; z = y[i][j] `,
			wantQueries: []string{
				`x = input; x[j] = z; i = 0; y = [x]`,
			},
		},
		{
			note:  "single term: save",
			query: `input.x = x; data.test.p[x]`,
			modules: []string{
				`package test
				p[y] { y = "foo" }
				p[z] { z = "bar" }`,
			},
			wantQueries: []string{
				`input.x = x; y1 = x; y1 = "foo"; y1 = x_term_0_1; x_term_0_1 != false`,
				`input.x = x; z2 = x; z2 = "bar"; z2 = x_term_0_1; x_term_0_1 != false`,
			},
		},
		{
			note:  "reference: partial object",
			query: "data.test.p[x].foo = 1",
			modules: []string{
				`package test
				p[x] = {y: z} { x = input.x; y = "foo"; z = 1 }
				p[x] = {y: z} { x = input.y; y = "bar"; z = 2 }`,
			},
			wantQueries: []string{
				`x = input.x`,
			},
		},
		{
			note:  "reference: partial set",
			query: "data.test.p[x].foo = 1",
			modules: []string{
				`package test
				p[x] { x = {y: z}; y = "foo"; z = input.x }
				p[x] { x = {y: z}; y = "bar"; z = input.x }`,
			},
			wantQueries: []string{
				`z1 = input.x; z1 = 1; x = {"foo": z1}`,
			},
		},
		{
			note:  "reference: complete",
			query: "data.test.p = 1",
			modules: []string{
				`package test

				p = x { input.x = x }`,
			},
			wantQueries: []string{
				`input.x = x1; x1 = 1`,
			},
		},
		{
			note:  "namespace: complete",
			query: "data.test.p = x",
			modules: []string{
				`package test
				 p = 1 { input.y = x; x = 2 }`,
			},
			wantQueries: []string{
				`input.y = x1; x1 = 2; 1 = x`,
			},
		},
		{
			note:  "namespace: complete head",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { input.x = x }`,
			},
			wantQueries: []string{
				`input.x = x1; x1 = x`,
			},
		},
		{
			note:  "namespace: partial set",
			query: "data.test.p[[x, y]]",
			modules: []string{
				`package test
				p[[y, x]] { input.z = z; z = y; a = input.a; a = x }`,
			},
			wantQueries: []string{
				`input.z = z1; z1 = x; a1 = input.a; a1 = y; x_term_0_0 = [x, y]`,
			},
		},
		{
			note:  "namespace: partial object",
			query: "input.x = x; data.test.p[x] = y; y = 2",
			modules: []string{
				`package test
				p[y] = x { y = "foo"; x = 2 }`,
			},
			wantQueries: []string{
				`input.x = x; y1 = x; y1 = "foo"; x1 = 2; x1 = y; y = 2`,
			},
		},
		{
			note:  "namespace: embedding",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { input.x = [y]; y = x }`,
			},
			wantQueries: []string{
				`input.x = [y1]; y1 = x1; x1 = x`,
			},
		},
		{
			note:  "namespace: multiple",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = [x, z] { input.x = y; y = x; q = z }
				q = x { input.y = y; x =  y }`,
			},
			wantQueries: []string{
				`input.x = y1; y1 = x1; input.y = y2; x2 = y2; x2 = z1; [x1, z1] = x`,
			},
		},
		{
			note:  "namespace: calls",
			query: "data.test.p = x",
			modules: []string{
				`package test

				p {
					a = "a"
					b = input.b
					a != b
				}
				`,
			},
			wantQueries: []string{
				`b1 = input.b; "a" != b1; x = true`,
			},
		},
		{
			note:  "namespace: reference head",
			query: "data.test.p = x",
			modules: []string{
				`package test

				p {
					input = x
					x[0] = true
				}`,
			},
			wantQueries: []string{
				`input = x1; x1[0] = true; true = x`,
			},
		},
		{
			note:  "ignore conflicts: complete",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = true { input.x = 1 }
				p = false { input.x = 2 }`,
			},
			wantQueries: []string{
				`input.x = 1; x = true`,
				`input.x = 2; x = false`,
			},
		},
		{
			note:  "ignore conflicts: functions",
			query: "data.test.f(1, x)",
			modules: []string{
				`package test
				f(x) = true { input.x = x }
				f(x) = false { input.y = x }`,
			},
			wantQueries: []string{
				`input.x = 1; x = true`,
				`input.y = 1; x = false`,
			},
		},
		{
			note:        "comprehensions: save",
			query:       `x = [true | true]; y = {true | true}; z = {a: true | a = "foo"}`,
			wantQueries: []string{`x = [true | true]; y = {true | true}; z = {a: true | a = "foo"}`},
		},
		{
			note:        "comprehensions: closure",
			query:       `i = 1; xs = [x | x = data.foo[i]]`,
			wantQueries: []string{`xs = [x | x = data.foo[1]]; i = 1`},
		},
		{
			note:  "save: sub path",
			query: "input.x = 1; input.y = 2; input.z.a = 3; input.z.b = x",
			input: `{"x": 1, "z": {"b": 4}}`,
			wantQueries: []string{
				`input.y = 2; input.z.a = 3; x = 4`,
			},
			unknowns: []string{
				"input.y",
				"input.z.a",
			},
		},
		{
			note:  "save: virtual doc",
			query: "data.test.p = 1; data.test.q = 2",
			modules: []string{
				`package test
				p = x { x = 1 }
				q = y { y = input.y }`,
			},
			wantQueries: []string{
				`data.test.p = 1; y1 = input.y; y1 = 2`,
			},
			unknowns: []string{
				"input",
				"data.test.p",
			},
		},
		{
			note:  "save: full extent: partial set",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p[x] { input.x = x }
				p[x] { input.y = x }`,
			},
			wantQueries: []string{`data.test.p = x`},
		},
		{
			note:  "save: full extent: partial object",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p[x] = y { x = input.x; y = input.y }
				p[x] = y { x = input.z; y = input.a }`,
			},
			wantQueries: []string{`data.test.p = x`},
		},
		{
			note:  "save: full extent",
			query: "data.test = x",
			modules: []string{
				`package test.a
				p = 1`,
				`package test
				q = 2`,
			},
			wantQueries: []string{`data.test = x`},
		},
		{
			note:  "save: full extent: iteration",
			query: "data.test[x] = y",
			modules: []string{
				`package test
				s[x] { x = input.x }
				p[x] = y { x = input.x; y = input.y }
				r = x { x = input.x }`,
			},
			wantQueries: []string{
				`data.test.s = y; x = "s"`,
				`data.test.p = y; x = "p"`,
				`x1 = input.x; x1 = y; x = "r"`,
			},
		},
		{
			note:  "save: call embedded",
			query: "x = input; a = [x]; count([a], n)",
			wantQueries: []string{
				`x = input; count([[x]], n); a = [x]`,
			},
		},
		{
			note:  "save: with",
			query: "data.test.p = true",
			modules: []string{
				`package test
				p { input.x = 1; q with input as {"y": 2} }
				q { input.y = 2 }`,
			},
			wantQueries: []string{
				`input.x = 1; data.test.q with input as {"y": 2}`,
			},
		},
		{
			note:  "save: else",
			query: "data.test.p = x",
			modules: []string{
				`package test
				p = x { q = x }
				q = 100 { false } else = 200 { true }`,
			},
			wantQueries: []string{
				`data.test.q = x1; x1 = x`,
			},
		},
		{
			note:  "save: ignore ast",
			query: "time.now_ns(x)",
			wantQueries: []string{
				`time.now_ns(x)`,
			},
		},
		{
			note:  "support: default trivial",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q }
				q { input.x = 1 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = true { input.x = 1 }
				default p = false`,
			},
		},
		{
			note:  "support: default with iteration (disjunction)",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q }
				q { input.x = 1 }
				q { input.x = 2 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = true { input.x = 1 }
				p = true { input.x = 2 }
				default p = false
				`,
			},
		},
		{
			note:  "support: default with iteration (data)",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q }
				q { input.x = a[i] }
				a = [1, 2]
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = true { 1 = input.x }
				p = true { 2 = input.x }
				default p = false`,
			},
		},
		{
			note:  "support: default with disjunction",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = 0
				p = 1 { q }
				p = 2 { r }
				q { input.x = 1 }
				r { input.x = 2 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = 1 { input.x = 1 }
				p = 2 { input.x = 2 }
				default p = 0
				`,
			},
		},
		{
			note:  "support: default head vars",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = 0
				p = x { x = 1; input.x = 1 }
				p = x { input.x = x }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				p = 1 { input.x = 1 }
				p = x { input.x = x }
				default p = 0
				`,
			},
		},
		{
			note:  "support: default multiple",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q = true; s } # using q = true syntax to avoid dealing with implicit != false expr
				default q = false
				q { r }
				r { input.x = 1 }
				r { input.y = 2 }
				s { input.z = 3 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test
				q = true { input.x = 1 }
				q = true { input.y = 2 }
				default q = false
				p = true { data.partial.test.q = true; input.z = 3 }
				default p = false
				`,
			},
		},
		{
			note:  "support: default bindings",
			query: "data.test.p = x",
			modules: []string{
				`package test
				default p = false
				p { q[x]; not r[x] }
				q[1] { input.x = 1 }
				q[2] { input.y = 2 }
				r[1] { input.z = 3 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = x`,
			},
			wantSupport: []string{
				`package partial.test

				p = true { input.x = 1; not data.partial.__not1_1__ }
				p = true { input.y = 2 }
				default p = false
				`,
				`package partial

				__not1_1__ { input.z = 3 }
				`,
			},
		},
		{
			note:  "support: iterate default",
			query: "data.test[x] = y",
			modules: []string{
				`package test
				default p = 0
				p = 1 { q }
				q { input.x = 1 }
				`,
			},
			wantQueries: []string{
				`data.partial.test.p = y; x = "p"`,
				`input.x = 1; x = "q"; y = true`,
			},
			wantSupport: []string{
				`package partial.test
				p = 1 { input.x = 1 }
				default p = 0`,
			},
		},
		{
			note:  "support: default memoized",
			query: "data.test.q[x] = y; data.test.p = z",
			modules: []string{
				`package test

				q = [1,2]

				default p = false
				p { input.x = 1 }`,
			},
			wantQueries: []string{
				`data.partial.test.p = z; x = 0; y = 1`,
				`data.partial.test.p = z; x = 1; y = 2`,
			},
			wantSupport: []string{
				`package partial.test

				p = true { input.x = 1 }
				default p = false`,
			},
		},
		{
			note:  "support: negation",
			query: "data.test.p = true",
			modules: []string{
				`package test
				p { input.x = 1; not q; not r }
				q { input.y = 2 }
				r { false }`,
			},
			wantQueries: []string{
				`input.x = 1; not data.partial.__not1_1__`,
			},
			wantSupport: []string{
				`package partial

				__not1_1__ { input.y = 2 }`,
			},
		},
		{
			note:  "support: negation with input",
			query: "input.x = x; input.y = y; not data.test.p[[x,y]]",
			modules: []string{
				`package test

				p[[0, 1]]
				p[[2, 3]]`,
			},
			wantQueries: []string{
				`input.x = x; input.y = y; not data.partial.__not0_2__(x, y)`,
			},
			wantSupport: []string{
				`package partial

				__not0_2__(x, y) { 0 = x; 1 = y }
				__not0_2__(x, y) { 2 = x; 3 = y }`,
			},
		},
	}

	ctx := context.Background()

	for _, tc := range tests {
		params := fixtureParams{
			note:    tc.note,
			query:   tc.query,
			modules: tc.modules,
			data:    tc.data,
			input:   tc.input,
		}
		prepareTest(ctx, t, params, func(ctx context.Context, t *testing.T, f fixture) {

			var save []string

			if tc.unknowns == nil {
				save = []string{"input"}
			} else {
				save = tc.unknowns
			}

			unknowns := make([]*ast.Term, len(save))
			for i, s := range save {
				unknowns[i] = ast.MustParseTerm(s)
			}

			var buf BufferTracer

			query := NewQuery(f.query).
				WithCompiler(f.compiler).
				WithStore(f.store).
				WithTransaction(f.txn).
				WithInput(f.input).
				WithTracer(&buf).
				WithUnknowns(unknowns)

			// Set genvarprefix so that tests can refer to vars in generated
			// expressions.
			query.genvarprefix = "x"

			partials, support, err := query.PartialRun(ctx)

			if err != nil {
				if buf != nil {
					PrettyTrace(os.Stdout, buf)
				}
				t.Fatal(err)
			}

			expectedQueries := make([]ast.Body, len(tc.wantQueries))
			for i := range tc.wantQueries {
				expectedQueries[i] = ast.MustParseBody(tc.wantQueries[i])
			}

			queriesA, queriesB := bodySet(partials), bodySet(expectedQueries)
			if !queriesB.Equal(queriesA) {
				missing := queriesB.Diff(queriesA)
				extra := queriesA.Diff(queriesB)
				t.Errorf("Partial evaluation results differ. Expected %d queries but got %d queries:\nMissing:\n%v\nExtra:\n%v", len(queriesB), len(queriesA), missing, extra)
			}

			expectedSupport := make([]*ast.Module, len(tc.wantSupport))
			for i := range tc.wantSupport {
				expectedSupport[i] = ast.MustParseModule(tc.wantSupport[i])
			}
			supportA, supportB := moduleSet(support), moduleSet(expectedSupport)
			if !supportA.Equal(supportB) {
				missing := supportB.Diff(supportA)
				extra := supportA.Diff(supportB)
				t.Errorf("Partial evaluation results differ. Expected %d modules but got %d:\nMissing:\n%v\nExtra:\n%v", len(supportB), len(supportA), missing, extra)
			}
		})
	}
}

type fixtureParams struct {
	note    string
	data    string
	modules []string
	query   string
	input   string
}

type fixture struct {
	query    ast.Body
	compiler *ast.Compiler
	store    storage.Store
	txn      storage.Transaction
	input    *ast.Term
}

func prepareTest(ctx context.Context, t *testing.T, params fixtureParams, f func(context.Context, *testing.T, fixture)) {

	test.Subtest(t, params.note, func(t *testing.T) {

		var store storage.Store

		if len(params.data) > 0 {
			j := util.MustUnmarshalJSON([]byte(params.data))
			store = inmem.NewFromObject(j.(map[string]interface{}))
		} else {
			store = inmem.New()
		}

		storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

			compiler := ast.NewCompiler()
			modules := map[string]*ast.Module{}

			for i, module := range params.modules {
				modules[fmt.Sprint(i)] = ast.MustParseModule(module)
			}

			if compiler.Compile(modules); compiler.Failed() {
				t.Fatal(compiler.Errors)
			}

			var input *ast.Term
			if len(params.input) > 0 {
				input = ast.MustParseTerm(params.input)
			}

			queryContext := ast.NewQueryContext()
			if input != nil {
				queryContext = queryContext.WithInput(input.Value)
			}

			queryCompiler := compiler.QueryCompiler().WithContext(queryContext)

			compiledQuery, err := queryCompiler.Compile(ast.MustParseBody(params.query))
			if err != nil {
				t.Fatal(err)
			}

			f(ctx, t, fixture{
				query:    compiledQuery,
				compiler: compiler,
				store:    store,
				txn:      txn,
				input:    input,
			})

			return nil
		})
	})
}

type bodySet []ast.Body

func (s bodySet) String() string {
	buf := make([]string, len(s))
	for i := range s {
		buf[i] = fmt.Sprintf("body %d: %v", i+1, s[i].String())
	}
	return strings.Join(buf, "\n")
}

func (s bodySet) Contains(b ast.Body) bool {
	for i := range s {
		if s[i].Equal(b) {
			return true
		}
	}
	return false
}

func (s bodySet) Diff(other bodySet) (r bodySet) {
	for i := range s {
		if !other.Contains(s[i]) {
			r = append(r, s[i])
		}
	}
	return r
}

func (s bodySet) Equal(other bodySet) bool {
	return len(s.Diff(other)) == 0 && len(other.Diff(s)) == 0
}

type moduleSet []*ast.Module

func (s moduleSet) String() string {
	buf := make([]string, len(s))
	for i := range s {
		buf[i] = fmt.Sprintf("module %d: %v", i+1, s[i].String())
	}
	return strings.Join(buf, "\n")
}

func (s moduleSet) Contains(b *ast.Module) bool {
	for i := range s {
		if s[i].Equal(b) {
			return true
		}
	}
	return false
}

func (s moduleSet) Diff(other moduleSet) (r moduleSet) {
	for i := range s {
		if !other.Contains(s[i]) {
			r = append(r, s[i])
		}
	}
	return r
}

func (s moduleSet) Equal(other moduleSet) bool {
	return len(s.Diff(other)) == 0 && len(other.Diff(s)) == 0
}
