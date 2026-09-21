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
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=2/stacktraces=false-16    	   28700	     40480 ns/op	   72940 B/op	    1117 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=2/stacktraces=true-16     	   26426	     45996 ns/op	   87421 B/op	    1317 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=8/stacktraces=false-16    	   26203	     45624 ns/op	   78060 B/op	    1246 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=8/stacktraces=true-16     	   22844	     52425 ns/op	  102182 B/op	    1446 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=16/stacktraces=false-16   	   22716	     52964 ns/op	   85074 B/op	    1417 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=16/stacktraces=true-16    	   18742	     64023 ns/op	  122092 B/op	    1617 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=2/stacktraces=false-16   	    3348	    365319 ns/op	  625758 B/op	   10689 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=2/stacktraces=true-16    	    2844	    422193 ns/op	  770931 B/op	   12691 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=8/stacktraces=false-16   	    3310	    368131 ns/op	  630848 B/op	   10818 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=8/stacktraces=true-16    	    2722	    441942 ns/op	  872065 B/op	   12820 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=16/stacktraces=false-16  	    3162	    373970 ns/op	  637913 B/op	   10989 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=16/stacktraces=true-16   	    2330	    519403 ns/op	 1009386 B/op	   12996 allocs/op
//
// Ranges from +14% time / +20% bytes to +39% / +58%, all of it on errors the
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
// BenchmarkStackTraceEarlyExit/stacktraces=false-16    	     229	   5212186 ns/op	 2083281 B/op	   96419 allocs/op
// BenchmarkStackTraceEarlyExit/stacktraces=true-16     	     232	   5169479 ns/op	 2083059 B/op	   96418 allocs/op
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

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
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

		if _, err := q.Run(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
