package topdown

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
)

func BenchmarkBulkStartsWithNaive(b *testing.B) {
	data := generateBulkStartsWithInput()
	ctx := context.Background()
	store := inmem.NewFromObject(data)

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `
package test

result if {
  startswith(data.strings[_], data.prefixes[_])
}
`,
	})

	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody("data.test.result"))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for range b.N {
		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			_, err := NewQuery(query).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				Run(ctx)

			return err
		})

		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBulkStartsWithOptimized(b *testing.B) {
	data := generateBulkStartsWithInput()
	ctx := context.Background()
	store := inmem.NewFromObject(data)

	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `
package test

result if {
  strings.any_prefix_match(data.strings, data.prefixes)
}
`,
	})

	query, err := compiler.QueryCompiler().Compile(ast.MustParseBody("data.test.result"))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()

	for range b.N {
		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			_, err := NewQuery(query).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				Run(ctx)

			return err
		})

		if err != nil {
			b.Fatal(err)
		}
	}
}

func generateBulkStartsWithInput() map[string]any {
	strs := make([]string, 0, 1000)
	for i := range strs {
		strs = append(strs, fmt.Sprintf("aabbccddeeffgghhiijjkkllmmnnoopp_%d", i))
	}
	prefixes := make([]string, 0, 100)
	for i := range prefixes {
		prefixes = append(prefixes, fmt.Sprintf("aabbccddeeffgghhiijjkkllmmnnoorr_%d", i))
	}
	return map[string]any{
		"strings":  strs,
		"prefixes": prefixes,
	}
}

func BenchmarkSplit(b *testing.B) {
	bctx := BuiltinContext{}
	operands := []*ast.Term{
		ast.StringTerm("a.b.c.d.e"),
		ast.StringTerm("."),
	}

	exp := eqIter(ast.ArrayTerm(
		ast.StringTerm("a"),
		ast.StringTerm("b"),
		ast.StringTerm("c"),
		ast.StringTerm("d"),
		ast.StringTerm("e"),
	))

	b.ResetTimer()
	b.ReportAllocs()
	for range b.N {
		if err := builtinSplit(bctx, operands, exp); err != nil {
			b.Fatal(err)
		}
	}
}

// Now down to 2 allocations per iteration for ASCII strings, more for non-ASCII as that requires
// string/rune conversion. 2 allocations unavoidable - 1 for the new Term and 1 for its Value.
func BenchmarkSubstring(b *testing.B) {
	operands := []*ast.Term{
		// insert any non-asci character to see the difference of that optimization
		ast.StringTerm("The quick brown fox jumps over the lazy dog"),
		ast.InternedTerm(6),
		ast.InternedTerm(10),
	}

	iter := eqIter(ast.StringTerm("ick brown "))

	b.ResetTimer()

	for range b.N {
		if err := builtinSubstring(BuiltinContext{}, operands, iter); err != nil {
			b.Fatal(err)
		}
	}
}

// Unicode
// BenchmarkIndexOf-10    	10498884	       114.0 ns/op	     176 B/op	       1 allocs/op
//
// ASCII
// BenchmarkIndexOf-10    	36625468	        31.57 ns/op	       0 B/op	       0 allocs/op
func BenchmarkIndexOf(b *testing.B) {
	operands := []*ast.Term{
		ast.StringTerm("The quick brown fox jumps over the lazy dog"),
		ast.StringTerm("dog"),
	}

	b.ResetTimer()

	for range b.N {
		if err := builtinIndexOf(BuiltinContext{}, operands, eqIter(ast.InternedTerm(40))); err != nil {
			b.Fatal(err)
		}
	}
}

func eqIter(a *ast.Term) func(*ast.Term) error {
	return func(b *ast.Term) error {
		if !a.Equal(b) {
			return fmt.Errorf("expected %v equal to %v", a, b)
		}
		return nil
	}
}

// 0 allocs for numbers between 0 and 100 and base 10, 3 allocs for anything else.
func BenchmarkFormatInt(b *testing.B) {
	operands := []*ast.Term{
		ast.InternedTerm(99),
		ast.InternedTerm(10),
	}
	bctx := BuiltinContext{}
	want := eqIter(ast.StringTerm("99"))

	b.ResetTimer()

	for range b.N {
		if err := builtinFormatInt(bctx, operands, want); err != nil {
			b.Fatal(err)
		}
	}
}

// 0 allocs for numbers between 0 and 100, 3 allocs for anything else.
func BenchmarkSprintfSingleInteger(b *testing.B) {
	operands := []*ast.Term{
		ast.StringTerm("%d"),
		ast.ArrayTerm(
			ast.InternedTerm(99),
		),
	}
	bctx := BuiltinContext{}
	want := eqIter(ast.StringTerm("99"))

	b.ResetTimer()

	for range b.N {
		if err := builtinSprintf(bctx, operands, want); err != nil {
			b.Fatal(err)
		}
	}
}

// This benchmark is not so much about trimming space, but the optimization of returning
// the operand as provided for string operations that don't change the string provided as
// input, like when trimming space around a string that doesn't have any, or replacing a
// substring that doesn't exist. Since string operations are often called in batch, this
// win can be significant.
//
// BenchmarkTrimSpace/trimmable-10        14425539        85.10 ns/op      64 B/op       2 allocs/op
// BenchmarkTrimSpace/not_trimmable-10    87051141        14.47 ns/op       0 B/op       0 allocs/op
func BenchmarkTrimSpace(b *testing.B) {
	bctx := BuiltinContext{}

	cases := []struct {
		input    string
		operands []*ast.Term
	}{
		{
			input:    "trimmable",
			operands: []*ast.Term{ast.StringTerm("  The quick brown fox jumps over the lazy dog  ")},
		},
		{
			input:    "not trimmable",
			operands: []*ast.Term{ast.StringTerm("The quick brown fox jumps over the lazy dog")},
		},
	}

	iter := eqIter(ast.StringTerm("The quick brown fox jumps over the lazy dog"))

	for _, c := range cases {
		b.Run(c.input, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				if err := builtinTrimSpace(bctx, c.operands, iter); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark to demonstrate the performance difference when calling lower with a string
// that is already lowercase vs. one that is not. In the former case, the provided operand
// is returned as-is, while in the latter case a new string is allocated and returned.
// While this tests the 'lower' builtin, the same optimization applies to 'upper'.
//
// BenchmarkLower/not_lowercase-10         	 5960936	       198.6 ns/op	      88 B/op	       3 allocs/op
// BenchmarkLower/lowercase-10             	20954871	        57.36 ns/op	       0 B/op	       0 allocs/op
func BenchmarkLower(b *testing.B) {
	bctx := BuiltinContext{}
	lower := ast.StringTerm("the quick brown fox jumps over the lazy dog")
	cases := []struct {
		name     string
		operands []*ast.Term
	}{
		{
			name:     "not lowercase",
			operands: []*ast.Term{ast.StringTerm("The Quick Brown Fox Jumps Over The Lazy Dog")},
		},
		{
			name:     "lowercase",
			operands: []*ast.Term{lower},
		},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ResetTimer()
			for range b.N {
				if err := builtinLower(bctx, c.operands, eqIter(lower)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConcat(b *testing.B) {
	bctx := BuiltinContext{}
	tests := []struct {
		name     string
		operands []*ast.Term
		expected *ast.Term
	}{
		{
			name:     "0 elements '.' sep",
			operands: []*ast.Term{ast.InternedTerm("."), ast.InternedEmptyArray},
			expected: ast.InternedEmptyString,
		},
		{
			name:     "1 element '.' sep",
			operands: []*ast.Term{ast.InternedTerm("."), ast.ArrayTerm(ast.InternedTerm("foobar"))},
			expected: ast.InternedTerm("foobar"),
		},
		{
			name:     "100 elements ',' sep",
			operands: []*ast.Term{ast.InternedTerm(","), repeatTerm(ast.InternedTerm("foobar"), 100)},
			expected: ast.StringTerm(strings.Repeat("foobar,", 99) + "foobar"),
		},
		{
			name:     "100 elements ', ' sep",
			operands: []*ast.Term{ast.InternedTerm(", "), repeatTerm(ast.InternedTerm("foobar"), 100)},
			expected: ast.StringTerm(strings.Repeat("foobar, ", 99) + "foobar"),
		},
		{
			name:     "100 elements blank sep",
			operands: []*ast.Term{ast.InternedEmptyString, repeatTerm(ast.InternedTerm("foobar"), 100)},
			expected: ast.StringTerm(strings.Repeat("foobar", 100)),
		},
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			for range b.N {
				if err := builtinConcat(bctx, test.operands, eqIter(test.expected)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConcatVsSprintfSimple(b *testing.B) {
	bctx := BuiltinContext{}

	foo := ast.InternedTerm("foo")
	bar := ast.InternedTerm("bar")
	expected := ast.InternedTerm("foobar")

	b.Run("concat foobar", func(b *testing.B) {
		operands := []*ast.Term{ast.InternedEmptyString, ast.ArrayTerm(foo, bar)}

		for range b.N {
			if err := builtinConcat(bctx, operands, eqIter(expected)); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.ResetTimer()

	b.Run("sprintf foobar", func(b *testing.B) {
		operands := []*ast.Term{ast.InternedTerm("%s%s"), ast.ArrayTerm(foo, bar)}

		for range b.N {
			if err := builtinSprintf(bctx, operands, eqIter(expected)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func repeatTerm(t *ast.Term, n int) *ast.Term {
	terms := make([]*ast.Term, 0, n)
	for range n {
		terms = append(terms, t)
	}
	return ast.ArrayTerm(terms...)
}

func BenchmarkSplitLenVsStringsCount(b *testing.B) {
	str := "a.b.c.d.e"

	b.Run("split len", func(b *testing.B) {
		for range b.N {
			if len(strings.Split(str, ".")) != 5 {
				b.Fatal("expected 5 elements")
			}
		}
	})

	b.Run("strings count", func(b *testing.B) {
		for range b.N {
			if strings.Count(str, ".")+1 != 5 {
				b.Fatal("expected 5 elements")
			}
		}
	})
}
