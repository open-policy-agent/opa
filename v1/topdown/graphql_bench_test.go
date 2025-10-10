// Copyright 2025 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
)

// The MaxNumEntries value for the named caches
const defaultCacheEntries = 10

// The number of types to add to the existing schema which already has one type definition
const extraTypes = 999

func BenchmarkGraphQLSchemaIsValid(b *testing.B) {
	benches := []struct {
		desc   string
		schema *ast.Term
		cache  cache.InterQueryValueCache
		result *ast.Term
	}{
		{
			desc:   "Trivial Schema - string",
			schema: ast.StringTerm(employeeGQLSchema),
			cache:  nil,
			result: ast.InternedTerm(true),
		},
		{
			desc:   "Trivial Schema with cache - string",
			schema: ast.StringTerm(employeeGQLSchema),
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			result: ast.InternedTerm(true),
		},
		{
			desc:   fmt.Sprintf("Schema w/ %d types - string", extraTypes+1),
			schema: ast.StringTerm(schemaWithExtraEmployeeTypes(extraTypes)),
			cache:  nil,
			result: ast.InternedTerm(true),
		},
		{
			desc:   fmt.Sprintf("Schema w/ %d types with cache - string", extraTypes+1),
			schema: ast.StringTerm(schemaWithExtraEmployeeTypes(extraTypes)),
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			result: ast.InternedTerm(true),
		},
		{
			desc:   "Trivial Schema - AST object",
			schema: ast.NewTerm(ast.MustParseTerm(employeeGQLSchemaAST).Value.(ast.Object)),
			cache:  nil,
			result: ast.InternedTerm(true),
		},
		{
			desc:   "Trivial Schema with cache - AST object",
			schema: ast.NewTerm(ast.MustParseTerm(employeeGQLSchemaAST).Value.(ast.Object)),
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			result: ast.InternedTerm(true),
		},
	}

	for _, bench := range benches {
		b.Run(bench.desc, func(b *testing.B) {
			for b.Loop() {
				var result *ast.Term
				err := builtinGraphQLSchemaIsValid(
					BuiltinContext{
						InterQueryBuiltinValueCache: bench.cache,
					},
					[]*ast.Term{bench.schema},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
				if !bench.result.Equal(result) {
					b.Fatalf("unexpected result: wanted: %#v got: %#v", bench.result, result)
				}
			}
		})
	}
}

func BenchmarkGraphQLParseSchema(b *testing.B) {
	benches := []struct {
		desc   string
		schema *ast.Term
		cache  cache.InterQueryValueCache
		result *ast.Term
	}{
		{
			desc:   "Trivial Schema - string",
			schema: ast.StringTerm(employeeGQLSchema),
			cache:  nil,
			result: ast.NewTerm(employeeGQLSchemaASTObj),
		},
		{
			desc:   "Trivial Schema with cache - string",
			schema: ast.StringTerm(employeeGQLSchema),
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			result: ast.NewTerm(employeeGQLSchemaASTObj),
		},
	}
	for _, bench := range benches {
		b.Run(bench.desc, func(b *testing.B) {
			for b.Loop() {
				var result *ast.Term
				err := builtinGraphQLParseSchema(
					BuiltinContext{
						InterQueryBuiltinValueCache: bench.cache,
					},
					[]*ast.Term{bench.schema},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
				if !bench.result.Equal(result) {
					b.Errorf("Unexpected result, expected %#v, got %#v", bench.result, result)
					return
				}
			}
		})
	}
}

func BenchmarkGraphQLParseQuery(b *testing.B) {
	benches := []struct {
		desc   string
		query  *ast.Term
		cache  cache.InterQueryValueCache
		result *ast.Term
	}{
		{
			desc:   "Trivial Query - string",
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			cache:  nil,
			result: ast.NewTerm(employeeGQLQueryASTObj),
		},
		{
			desc:   "Trivial Query with cache - string",
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			result: ast.NewTerm(employeeGQLQueryASTObj),
		},
	}
	for _, bench := range benches {
		b.Run(bench.desc, func(b *testing.B) {
			for b.Loop() {
				var result *ast.Term
				err := builtinGraphQLParseQuery(
					BuiltinContext{
						InterQueryBuiltinValueCache: bench.cache,
					},
					[]*ast.Term{bench.query},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
				if !bench.result.Equal(result) {
					b.Errorf("Unexpected result, expected %#v, got %#v", bench.result, result)
					return
				}
			}
		})
	}
}

func BenchmarkGraphQLIsValid(b *testing.B) {
	benches := []struct {
		desc   string
		schema *ast.Term
		cache  cache.InterQueryValueCache
		query  *ast.Term
		result *ast.Term
	}{
		{
			desc:   "Trivial Schema - string",
			cache:  nil,
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(employeeGQLSchema),
			result: ast.InternedTerm(true),
		},
		{
			desc:   "Trivial Schema with cache - string",
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(employeeGQLSchema),
			result: ast.InternedTerm(true),
		},
		{
			desc:   fmt.Sprintf("Schema w/ %d types - string", extraTypes+1),
			cache:  nil,
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(schemaWithExtraEmployeeTypes(extraTypes)),
			result: ast.InternedTerm(true),
		},
		{
			desc:   fmt.Sprintf("Schema w/ %d types with cache - string", extraTypes+1),
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(schemaWithExtraEmployeeTypes(extraTypes)),
			result: ast.InternedTerm(true),
		},
	}
	for _, bench := range benches {
		b.Run(bench.desc, func(b *testing.B) {
			for b.Loop() {
				var result *ast.Term
				err := builtinGraphQLIsValid(
					BuiltinContext{
						InterQueryBuiltinValueCache: bench.cache,
					},
					[]*ast.Term{bench.query, bench.schema},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
				if !bench.result.Equal(result) {
					b.Errorf("Unexpected result, expected %#v, got %#v", bench.result, result)
					return
				}
			}
		})
	}
}

func BenchmarkGraphQLParse(b *testing.B) {
	// Use this to map result item position to purpose for better error messages
	resultItemDescription := []string{"query_ast", "schema_ast"}

	benches := []struct {
		desc   string
		schema *ast.Term
		cache  cache.InterQueryValueCache
		query  *ast.Term
		result *ast.Term
	}{
		{
			desc:   "Trivial Schema - string",
			cache:  nil,
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(employeeGQLSchema),
			result: ast.ArrayTerm(
				ast.NewTerm(employeeGQLQueryASTObj),
				ast.NewTerm(employeeGQLSchemaASTObj),
			),
		},
		{
			desc:   "Trivial Schema with cache - string",
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(employeeGQLSchema),
			result: ast.ArrayTerm(
				ast.NewTerm(employeeGQLQueryASTObj),
				ast.NewTerm(employeeGQLSchemaASTObj),
			),
		},
	}
	for _, bench := range benches {
		b.Run(bench.desc, func(b *testing.B) {
			for b.Loop() {
				var result *ast.Term
				err := builtinGraphQLParse(
					BuiltinContext{
						InterQueryBuiltinValueCache: bench.cache,
					},
					[]*ast.Term{bench.query, bench.schema},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				if err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
				if !bench.result.Equal(result) {
					b.Errorf("Unexpected result, expected %#v, got %#v", bench.result, result)
					return
				}
				// Check each item in array result
				for i := range bench.result.Value.(*ast.Array).Len() {
					expected := bench.result.Value.(*ast.Array).Elem(i)
					actual := result.Value.(*ast.Array).Elem(i)
					if !expected.Equal(actual) {
						b.Errorf("Unexpected value at result[%d] (%s), expected %#v, got %#v", i, resultItemDescription[i], expected, actual)
						return
					}
				}
			}
		})
	}
}

func BenchmarkGraphQLParseAndVerify(b *testing.B) {
	// Use this to map result item position to purpose for better error messages
	resultItemDescription := []string{"is_valid", "query_ast", "schema_ast"}

	benches := []struct {
		desc   string
		schema *ast.Term
		cache  cache.InterQueryValueCache
		query  *ast.Term
		result *ast.Term
	}{
		{
			desc:   "Trivial Schema - string",
			cache:  nil,
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(employeeGQLSchema),
			result: ast.ArrayTerm(
				ast.InternedTerm(true),
				ast.NewTerm(employeeGQLQueryASTObj),
				ast.NewTerm(employeeGQLSchemaASTObj),
			),
		},
		{
			desc:   "Trivial Schema with cache - string",
			cache:  valueCacheFactory(gqlCacheName, defaultCacheEntries),
			query:  ast.StringTerm(`{ employeeByID(id: "alice") { salary } }`),
			schema: ast.StringTerm(employeeGQLSchema),
			result: ast.ArrayTerm(
				ast.InternedTerm(true),
				ast.NewTerm(employeeGQLQueryASTObj),
				ast.NewTerm(employeeGQLSchemaASTObj),
			),
		},
	}
	for _, bench := range benches {
		b.Run(bench.desc, func(b *testing.B) {
			for b.Loop() {
				var result *ast.Term
				b.StartTimer()
				err := builtinGraphQLParseAndVerify(
					BuiltinContext{
						InterQueryBuiltinValueCache: bench.cache,
					},
					[]*ast.Term{bench.query, bench.schema},
					func(term *ast.Term) error {
						result = term
						return nil
					},
				)
				b.StopTimer()
				if err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
				if !bench.result.Equal(result) {
					b.Errorf("Unexpected result, expected %#v, got %#v", bench.result, result)
					return
				}
				// Check each item in array result
				for i := range bench.result.Value.(*ast.Array).Len() {
					expected := bench.result.Value.(*ast.Array).Elem(i)
					actual := result.Value.(*ast.Array).Elem(i)
					if !expected.Equal(actual) {
						b.Errorf("Unexpected value at result[%d] (%s), expected %#v, got %#v", i, resultItemDescription[i], expected, actual)
						return
					}
				}
			}
		})
	}
}

func valueCacheFactory(name string, maxEntries int) cache.InterQueryValueCache {
	return cache.NewInterQueryValueCache(
		context.Background(),
		&cache.Config{
			InterQueryBuiltinValueCache: cache.InterQueryBuiltinValueCacheConfig{
				NamedCacheConfigs: map[string]*cache.NamedValueCacheConfig{
					name: {
						MaxNumEntries: &[]int{maxEntries}[0],
					},
				},
			},
		})
}
