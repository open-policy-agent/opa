// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
)

const employeeGQLSchema = `
       type Employee {
               id: String!
               salary: Int!
       }

       schema {
               query: Query
       }

       type Query {
               employeeByID(id: String!): Employee
       }`

const invalidEmployeeGQLSchema = `
	type Employee {
		id: String!
		salary: Int!
	}

	schema {
		query: Query
	}

	type Broken {
		fixme

	type Query {
		employeeByID(id: String!): Employee
	}`

const employeeGQLQueryAST = `{"Operations":[{"Name":"","Operation":"query","SelectionSet":[{"Alias":"employeeByID","Arguments":[{"Name":"id","Value":{"Kind":3,"Raw":"alice"}}],"Name":"employeeByID","SelectionSet":[{"Alias":"salary","Name":"salary"}]}]}]}`

const employeeGQLSchemaAST = `{"Definitions":[{"BuiltIn":false,"Description":"","Fields":[{"Description":"","Name":"id","Type":{"NamedType":"String","NonNull":true}},{"Description":"","Name":"salary","Type":{"NamedType":"Int","NonNull":true}}],"Kind":"OBJECT","Name":"Employee"},{"BuiltIn":false,"Description":"","Fields":[{"Arguments":[{"Description":"","Name":"id","Type":{"NamedType":"String","NonNull":true}}],"Description":"","Name":"employeeByID","Type":{"NamedType":"Employee","NonNull":false}}],"Kind":"OBJECT","Name":"Query"}],"Schema":[{"Description":"","OperationTypes":[{"Operation":"query","Type":"Query"}]}]}`

var employeeGQLQueryASTObj = ast.MustParseTerm(employeeGQLQueryAST).Value.(ast.Object)
var employeeGQLSchemaASTObj = ast.MustParseTerm(employeeGQLSchemaAST).Value.(ast.Object)

func TestGraphQLParseString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		schema  string
		query   string
		result  string
		wantErr bool
	}{
		{
			note:    "valid employee query and GQL schema",
			schema:  employeeGQLSchema,
			query:   `{ employeeByID(id: "alice") { salary } }`,
			result:  `[{"Operations": [{"Name": "", "Operation": "query", "SelectionSet": [{"Alias": "employeeByID", "Arguments": [{"Name": "id", "Value": {"Kind": 3, "Raw": "alice"}}], "Name": "employeeByID", "SelectionSet": [{"Alias": "salary", "Name": "salary"}]}]}]}, {"Definitions": [{"BuiltIn": false, "Description": "", "Fields": [{"Description": "", "Name": "id", "Type": {"NamedType": "String", "NonNull": true}}, {"Description": "", "Name": "salary", "Type": {"NamedType": "Int", "NonNull": true}}], "Kind": "OBJECT", "Name": "Employee"}, {"BuiltIn": false, "Description": "", "Fields": [{"Arguments": [{"Description": "", "Name": "id", "Type": {"NamedType": "String", "NonNull": true}}], "Description": "", "Name": "employeeByID", "Type": {"NamedType": "Employee", "NonNull": false}}], "Kind": "OBJECT", "Name": "Query"}], "Schema": [{"Description": "", "OperationTypes": [{"Operation": "query", "Type": "Query"}]}]}]`,
			wantErr: false,
		},
		{
			note:    "valid employee schema, invalid query",
			schema:  employeeGQLSchema,
			query:   `{employeeByID("alice"`,
			result:  "",
			wantErr: true,
		},
		{
			note:    "invalid",
			schema:  invalidEmployeeGQLSchema,
			query:   `{ employeeByID(id:"bob") } `, // missing fields
			result:  "",
			wantErr: true,
		},
		{
			note:    "empty",
			schema:  ``,
			query:   `{ employeeByID(id: "charlie") { id salary } }`,
			result:  "",
			wantErr: true,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result *ast.Term
			var err error
			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {

				err = builtinGraphQLParse(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{ast.NewTerm(ast.String(tc.query)), ast.NewTerm(ast.String(tc.schema))},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				if (result != nil) && (tc.result != result.String()) {
					t.Errorf("Unexpected result, expected %#v, got %s", tc.result, result.String())
					return
				}
			}
			// Without the cache
			err = builtinGraphQLParse(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{ast.NewTerm(ast.String(tc.query)), ast.NewTerm(ast.String(tc.schema))},
				func(term *ast.Term) error {
					result = term
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			if (result != nil) && (tc.result != result.String()) {
				t.Errorf("Unexpected result, expected %#v, got %s", tc.result, result.String())
				return
			}
		})
	}
}

func TestGraphQLParseObject(t *testing.T) {
	t.Parallel()

	// Create a default Term with the expected result for the happy path here
	// so we can include it in the test case table
	defaultExpectedResult := ast.ArrayTerm(
		ast.NewTerm(employeeGQLQueryASTObj),
		ast.NewTerm(employeeGQLSchemaASTObj),
	)

	cases := []struct {
		note    string
		schema  string
		query   string
		result  *ast.Term
		wantErr bool
	}{
		{
			note:    "valid employee schema, valid query",
			schema:  employeeGQLSchemaAST,
			query:   `{ employeeByID(id: "alice") { salary } }`,
			result:  defaultExpectedResult,
			wantErr: false,
		},
		{
			note:    "valid employee schema, invalid query",
			schema:  employeeGQLSchemaAST,
			query:   `{employeeByID("alice"`,
			result:  defaultExpectedResult,
			wantErr: true,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result *ast.Term
			var err error
			inputTerm := ast.NewTerm(ast.MustParseTerm(tc.schema).Value.(ast.Object))

			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {

				err = builtinGraphQLParse(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{ast.NewTerm(ast.String(tc.query)), inputTerm},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				if result != nil && !tc.wantErr {
					if !tc.result.Equal(result) {
						t.Errorf("Unexpected result, expected\n%s\ngot\n%s\n", tc.result.String(), result.String())
						return
					}
				}
				result = nil
			}
			// Without the cache
			err = builtinGraphQLParse(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{ast.NewTerm(ast.String(tc.query)), inputTerm},
				func(term *ast.Term) error {
					result = term
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			if result != nil && !tc.wantErr {
				if !tc.result.Equal(result) {
					t.Errorf("Unexpected result, expected\n%s\ngot\n%s\n", tc.result.String(), result.String())
					return
				}
			}
			result = nil
		})
	}
}

func TestGraphQLSchemaIsValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		schema  *ast.Term
		result  ast.Value
		wantErr bool
	}{
		{
			note:    "valid employee",
			schema:  ast.NewTerm(ast.String(employeeGQLSchema)),
			result:  ast.Boolean(true),
			wantErr: false,
		},
		{
			note:    "invalid",
			schema:  ast.NewTerm(ast.String(invalidEmployeeGQLSchema)),
			result:  ast.Boolean(false),
			wantErr: false,
		},
		{
			note:    "empty",
			schema:  ast.NewTerm(ast.String(``)),
			result:  ast.Boolean(true), // An empty schema is valid because it is merged with the base schema
			wantErr: false,
		},
		{
			note:    "valid employee schema as object",
			schema:  ast.NewTerm(ast.MustParseTerm(employeeGQLSchemaAST).Value.(ast.Object)),
			result:  ast.Boolean(true),
			wantErr: false,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result ast.Value
			var err error
			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {
				err = builtinGraphQLSchemaIsValid(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{tc.schema},
					func(term *ast.Term) error {
						result = term.Value
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				if tc.result != result {
					t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
					return
				}
			}
			// Without the cache
			err = builtinGraphQLSchemaIsValid(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{tc.schema},
				func(term *ast.Term) error {
					result = term.Value
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			if tc.result != result {
				t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
				return
			}
		})
	}
}

func TestGraphQLParseAndVerify(t *testing.T) {
	t.Parallel()

	// Use this to map result item position to purpose for better error messages
	resultItemDescription := []string{"is_valid", "query_ast", "schema_ast"}

	failureResult := ast.ArrayTerm(
		ast.BooleanTerm(false),
		ast.MustParseTerm("{}"),
		ast.MustParseTerm("{}"),
	)

	cases := []struct {
		note    string
		schema  string
		query   string
		result  *ast.Term
		wantErr bool
	}{
		{
			note:   "valid employee query and GQL schema",
			schema: employeeGQLSchema,
			query:  `{ employeeByID(id: "alice") { salary } }`,
			result: ast.ArrayTerm(
				ast.BooleanTerm(true),
				ast.NewTerm(employeeGQLQueryASTObj),
				ast.NewTerm(employeeGQLSchemaASTObj),
			),
			wantErr: false,
		},
		{
			note:    "valid employee schema, invalid query",
			schema:  employeeGQLSchema,
			query:   `{employeeByID("alice"`,
			result:  failureResult,
			wantErr: false,
		},
		{
			note:    "invalid schema",
			schema:  invalidEmployeeGQLSchema,
			query:   `{ employeeByID(id: "alice") { salary } }`,
			result:  failureResult,
			wantErr: false,
		},
		{
			note:    "invalid query",
			schema:  employeeGQLSchema,
			query:   `{ employeeByID(id:"bob") } `, // missing fields
			result:  failureResult,
			wantErr: false,
		},
		{
			note:    "empty schema is not ok",
			schema:  ``,
			query:   `{ employeeByID(id: "charlie") { id salary } }`,
			result:  failureResult,
			wantErr: false,
		},
		{
			note:   "empty query is ok",
			schema: employeeGQLSchema,
			query:  ``,
			result: ast.ArrayTerm(
				ast.BooleanTerm(true),
				ast.MustParseTerm("{}"),
				ast.NewTerm(employeeGQLSchemaASTObj),
			),
			wantErr: false,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result *ast.Term
			var err error
			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {

				err = builtinGraphQLParseAndVerify(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{ast.NewTerm(ast.String(tc.query)), ast.NewTerm(ast.String(tc.schema))},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				// Check each item in array result
				for i := range tc.result.Value.(*ast.Array).Len() {
					expected := tc.result.Value.(*ast.Array).Elem(i)
					actual := result.Value.(*ast.Array).Elem(i)
					if !expected.Equal(actual) {
						fmt.Fprintf(os.Stderr, "DEBUG: expected:\n%s\ngot:\n%s\n", expected.String(), actual.String())
						t.Errorf("Unexpected value at result[%d] (%s), expected %#v, got %#v", i, resultItemDescription[i], expected, actual)
						return
					}
				}
			}
			// Without the cache
			err = builtinGraphQLParseAndVerify(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{ast.NewTerm(ast.String(tc.query)), ast.NewTerm(ast.String(tc.schema))},
				func(term *ast.Term) error {
					result = term
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			// Check each item in array result
			for i := range tc.result.Value.(*ast.Array).Len() {
				expected := tc.result.Value.(*ast.Array).Elem(i)
				actual := result.Value.(*ast.Array).Elem(i)
				if !expected.Equal(actual) {
					t.Errorf("Unexpected value at result[%d] (%s), expected %#v, got %#v", i, resultItemDescription[i], expected, actual)
					return
				}
			}
		})
	}
}

func TestGraphQLIsValid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		query   *ast.Term
		schema  *ast.Term
		result  ast.Value
		wantErr bool
	}{
		{
			note:    "valid employee - query as string",
			query:   ast.NewTerm(ast.String(`{ employeeByID(id: "alice") { salary } }`)),
			schema:  ast.NewTerm(ast.String(employeeGQLSchema)),
			result:  ast.Boolean(true),
			wantErr: false,
		},
		{
			note:    "valid employee - query as object",
			query:   ast.NewTerm(ast.MustParseTerm(employeeGQLQueryAST).Value.(ast.Object)),
			schema:  ast.NewTerm(ast.String(employeeGQLSchema)),
			result:  ast.Boolean(true),
			wantErr: false,
		},
		{
			note:    "invalid schema",
			query:   ast.NewTerm(ast.String(`{ employeeByID(id: "alice") { salary } }`)),
			schema:  ast.NewTerm(ast.String(invalidEmployeeGQLSchema)),
			result:  ast.Boolean(false),
			wantErr: false,
		},
		{
			note:    "invalid query",
			query:   ast.NewTerm(ast.String(`{ employeeByID(id: "bob") }`)),
			schema:  ast.NewTerm(ast.String(employeeGQLSchema)),
			result:  ast.Boolean(false),
			wantErr: false,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result ast.Value
			var err error
			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {
				err = builtinGraphQLIsValid(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{tc.query, tc.schema},
					func(term *ast.Term) error {
						result = term.Value
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				if tc.result != result {
					t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
					return
				}
			}
			// Without the cache
			err = builtinGraphQLIsValid(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{tc.query, tc.schema},
				func(term *ast.Term) error {
					result = term.Value
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			if tc.result != result {
				t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
				return
			}
		})
	}
}

func TestGraphQLParseQuery(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		query   *ast.Term
		result  *ast.Term
		wantErr bool
	}{
		{
			note:    "valid employee - query as string",
			query:   ast.NewTerm(ast.String(`{ employeeByID(id: "alice") { salary } }`)),
			result:  ast.NewTerm(employeeGQLQueryASTObj),
			wantErr: false,
		},
		{
			note:    "invalid query",
			query:   ast.NewTerm(ast.String(`{ employeeByID("id: bob") }`)),
			result:  nil,
			wantErr: true,
		},
		{
			note:    "empty query is valid",
			query:   ast.NewTerm(ast.String(``)),
			result:  ast.MustParseTerm("{}"),
			wantErr: false,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result *ast.Term
			var err error
			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {
				err = builtinGraphQLParseQuery(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{tc.query},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				if !tc.result.Equal(result) {
					t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
					return
				}
			}
			// Without the cache
			err = builtinGraphQLParseQuery(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{tc.query},
				func(term *ast.Term) error {
					result = term
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			if !tc.result.Equal(result) {
				t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
				return
			}
		})
	}
}

func TestGraphQLParseSchema(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note    string
		schema  *ast.Term
		result  *ast.Term
		wantErr bool
	}{
		{
			note:    "valid schema as string",
			schema:  ast.NewTerm(ast.String(employeeGQLSchema)),
			result:  ast.NewTerm(employeeGQLSchemaASTObj),
			wantErr: false,
		},
		{
			note:    "invalid schema as string",
			schema:  ast.NewTerm(ast.String(invalidEmployeeGQLSchema)),
			result:  nil,
			wantErr: true,
		},
		{
			note:    "empty schema is valid",
			schema:  ast.NewTerm(ast.String(``)),
			result:  ast.MustParseTerm("{}"),
			wantErr: false,
		},
	}

	valueCache := cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					gqlCacheName: {
						MaxNumEntries: &[]int{10}[0],
					},
				},
			},
		})

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			var result *ast.Term
			var err error
			// Call function multiple times to hit the cache
			for i := 1; i <= 3; i++ {
				err = builtinGraphQLParseSchema(
					BuiltinContext{
						InterQueryBuiltinValueCache: valueCache,
					},
					[]*ast.Term{tc.schema},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if tc.wantErr && err == nil {
					t.Errorf("Unexpected return value, expected error, got nil")
					return
				}
				if !tc.wantErr && err != nil {
					t.Errorf("Unexpected return value, expected nil, got error: %s", err)
					return
				}
				if !tc.result.Equal(result) {
					t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
					return
				}
			}
			// Without the cache
			err = builtinGraphQLParseSchema(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{tc.schema},
				func(term *ast.Term) error {
					result = term
					return nil
				},
			)
			if tc.wantErr && err == nil {
				t.Errorf("Unexpected return value, expected error, got nil")
				return
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Unexpected return value, expected nil, got error: %s", err)
				return
			}
			if !tc.result.Equal(result) {
				t.Errorf("Unexpected result, expected %#v, got %#v", tc.result, result)
				return
			}
		})
	}
}

func TestGraphQLParseSchemaAlloc(t *testing.T) {
	cases := []struct {
		note     string
		schema   *ast.Term
		maxAlloc uint64
	}{
		{
			note:     "default schema",
			schema:   ast.NewTerm(ast.String(schemaWithExtraEmployeeTypes(0))),
			maxAlloc: 1 * 1024 * 1024,
		},
		// Uncomment when https://github.com/open-policy-agent/opa/pull/7509 is merged
		// {
		// 	note:     "default schema plus 100 additional types",
		// 	schema:   ast.NewTerm(ast.String(schemaWithExtraEmployeeTypes(100))),
		// 	maxAlloc: 10 * 1024 * 1024,
		// },
		// {
		// 	note:     "default schema plus 1,000 additional types",
		// 	schema:   ast.NewTerm(ast.String(schemaWithExtraEmployeeTypes(1000))),
		// 	maxAlloc: 50 * 1024 * 1024,
		// },
		// {
		// 	note:     "default schema plus 10,000 additional types",
		// 	schema:   ast.NewTerm(ast.String(schemaWithExtraEmployeeTypes(10000))),
		// 	maxAlloc: 100 * 1024 * 1024,
		// },
	}

	for _, tc := range cases {

		t.Run(tc.note, func(t *testing.T) {

			var startMemStats runtime.MemStats
			runtime.ReadMemStats(&startMemStats)

			_ = builtinGraphQLParseSchema(
				BuiltinContext{
					InterQueryBuiltinValueCache: nil,
				},
				[]*ast.Term{tc.schema},
				func(term *ast.Term) error {
					return nil
				},
			)

			var finishMemStats runtime.MemStats
			runtime.ReadMemStats(&finishMemStats)
			allocDifference := finishMemStats.Alloc - startMemStats.Alloc
			runtime.GC()

			if allocDifference > tc.maxAlloc {
				t.Errorf("Parsing schema '%s' expected alloc < %d, got %d", tc.note, tc.maxAlloc, allocDifference)
				return
			}
		})
	}
}

// Inflate GraphQL schema size with `count` extra types
func schemaWithExtraEmployeeTypes(count int) string {

	// build up `count` more types on basic schema
	var builder strings.Builder
	builder.WriteString(employeeGQLSchema)

	for i := range count {
		fmt.Fprintf(&builder, "\ntype Employee%d {\n    id: String!\n    salary: Int!\n}\n", i)
	}

	return builder.String()
}
