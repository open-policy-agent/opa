// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Benchmarks behind the decision to keep stack traces opt-in. Each case runs
// both modes so the pair can be compared directly with benchstat. The results
// recorded below were taken with -benchmem.
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
// and a resolved frame per query on it. Depth is swept as well as error count
// because both are proportional to it.
//
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=2/stacktraces=false-16    	   32085	     37078 ns/op	   48085 B/op	    1102 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=2/stacktraces=true-16     	   20283	     59257 ns/op	   71535 B/op	    1606 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=8/stacktraces=false-16    	   29112	     41292 ns/op	   53197 B/op	    1231 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=8/stacktraces=true-16     	   14090	     84914 ns/op	   95956 B/op	    1735 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=16/stacktraces=false-16   	   24403	     49264 ns/op	   60206 B/op	    1401 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=100/depth=16/stacktraces=true-16    	    9816	    119575 ns/op	  135159 B/op	    1906 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=2/stacktraces=false-16   	    3408	    358635 ns/op	  444537 B/op	   10670 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=2/stacktraces=true-16    	    2010	    598633 ns/op	  709073 B/op	   15676 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=8/stacktraces=false-16   	    3344	    354682 ns/op	  449724 B/op	   10799 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=8/stacktraces=true-16    	    1479	    811721 ns/op	  907602 B/op	   15808 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=16/stacktraces=false-16  	    3207	    365128 ns/op	  456894 B/op	   10970 allocs/op
// BenchmarkStackTraceCollectedBuiltinErrors/errors=1000/depth=16/stacktraces=true-16   	    1087	   1105354 ns/op	 1235738 B/op	   15982 allocs/op
//
// Ranges from +60% time / +49% bytes to +203% / +170%, all of it on errors the
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
// BenchmarkStackTraceEarlyExit/stacktraces=false-16    	     229	   5183392 ns/op	 2082660 B/op	   96411 allocs/op
// BenchmarkStackTraceEarlyExit/stacktraces=true-16     	     230	   5170409 ns/op	 2082777 B/op	   96412 allocs/op
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

	for b.Loop() {
		builtinErrors = builtinErrors[:0]

		if _, err := q.Run(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
