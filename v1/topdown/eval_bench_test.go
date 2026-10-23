package topdown_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func BenchmarkBiunifyArraysDestructure(b *testing.B) {
	pairs := make([]any, 100)
	for i := range pairs {
		role := "user"
		if i%10 == 0 {
			role = "admin"
		}
		pairs[i] = []any{fmt.Sprintf("u%d", i), role}
	}

	q := topdown.NewQuery(ast.MustParseBody(`[x, "admin"] = input.pairs[_]`)).
		WithInput(ast.NewTerm(ast.MustInterfaceToValue(map[string]any{"pairs": pairs})))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

func BenchmarkBiunifyArrays(b *testing.B) {
	q := topdown.NewQuery(ast.MustParseBody("[1,x,3] = [y,5,6]"))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

func BenchmarkEvalNot(b *testing.B) {
	q := topdown.NewQuery(ast.MustParseBody("x = 2; not x = 1; not x = 3; not x = 4"))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}

// Checking a known value against a set with `some x in s` should cost the same as
// `s[x]`, regardless of the size of the set. Both are a membership lookup.
// See https://github.com/open-policy-agent/opa/issues/6740.
//
// BenchmarkEvalSetMembershipByIteration/some_in/10-16         	  466572	      2549 ns/op	    4203 B/op	      85 allocs/op
// BenchmarkEvalSetMembershipByIteration/bracket/10-16         	  489201	      2430 ns/op	    4191 B/op	      87 allocs/op
// BenchmarkEvalSetMembershipByIteration/some_in/100-16        	  474472	      2518 ns/op	    4203 B/op	      85 allocs/op
// BenchmarkEvalSetMembershipByIteration/bracket/100-16        	  484470	      2431 ns/op	    4191 B/op	      87 allocs/op
// BenchmarkEvalSetMembershipByIteration/some_in/1000-16       	  459765	      2605 ns/op	    4204 B/op	      85 allocs/op
// BenchmarkEvalSetMembershipByIteration/bracket/1000-16       	  474529	      2523 ns/op	    4192 B/op	      87 allocs/op
func BenchmarkEvalSetMembershipByIteration(b *testing.B) {
	for _, n := range []int{10, 100, 1000} {
		for _, tc := range []struct{ note, body string }{
			{"some_in", "some x in s"},
			{"bracket", "s[x]"},
		} {
			b.Run(fmt.Sprintf("%s/%d", tc.note, n), func(b *testing.B) {
				elems := make([]string, n)
				for i := range elems {
					elems[i] = strconv.Itoa(i + 1)
				}

				compiler := ast.MustCompileModules(map[string]string{"test.rego": fmt.Sprintf(`package p

s := {%s}

member contains x if {
	%s
}
`, strings.Join(elems, ", "), tc.body)})

				q := topdown.NewQuery(ast.MustParseBody(fmt.Sprintf("data.p.member[%d]", n))).WithCompiler(compiler)

				ctx := b.Context()

				for b.Loop() {
					rs, err := q.Run(ctx)
					if err != nil {
						b.Fatalf("Query failed: %v", err)
					}
					if len(rs) != 1 {
						b.Fatalf("expected 1 result, got %d", len(rs))
					}
				}
			})
		}
	}
}

func BenchmarkEvalBuiltinContext(b *testing.B) {
	q := topdown.NewQuery(ast.MustParseBody(`regex.match("^a", "abc"); concat(",", ["a", "b"], x); glob.match("a*", [], "abc")`))

	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatalf("Query failed: %v", err)
		}
	}
}
