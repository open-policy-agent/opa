// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Benchmarks behind the decision to keep stack traces opt-in. Each case runs
// both modes so the pair can be compared directly with benchstat.
//
// Both supply a builtin error list. Without one the collected errors have no
// consumer, evalBuiltin skips annotating them entirely, and the benchmark would
// measure that skip rather than the feature.

package topdown

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

// BenchmarkStackTraceCollectedBuiltinErrors is the worst case for the feature.
// A failing built-in leaves the expression undefined and evaluation carries on,
// so messy data raises one error per row, and each one costs a parent-chain walk
// and two allocations. Depth is swept as well as error count because the walk
// is proportional to it.
//
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=2/stacktraces=false-16    	   31813	     36799 ns/op	   48088 B/op	    1102 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=2/stacktraces=true-16     	   27426	     43181 ns/op	   62561 B/op	    1302 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=8/stacktraces=false-16    	   28749	     41752 ns/op	   53199 B/op	    1231 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=8/stacktraces=true-16     	   24178	     49639 ns/op	   77334 B/op	    1431 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=16/stacktraces=false-16   	   24164	     49782 ns/op	   60208 B/op	    1401 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=16/stacktraces=true-16    	   19729	     60518 ns/op	   97216 B/op	    1602 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=2/stacktraces=false-16   	    3289	    362875 ns/op	  444556 B/op	   10670 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=2/stacktraces=true-16    	    2842	    403191 ns/op	  589390 B/op	   12671 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=8/stacktraces=false-16   	    3376	    359189 ns/op	  449718 B/op	   10799 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=8/stacktraces=true-16    	    2708	    443949 ns/op	  691559 B/op	   12801 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=16/stacktraces=false-16  	    3265	    367829 ns/op	  456874 B/op	   10970 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=16/stacktraces=true-16   	    2414	    503342 ns/op	  827582 B/op	   12975 allocs/op
//
// Ranges from +17% time / +30% bytes to +37% / +81%, all of it on errors the
// caller asked to collect. That is why capture stays behind
// Query.WithStackTraces.
func BenchmarkStackTraceCollectedBuiltinErrors(b *testing.B) {
	for _, errs := range []int{100, 1000} {
		for _, depth := range []int{2, 8, 16} {
			for _, on := range []bool{false, true} {
				name := fmt.Sprintf("errors=%d/depth=%d/stacktraces=%t", errs, depth, on)
				b.Run(name, func(b *testing.B) {
					runStackTraceBenchmark(b, builtinErrorModule(depth), errs, on)
				})
			}
		}
	}
}

// BenchmarkStackTraceEarlyExit is the control. Early exit propagates as an
// error through every enclosing query, making it the hottest error path by far,
// but it never carries an *Error, so the guard in withStackTrace should be the
// only cost.
//
// BenchmarkStackTraceEarlyExit/stacktraces=false-16    	     225	   5292645 ns/op	 2082595 B/op	   96411 allocs/op
// BenchmarkStackTraceEarlyExit/stacktraces=true-16     	     224	   5359468 ns/op	 2082750 B/op	   96412 allocs/op
func BenchmarkStackTraceEarlyExit(b *testing.B) {
	for _, on := range []bool{false, true} {
		b.Run("stacktraces="+strconv.FormatBool(on), func(b *testing.B) {
			runStackTraceBenchmark(b, earlyExitModule(), 300, on)
		})
	}
}

func builtinErrorModule(depth int) string {
	s := strings.Builder{}
	s.WriteString("package bench\n\nlevel0 contains y if {\n\tsome x in input.xs\n\ty := to_number(x)\n}\n")
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&s, "\nlevel%d contains y if {\n\tlevel%d[y]\n}\n", i, i-1)
	}
	fmt.Fprintf(&s, "\ntop contains y if {\n\tlevel%d[y]\n}\n", depth-1)
	return s.String()
}

func earlyExitModule() string {
	return `package bench

top contains y if {
	some x in input.xs
	f(x)
	y := x
}

f(x) if {
	some z in input.xs
	z == x
}
`
}

func runStackTraceBenchmark(b *testing.B, module string, n int, stackTraces bool) {
	b.Helper()

	m, err := ast.ParseModuleWithOpts("bench.rego", module, ast.ParserOptions{})
	if err != nil {
		b.Fatal(err)
	}
	compiler := ast.NewCompiler()
	if compiler.Compile(map[string]*ast.Module{"bench.rego": m}); compiler.Failed() {
		b.Fatal(compiler.Errors)
	}

	xs := make([]*ast.Term, n)
	for i := range xs {
		xs[i] = ast.StringTerm("v" + strconv.Itoa(i))
	}
	input := ast.NewTerm(ast.NewObject([2]*ast.Term{ast.StringTerm("xs"), ast.NewTerm(ast.NewArray(xs...))}))

	ctx := b.Context()
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	query := ast.MustParseBody("data.bench.top")

	// A builtin error list is what makes the collected errors reportable;
	// without one they are dropped and never annotated at all.
	var builtinErrors []Error

	q := NewQuery(query).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithInput(input).
		WithBuiltinErrorList(&builtinErrors).
		WithStackTraces(stackTraces)

	b.ReportAllocs()

	for b.Loop() {
		builtinErrors = builtinErrors[:0]

		if _, err := q.Run(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
