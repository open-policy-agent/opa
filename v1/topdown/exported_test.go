// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
	"github.com/open-policy-agent/opa/v1/test/cases"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func TestRego(t *testing.T) {
	t.Parallel()

	for _, tc := range cases.MustLoad("../test/cases/testdata/v0").Sorted().Cases {
		t.Run("v0/"+tc.Note, func(t *testing.T) {
			t.Parallel()

			testRun(t, tc, ast.RegoV0)
		})
	}
	for _, tc := range cases.MustLoad("../test/cases/testdata/v1").Sorted().Cases {
		t.Run("v1/"+tc.Note, func(t *testing.T) {
			t.Parallel()

			testRun(t, tc, ast.RegoV1)
		})
	}
}

func TestOPARego(t *testing.T) {
	t.Parallel()

	for _, tc := range cases.MustLoad("testdata/cases").Sorted().Cases {
		t.Run(tc.Note, func(t *testing.T) {
			t.Parallel()

			testRun(t, tc, ast.RegoV0)
		})
	}
}

func TestRegoWithNDBCache(t *testing.T) {
	t.Parallel()

	for _, tc := range cases.MustLoad("../test/cases/testdata/v0").Sorted().Cases {
		t.Run("v0/"+tc.Note, func(t *testing.T) {
			t.Parallel()

			testRun(t, tc, ast.RegoV0, func(q *Query) *Query {
				return q.WithNDBuiltinCache(builtins.NDBCache{})
			})
		})
	}
	for _, tc := range cases.MustLoad("../test/cases/testdata/v1").Sorted().Cases {
		t.Run("v1/"+tc.Note, func(t *testing.T) {
			t.Parallel()

			testRun(t, tc, ast.RegoV1, func(q *Query) *Query {
				return q.WithNDBuiltinCache(builtins.NDBCache{})
			})
		})
	}
}

type opt func(*Query) *Query

func testRun(t *testing.T, tc cases.TestCase, regoVersion ast.RegoVersion, opts ...opt) {
	t.Helper()
	for k, v := range tc.Env {
		t.Setenv(k, v)
	}

	ctx := t.Context()

	modules := map[string]string{}
	for i, module := range tc.Modules {
		modules[fmt.Sprintf("test-%d.rego", i)] = module
	}

	compiler := ast.MustCompileModulesWithOpts(modules, ast.CompileOpts{
		ParserOptions: ast.ParserOptions{
			RegoVersion: regoVersion,
		},
	})
	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(tc.Query))

	if err != nil {
		t.Fatal(err)
	}

	var store storage.Store

	if tc.Data != nil {
		store = inmem.NewFromObject(*tc.Data)
	} else {
		store = inmem.New()
	}

	txn := storage.NewTransactionOrDie(ctx, store)

	var input *ast.Term

	if tc.InputTerm != nil {
		input = ast.MustParseTerm(*tc.InputTerm)
	} else if tc.Input != nil {
		input = ast.NewTerm(ast.MustInterfaceToValue(*tc.Input))
	}

	cncl := NewCancel()
	buf := NewBufferTracer()
	q := NewQuery(query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithStrictBuiltinErrors(tc.StrictError).
		WithTracer(buf).
		WithCancel(cncl)

	for _, o := range opts {
		q = o(q)
	}

	rs, err := q.Run(ctx)

	if tc.WantError != nil {
		testAssertErrorText(t, *tc.WantError, err)
	}

	if tc.WantErrorCode != nil {
		testAssertErrorCode(t, *tc.WantErrorCode, err)
	}

	if err != nil && tc.WantErrorCode == nil && tc.WantError == nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tc.WantResult != nil {
		testAssertResultSet(t, *tc.WantResult, rs, tc.SortBindings, tc.IgnoreGeneratedVars)
	}

	if tc.WantResult == nil && tc.WantErrorCode == nil && tc.WantError == nil {
		t.Fatal("expected one of: 'want_result', 'want_error_code', or 'want_error'")
	}

	if testing.Verbose() {
		PrettyTrace(os.Stderr, *buf)
	}
}

func testAssertResultSet(t *testing.T, wantResult []map[string]any, rs QueryResultSet, sortBindings bool, ignoreGeneratedVars bool) {

	exp := ast.NewSet()

	for _, b := range wantResult {
		obj := ast.NewObject()
		for k, v := range b {
			obj.Insert(ast.StringTerm(k), ast.NewTerm(ast.MustInterfaceToValue(v)))
		}
		exp.Add(ast.NewTerm(obj))
	}

	got := ast.NewSet()

	for _, b := range rs {
		obj := ast.NewObject()
		for k, term := range b {
			v, err := ast.JSON(term.Value)
			if err != nil {
				t.Fatal(err)
			}
			if sortBindings {
				sort.Sort(resultSet(v.([]any)))
			}
			name := string(k)
			if !ignoreGeneratedVars || !strings.HasPrefix(name, "__localq") {
				obj.Insert(ast.StringTerm(name), ast.NewTerm(ast.MustInterfaceToValue(v)))
			}
		}
		got.Add(ast.NewTerm(obj))
	}

	if exp.Compare(got) != 0 {
		t.Fatalf("unexpected query result:\nexp: %v\ngot: %v", exp, got)
	}
}

func testAssertErrorCode(t *testing.T, wantErrorCode string, err error) {
	e, ok := err.(*Error)
	if !ok {
		t.Fatal("expected topdown error but got:", err)
	}

	if e.Code != wantErrorCode {
		t.Fatalf("expected error code %q but got %q", wantErrorCode, e.Code)
	}
}

func testAssertErrorText(t *testing.T, wantText string, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error but got success")
	}
	if !strings.Contains(err.Error(), wantText) {
		t.Fatalf("expected topdown error text %q but got: %q", wantText, err.Error())
	}
}
