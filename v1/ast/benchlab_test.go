// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.
//
// Every case costs the same fixed slice of wall clock, whatever it measures, so
// size sweeps are cut to the sizes that say something the others do not. Where
// a family covers one code path at several sizes, only the ends are kept.

package ast

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// --- compilation ---------------------------------------------------------

// The whole Compiler.Compile pipeline, which nothing else here reaches. The
// modules are shaped to hit the term-rewriting stages: composite ref subjects,
// dynamic ref operands and comprehensions all get hoisted into generated
// locals. 1 is the per-compile fixed cost, 100 the per-module work.
func BenchmarkBenchlabCompileModules(b *testing.B) {
	for _, size := range []int{1, 100} {
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

// Safety-error reporting, which keeps checking rules past the error limit and
// so reports one error per rule. Guards the per-module index of shared source
// lines staying off that path; looking lines up per error is quadratic.
func BenchmarkBenchlabCompileUnsafeRules(b *testing.B) {
	const size = 5000
	b.Run(strconv.Itoa(size), func(b *testing.B) {
		var sb strings.Builder
		sb.WriteString("package bench\n\n")
		for i := range size {
			// x is never bound, so every rule fails the safety check.
			fmt.Fprintf(&sb, "p%d if {\n\tx == %d\n}\n\n", i, i)
		}
		base := MustParseModule(sb.String())

		for b.Loop() {
			b.StopTimer()
			module := base.Copy()
			b.StartTimer()

			c := NewCompiler()
			if c.Compile(map[string]*Module{"mod.rego": module}); !c.Failed() {
				b.Fatal("expected safety errors")
			}
		}
	})
}

// The rewrite stage in isolation. Its sweep is linear, so one point stands in
// for the six.
func BenchmarkBenchlabRewriteDynamics(b *testing.B) {
	const size = 10000
	body := MustParseBody(`
		glob.match("a:*", [":"], input.abcdef.x12345);
		glob.match("a:*", [":"], input.abcdef.y12345);
		glob.match("a:*", [":"], input.abcdef.z12345)
	`)
	queries := makeQueriesForRewriteDynamicsBenchmark([]int{size}, body)

	b.Run(strconv.Itoa(size), func(b *testing.B) {
		factory := newEqualityFactory(newLocalVarGenerator("q", nil))
		b.ResetTimer()
		for b.Loop() {
			for _, body := range queries[0] {
				rewriteDynamics(factory, body)
			}
		}
	})
}

// --- rule index ----------------------------------------------------------

func BenchmarkBenchlabBuildEqIndex(b *testing.B) {
	const n = 10000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		rules := eqIndexRules(n)
		for b.Loop() {
			index := newBaseDocEqIndex(isVirtual)
			if !index.Build(rules) {
				b.Fatal("failed to build index")
			}
		}
	})
}

func BenchmarkBenchlabLookupEqIndex(b *testing.B) {
	index := newBaseDocEqIndex(isVirtual)
	if !index.Build(eqIndexRules(1000)) {
		b.Fatal("failed to build index")
	}
	input := inputResolver{input: MustParseTerm(`{"foo": {"bar": 999}}`).Value}

	for b.Loop() {
		res, err := index.Lookup(input)
		if err != nil {
			b.Fatal(err)
		} else if len(res.Rules) != 1 {
			b.Fatalf("expected 1 rule, got %d", len(res.Rules))
		}
		IndexResultPool.Put(res)
	}
}

// The only lookup whose cost depends on the trie's shape: every rule reads a
// ref of its own and all of them hold, so traversal descends every branch.
// Guards the quadratic path-padding regression.
func BenchmarkBenchlabLookupDistinctRefIndex(b *testing.B) {
	const n = 1000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		index := newBaseDocEqIndex(isVirtual)
		if !index.Build(distinctRefRules(n)) {
			b.Fatal("failed to build index")
		}
		input := inputResolver{input: distinctRefInput(n)}

		b.ResetTimer()
		for b.Loop() {
			res, err := index.Lookup(input)
			if err != nil {
				b.Fatal(err)
			} else if len(res.Rules) != n {
				b.Fatalf("expected %d rules, got %d", n, len(res.Rules))
			}
			IndexResultPool.Put(res)
		}
	})
}

// --- parsing -------------------------------------------------------------

// The closest thing here to a user workload: default, if, and refs into both
// data and input.
func BenchmarkBenchlabParseBasicABACModule(b *testing.B) {
	BenchmarkParseBasicABACModule(b)
}

func BenchmarkBenchlabParseModuleRulesBase(b *testing.B) {
	const size = 1000
	b.Run(strconv.Itoa(size), func(b *testing.B) {
		runParseModuleBenchmark(b, generateModule(size))
	})
}

// A mid-depth object with every scalar type, which stands in for the whole
// nested-object family.
func BenchmarkBenchlabParseStatementMixedJSON(b *testing.B) {
	BenchmarkParseStatementMixedJSON(b)
}

// The recursion-depth guard. Arrays and objects are separate parser branches,
// so both are kept; depth is linear, so one point each.
func BenchmarkBenchlabParseDeepNesting(b *testing.B) {
	const depth = 12500
	b.Run("NestedArrays", func(b *testing.B) {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			runParseStatementBenchmark(b, generateDeeplyNestedArray(depth))
		})
	})
	b.Run("NestedObjects", func(b *testing.B) {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			runParseStatementBenchmark(b, generateDeeplyNestedObject(depth))
		})
	})
}

func BenchmarkBenchlabParseStatementSimpleArray(b *testing.B) {
	const size = 1000
	b.Run(strconv.Itoa(size), func(b *testing.B) {
		runParseStatementBenchmark(b, generateArrayStatement(size))
	})
}

// The only parse-error and backtracking case in the set.
func BenchmarkBenchlabParseStatementNestedObjectsOrSets(b *testing.B) {
	const size = 20
	b.Run(strconv.Itoa(size), func(b *testing.B) {
		stmt := generateObjectOrSetStatement(size)
		for b.Loop() {
			if _, err := ParseStatement(stmt); err == nil {
				b.Fatal("expected error")
			}
		}
	})
}

func BenchmarkBenchlabParseAnnotations(b *testing.B) {
	BenchmarkParseAnnotations(b)
}

func BenchmarkBenchlabParseComments(b *testing.B) {
	BenchmarkParseComments(b)
}

// The small-expression floor, on the path that is not handed capabilities.
func BenchmarkBenchlabParseVars(b *testing.B) {
	b.Run("with default options", func(b *testing.B) {
		for b.Loop() {
			if _, err := ParseExpr(`data[i][_][j]`); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Supersedes ParseSome.
func BenchmarkBenchlabParseEvery(b *testing.B) {
	BenchmarkParseEvery(b)
}

// --- objects, sets, arrays ----------------------------------------------

// The best tripwire in the package: a documented 26.8ms to 26.8us rehash fix.
// Compiling a rule holding a 16000-element array once spent 95% of its time in
// rehash.
func BenchmarkBenchlabArrayFill(b *testing.B) {
	const n = 5000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		elems := make([]*Term, n)
		for i := range elems {
			elems[i] = IntNumberTerm(i)
		}
		arr := NewArray(elems...)
		v := IntNumberTerm(-1)

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			for i := range n {
				arr.Set(i, v)
			}
		}
	})
}

// object.Insert growth under adversarial and favourable key order. The pair is
// the signal; either alone says little.
func BenchmarkBenchlabObjectConstruction(b *testing.B) {
	const n = 50000
	const seed = 67 // fixed, so the key order does not vary between runs

	b.Run("shuffled keys", func(b *testing.B) {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			es := make([]struct{ k, v int }, 0, n)
			for i := range n {
				es = append(es, struct{ k, v int }{i, i})
			}
			r := rand.New(rand.NewSource(seed))
			r.Shuffle(len(es), func(i, j int) { es[i], es[j] = es[j], es[i] })
			b.ResetTimer()
			for b.Loop() {
				obj := NewObject()
				for _, e := range es {
					obj.Insert(IntNumberTerm(e.k), IntNumberTerm(e.v))
				}
			}
		})
	})

	b.Run("increasing keys", func(b *testing.B) {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			es := make([]struct{ k, v int }, 0, n)
			for v := range n {
				es = append(es, struct{ k, v int }{v, v})
			}
			b.ResetTimer()
			for b.Loop() {
				obj := NewObject()
				for _, e := range es {
					obj.Insert(IntNumberTerm(e.k), IntNumberTerm(e.v))
				}
			}
		})
	})
}

// The insert-update paths, which ObjectConstruction does not reach.
func BenchmarkBenchlabObjectInsert(b *testing.B) {
	nums := slices.Collect(InternedIntRange(0, 100))

	b.Run("new key, new value", func(b *testing.B) {
		obj := newobject(0)
		for b.Loop() {
			for i := range nums {
				obj.Insert(nums[i], nums[i])
				if i >= len(nums)-1 {
					reset(obj)
				}
			}
		}
	})
}

// Three distinct branches out of the family's ten: interned-pointer identity,
// String hash and equal, and Number compare across representations. The rest
// are the same two functions with different arithmetic.
func BenchmarkBenchlabObjectGet(b *testing.B) {
	obj := NewObject(
		Item(InternedTerm("env"), InternedTerm("production")), // known interned string key
		Item(StringTerm("a"), InternedTerm(1)),
		Item(IntNumberTerm(222), InternedTerm("b")),
		Item(NumberTerm("3.14"), InternedTerm("c")),
		Item(NumberTerm("2.0"), InternedTerm("d")),
	)

	b.Run("existing interned key", func(b *testing.B) {
		key := InternedTerm("env")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing string key", func(b *testing.B) {
		key := StringTerm("a")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})

	b.Run("existing int number key as float", func(b *testing.B) {
		key := NumberTerm("222.0")
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})
}

// Get against object size. Replaces the ObjectCreationAndLookup family, which
// times the same single Get at six sizes.
func BenchmarkBenchlabObjectLookup(b *testing.B) {
	const n = 5000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		obj := NewObject()
		for i := range n {
			obj.Insert(StringTerm(strconv.Itoa(i)), InternedTerm(i))
		}
		key := StringTerm(strconv.Itoa(n - 1))
		b.ResetTimer()
		for b.Loop() {
			if obj.Get(key) == nil {
				b.Fatal("expected hit")
			}
		}
	})
}

// Ref traversal, object then array. Its 16-cell grid is nearly flat, so only
// the corner where both are large is kept.
func BenchmarkBenchlabObjectFind(b *testing.B) {
	const n, m = 5000, 5000
	b.Run(fmt.Sprintf("%d_%d", n, m), func(b *testing.B) {
		obj := NewObject()
		for i := range n {
			arr := NewArray()
			for j := range m {
				arr = arr.Append(IntNumberTerm(j))
			}
			obj.Insert(StringTerm(strconv.Itoa(i)), NewTerm(arr))
		}
		key := Ref{StringTerm(strconv.Itoa(n - 1)), IntNumberTerm(m - 1)}
		b.ResetTimer()
		for b.Loop() {
			value, err := obj.Find(key)
			if err != nil {
				b.Fatal(err)
			}
			if value == nil {
				b.Fatal("expected hit")
			}
		}
	})
}

// LazyObject is a separate map-backed implementation with its own
// materialization cost, and nothing else in the set covers it.
func BenchmarkBenchlabLazyObjectFind(b *testing.B) {
	const n, m = 5000, 5000
	b.Run(fmt.Sprintf("%d_%d", n, m), func(b *testing.B) {
		data := make(map[string]any, n)
		for i := range n {
			arr := make([]string, 0, m)
			for j := range m {
				arr = append(arr, strconv.Itoa(j))
			}
			data[strconv.Itoa(i)] = arr
		}
		obj := LazyObject(data)
		key := Ref{StringTerm(strconv.Itoa(n - 1)), IntNumberTerm(m - 1)}
		b.ResetTimer()
		for b.Loop() {
			value, err := obj.Find(key)
			if err != nil {
				b.Fatal(err)
			}
			if value == nil {
				b.Fatal("expected hit")
			}
		}
	})
}

// Replaces the SetCreationAndLookup family, same reason as ObjectLookup.
func BenchmarkBenchlabSetMembership(b *testing.B) {
	const n = 5000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		set := NewSet()
		for i := range n {
			set.Add(IntNumberTerm(i))
		}
		key := IntNumberTerm(n - 1)
		b.ResetTimer()
		for b.Loop() {
			if !set.Contains(key) {
				b.Fatal("expected hit")
			}
		}
	})
}

// Stands in for the Union / UnionOverlapping / IntersectionDifferentSize
// families, which are the same set-to-set machinery.
func BenchmarkBenchlabSetIntersection(b *testing.B) {
	const n = 5000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		setA := NewSet()
		setB := NewSet()
		for i := range n {
			setA.Add(IntNumberTerm(i))
			setB.Add(IntNumberTerm(i))
		}
		b.ResetTimer()
		for b.Loop() {
			if setC := setA.Intersect(setB); setC.Len() != setA.Len() {
				b.Fatal("expected equal")
			}
		}
	})
}

// String.Hash in isolation -- the primitive under every object and set case
// above.
func BenchmarkBenchlabTermHashing(b *testing.B) {
	const n = 1000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		s := String(strings.Repeat("a", n))
		b.ResetTimer()
		for b.Loop() {
			_ = s.Hash()
		}
	})
}

// --- refs and values ----------------------------------------------------

// CopyNonGround, which partial eval leans on.
func BenchmarkBenchlabRefCopyNonGround(b *testing.B) {
	b.Run("mixed", func(b *testing.B) {
		ref := MustParseRef("data.foo[x].bar[y]")
		for b.Loop() {
			_ = ref.CopyNonGround()
		}
	})
}

// Ref.String on the two branches that differ: many string parts, and
// escaping. It is the hot path in error messages and index keys.
func BenchmarkBenchlabRefString(b *testing.B) {
	for _, tc := range []struct{ name, inp, exp string }{
		{"really long",
			`data.policy.test1.test2.test3.test4["main1"]["main2"]["main3"]["main4"]`,
			`data.policy.test1.test2.test3.test4.main1.main2.main3.main4`},
		{"with escape",
			`data.policy["ma\tin"]`,
			`data.policy["ma\tin"]`},
	} {
		ref := MustParseRef(tc.inp)
		b.Run(tc.name, func(b *testing.B) {
			for b.Loop() {
				if ref.String() != tc.exp {
					b.Fatal("unexpected string")
				}
			}
		})
	}
}

// The AST-to-Go boundary, over a mixed term holding sets and maps.
func BenchmarkBenchlabValueToInterfaceInt(b *testing.B) {
	BenchmarkValueToInterfaceInt(b)
}

// The Go-to-AST boundary on the interning miss path; the interned arm is zero
// allocs and near noise.
func BenchmarkBenchlabInterfaceToValueInt(b *testing.B) {
	var v any = json.Number("12345")
	b.Run("non-interned int value", func(b *testing.B) {
		for b.Loop() {
			if _, err := InterfaceToValue(v); err != nil {
				b.Fatal(err)
			}
		}
	})
}
