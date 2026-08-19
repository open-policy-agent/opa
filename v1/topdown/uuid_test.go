// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func TestUUIDRFC4122SeedingAndCaching(t *testing.T) {
	t.Parallel()

	query := `uuid.rfc4122("x",x); uuid.rfc4122("y", y); uuid.rfc4122("x",x2)`

	q := NewQuery(ast.MustParseBody(query)).WithSeed(rand.New(rand.NewSource(0))).WithCompiler(ast.NewCompiler())

	qrs, err := q.Run(t.Context())
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected exactly one result but got:", qrs)
	}

	exp := ast.MustParseTerm(`
		{
			{
				x: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				x2: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				y: "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
			}
		}
	`)

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}
}

type fakeSeedErrorReader struct{}

func (fakeSeedErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("xxx")
}

func TestUUIDRFC4122SeedError(t *testing.T) {
	t.Parallel()

	query := `uuid.rfc4122("x",x)`

	q := NewQuery(ast.MustParseBody(query)).WithSeed(fakeSeedErrorReader{}).WithCompiler(ast.NewCompiler()).WithStrictBuiltinErrors(true)

	_, err := q.Run(t.Context())

	if topdownErr, ok := err.(*Error); !ok || topdownErr.Code != BuiltinErr {
		t.Fatal("unexpected error (or lack of error):", err)
	}
}

func TestUUIDRFC4122SavingDuringPartialEval(t *testing.T) {
	t.Parallel()

	query := `foo = "x"; uuid.rfc4122(foo,x)`
	c := ast.NewCompiler().
		WithCapabilities(&ast.Capabilities{Builtins: []*ast.Builtin{ast.UUIDRFC4122}})
	// Must compile to initialize type environment after WithCapabilities
	c.Compile(nil)

	q := NewQuery(ast.MustParseBody(query)).WithSeed(rand.New(rand.NewSource(0))).WithCompiler(c)

	queries, modules, err := q.PartialRun(t.Context())
	if err != nil {
		t.Fatal(err)
	} else if len(modules) > 0 {
		t.Fatal("expected no support")
	}

	exp := ast.MustParseBody(`uuid.rfc4122("x", x); foo = "x"`)

	if len(queries) != 1 || !queries[0].Equal(exp) {
		t.Fatalf("expected %v but got: %v", exp, queries)
	}
}

func queryResultSetToTerm(qrs QueryResultSet) *ast.Term {
	s := ast.NewSet()
	for i := range qrs {
		bindings := ast.NewObject()
		for k := range qrs[i] {
			bindings.Insert(ast.NewTerm(k), qrs[i][k])
		}
		s.Add(ast.NewTerm(bindings))
	}
	return ast.NewTerm(s)
}

// BenchmarkUUIDRFC4122/uncached-16         	 1581787	       748.1 ns/op	    1953 B/op	      32 allocs/op
// BenchmarkUUIDRFC4122/cached-16           	 1271378	       942.0 ns/op	    2354 B/op	      37 allocs/op
// BenchmarkUUIDRFC4122/different-16        	 1000000	      1046 ns/op	    2466 B/op	      41 allocs/op
func BenchmarkUUIDRFC4122Query(b *testing.B) {
	names := []string{"uncached", "cached", "different"}
	queries := []ast.Body{
		ast.MustParseBody(`uuid.rfc4122("x")`),
		ast.MustParseBody(`uuid.rfc4122("x"); uuid.rfc4122("x")`),
		ast.MustParseBody(`uuid.rfc4122("x"); uuid.rfc4122("y")`),
	}

	for i, name := range names {
		b.Run(name, func(b *testing.B) {
			q := NewQuery(queries[i]).WithSeed(rand.New(rand.NewSource(12345)))
			for b.Loop() {
				if _, err := q.Run(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// uncached-16         	 7452488       155.2 ns/op     136 B/op       6 allocs/op // Using String() as key
// uncached-16         	10524730       120.1 ns/op     112 B/op       4 allocs/op // Using Hash() as key
// cached-16           	77896573        15.4 ns/op       0 B/op       0 allocs/op
func BenchmarkUUIDRFC4122(b *testing.B) {
	// 101.0 ns/op    112 B/op    4 allocs/op (1 UUID string + 1 Value boxing + 1 *Term + 1 cache.Put)
	b.Run("uncached", func(b *testing.B) {
		bcx := BuiltinContext{Cache: make(builtins.Cache), Seed: rand.New(rand.NewSource(12345))}
		ops := []*ast.Term{ast.InternedTerm("x")}
		key := uuidCachingKey(ops[0].Value.Hash())

		for b.Loop() {
			if err := builtinUUIDRFC4122(bcx, ops, noOpIter); err != nil {
				b.Fatal(err)
			}
			delete(bcx.Cache, key)
		}
	})

	// 15.04 ns/op	       0 B/op	       0 allocs/op
	b.Run("cached", func(b *testing.B) {
		bcx := BuiltinContext{Cache: make(builtins.Cache), Seed: rand.New(rand.NewSource(12345))}
		ops := []*ast.Term{ast.InternedTerm("x")}
		for b.Loop() {
			if err := builtinUUIDRFC4122(bcx, ops, noOpIter); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkParseUUIDRFC4122/version=1-16         	 1159633	      1032 ns/op	    1489 B/op	      28 allocs/op
// BenchmarkParseUUIDRFC4122/version=2-16         	 1000000	      1199 ns/op	    1761 B/op	      34 allocs/op
func BenchmarkParseUUIDRFC4122(b *testing.B) {
	b.Run("version=1", func(b *testing.B) {
		operands := []*ast.Term{ast.StringTerm("c2fc67c2-47f2-11ee-b67a-9f3619c7493f")}
		for b.Loop() {
			if err := builtinUUIDParse(BuiltinContext{}, operands, noOpIter); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("version=2", func(b *testing.B) {
		operands := []*ast.Term{ast.StringTerm("000003e8-48b9-21ee-b200-325096b39f47")}
		for b.Loop() {
			if err := builtinUUIDParse(BuiltinContext{}, operands, noOpIter); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func noOpIter(term *ast.Term) error {
	return nil
}
