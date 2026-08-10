package ast

import (
	"fmt"
	"strconv"
	"testing"
)

// Cost of recording generated locals for ref type errors, at 100 modules:
// 9452905 ns/op   6487160 B/op   162824 allocs/op // not recorded
// 9944902 ns/op   6751259 B/op   168040 allocs/op // every subject copied
// 9578020 ns/op   6580108 B/op   163241 allocs/op // copied only where a later stage can rewrite it
func BenchmarkCompileModules(b *testing.B) {
	// The choice of module set is somewhat arbitrary. These rules are
	// representative of the ones that exercise the term-rewriting stages:
	// composite ref subjects, dynamic ref operands and comprehensions all get
	// hoisted into generated locals, so per-hoist work in those stages shows up
	// here. BenchmarkRewriteDynamics covers one of them in isolation, but reuses
	// the same bodies across iterations, so they are already rewritten after the
	// first pass.
	sizes := []int{1, 10, 100}

	for _, size := range sizes {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			base := make(map[string]*Module, size)
			for i := range size {
				base[fmt.Sprintf("mod%d.rego", i)] = MustParseModule(fmt.Sprintf(`package bench.p%d

allow if {
	some x
	input.users[x].roles[_] == "admin"
	data.perms[input.tenant][x]
	count([y | y := input.items[_]; y.n > 0]) > 2
	input.a.b.c[input.i].d == data.z.w[input.j]
}

deny contains msg if {
	msg := input.msgs[input.i].text
	[1, 2][input.k]
}
`, i))
			}

			for b.Loop() {
				// Compile rewrites modules in place, so every iteration needs
				// its own copies. Copying is not what we're measuring.
				b.StopTimer()
				modules := make(map[string]*Module, len(base))
				for name, module := range base {
					modules[name] = module.Copy()
				}
				b.StartTimer()

				c := NewCompiler()
				if c.Compile(modules); c.Failed() {
					b.Fatal(c.Errors)
				}
			}
		})
	}
}

func BenchmarkRewriteDynamics(b *testing.B) {
	// The choice of query to use is somewhat arbitrary. This query is
	// representative of the ones that result from partial evaluation on IAM
	// data models (e.g., a triple glob match on subject/action/resource.)
	body := MustParseBody(`
		glob.match("a:*", [":"], input.abcdef.x12345);
		glob.match("a:*", [":"], input.abcdef.y12345);
		glob.match("a:*", [":"], input.abcdef.z12345)
	`)
	sizes := []int{1, 10, 100, 1000, 10000, 100000}
	queries := makeQueriesForRewriteDynamicsBenchmark(sizes, body)

	for i := range sizes {
		b.Run(strconv.Itoa(sizes[i]), func(b *testing.B) {
			factory := newEqualityFactory(newLocalVarGenerator("q", nil))
			b.ResetTimer()
			for b.Loop() {
				for _, body := range queries[i] {
					rewriteDynamics(factory, body)
				}
			}
		})
	}
}

// 32.38 ns/op	      31 B/op	       1 allocs/op // String concatenation
// 18.77 ns/op	      23 B/op	       1 allocs/op // []byte appends
func BenchmarkGenerateLocalVar(b *testing.B) {
	g := newLocalVarGenerator("q", nil)

	for b.Loop() {
		g.Generate()
	}
}

func makeQueriesForRewriteDynamicsBenchmark(sizes []int, body Body) [][]Body {
	queries := make([][]Body, len(sizes))

	for i := range queries {
		queries[i] = make([]Body, sizes[i])
		for j := range sizes[i] {
			queries[i][j] = body.Copy()
		}
	}

	return queries
}
