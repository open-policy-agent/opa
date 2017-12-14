package topdown

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestSaveSet(t *testing.T) {

	tests := []struct {
		terms    []string
		input    string
		expected bool
	}{
		{
			terms:    []string{},
			input:    `input`,
			expected: false,
		},
		{
			terms:    []string{`input`},
			input:    `data.x`,
			expected: false,
		},
		{
			terms:    []string{`input`},
			input:    `input.x`,
			expected: true,
		},
		{
			terms:    []string{`input.x`, `input.y`},
			input:    `input`,
			expected: true,
		},
		{
			terms:    []string{`input.x`, `input.y`},
			input:    `input.z`,
			expected: false,
		},
		{
			terms:    []string{`input.x`, `input.y`},
			input:    `input.x.foo`,
			expected: true,
		},
	}

	for _, tc := range tests {
		terms := make([]*ast.Term, len(tc.terms))
		for i := range terms {
			terms[i] = ast.MustParseTerm(tc.terms[i])
		}
		saveSet := newSaveSet(terms)
		input := ast.MustParseTerm(tc.input)
		if saveSet.Contains(input) != tc.expected {
			t.Errorf("Expected %v for %v contains %v", tc.expected, terms, input)
		}
	}

}

// FIXME(tsandall): wip
func testPartial(t *testing.T) {

	saveInput := []string{`input`}

	tests := []struct {
		note    string
		query   string
		modules []string
		data    string
		partial []string
		input   string
	}{
		{
			note:    "empty",
			query:   "x = 1",
			partial: saveInput,
		},
		{
			note:    "save",
			query:   "input.x = 1",
			partial: saveInput,
		},
		{
			note:    "iterate",
			query:   "x = [1,2,3]; x[input.x]",
			partial: saveInput,
		},
		{
			note:    "namespace",
			query:   "data.test.p[x]; input.y = x",
			partial: saveInput,
			modules: []string{
				`package test

				p[x] { x = input.x }`,
			},
		},
		{
			note:    "complete",
			query:   "data.test.p = input.x",
			partial: saveInput,
			modules: []string{
				`package test

				p = x { x = "foo" }`,
			},
		},
		{
			note:  "complete-namespace",
			query: "data.test.p = x; x = input.y",
			modules: []string{
				`package test

				p = x { input.x = x }`,
			},
			partial: saveInput,
		},
		{
			note:    "both",
			query:   "input.x = input.y",
			partial: saveInput,
		},
		{
			note:    "transitive",
			query:   "input.x = x; x[0] = y; x = z; y = 1; z = 2",
			partial: saveInput,
		},
		{
			note:  "call",
			query: "input.a = a; data.test.f(a) = b; b[0] = c",
			modules: []string{
				`package test

				f(x) = [y] {
					x = 1
					y = x
				}

				f(x) = [y] {
					x = 2
					y = 3
				}`,
			},
			partial: saveInput,
		},
		{
			note:    "else",
			query:   "data.test.p = x",
			partial: saveInput,
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

			partial := make([]*ast.Term, len(tc.partial))
			for i := range tc.partial {
				partial[i] = ast.MustParseTerm(tc.partial[i])
			}

			query := NewQuery(f.query).
				WithCompiler(f.compiler).
				WithStore(f.store).
				WithTransaction(f.txn).
				WithInput(f.input).
				WithPartial(partial)

			partials, err := query.PartialRun(ctx)
			t.Logf("err: %v", err)

			for i := range partials {
				t.Logf("%v", partials[i])
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

func toTerm(qrs QueryResultSet) *ast.Term {
	set := ast.NewSet()
	for _, qr := range qrs {
		obj := ast.NewObject()
		for k, v := range qr {
			if !k.IsWildcard() {
				obj.Insert(ast.NewTerm(k), v)
			}
		}
		set.Add(ast.NewTerm(obj))
	}
	return ast.NewTerm(set)
}
