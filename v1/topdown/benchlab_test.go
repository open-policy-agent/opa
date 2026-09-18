// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Curated subset of this package's benchmarks for the nightly benchlab run.
// See build/bench-nightly/set.json.
//
// Every case costs the same fixed slice of wall clock, whatever it measures, so
// size sweeps are cut to the sizes that say something the others do not. Where
// a family covers one code path at several sizes, only the ends are kept.

package topdown

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/storage/inmem"
	"github.com/open-policy-agent/opa/v1/util/test"
)

// --- whole-query eval ----------------------------------------------------

// 1x1 is the per-query fixed cost; 1000x1 is the scaling axis. Complete rules
// early-exit on the first hit, so the variants with many hits cost about the
// same as 1x1 and are dropped.
func BenchmarkBenchlabVirtualDocs(b *testing.B) {
	b.Run("1x1", func(b *testing.B) { runVirtualDocsBenchmark(b, 1, 1) })
	b.Run("1000x1", func(b *testing.B) { runVirtualDocsBenchmark(b, 1000, 1) })
}

func BenchmarkBenchlabLargeJSON(b *testing.B) {
	BenchmarkLargeJSON(b)
}

// A single key out of a 100k-entry base document, reached two ways: without
// materialising the object, and through object.get.
func BenchmarkBenchlabMemberWithKeyFromBaseDoc(b *testing.B) {
	BenchmarkMemberWithKeyFromBaseDoc(b)
}

func BenchmarkBenchlabObjectGetFromBaseDoc(b *testing.B) {
	BenchmarkObjectGetFromBaseDoc(b)
}

// Uncontended, read-scaling, and mixed read/write. The remaining Concurrency
// variants sit between these.
func BenchmarkBenchlabConcurrency1(b *testing.B) {
	benchmarkConcurrency(b, getParams(1, 0))
}

func BenchmarkBenchlabConcurrency8(b *testing.B) {
	benchmarkConcurrency(b, getParams(8, 0))
}

func BenchmarkBenchlabConcurrency4Readers1Writer(b *testing.B) {
	benchmarkConcurrency(b, getParams(4, 1))
}

func BenchmarkBenchlabWalk(b *testing.B) {
	const n = 3000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		store := inmem.NewFromObject(genWalkBenchmarkData(n))
		compiler := ast.NewCompiler()
		compiled, err := compiler.QueryCompiler().Compile(
			ast.MustParseBody(fmt.Sprintf(`walk(data, [["arr", %d], x])`, n-1)))
		if err != nil {
			b.Fatal(err)
		}
		ctx := b.Context()

		err = storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
			q := NewQuery(compiled).WithStore(store).WithCompiler(compiler).WithTransaction(txn)
			for b.Loop() {
				if _, err := q.Run(ctx); err != nil {
					b.Fatal(err)
				}
			}
			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	})
}

// --- iteration and enumerate --------------------------------------------

// These build the fixture rather than enumerating it (see benchmarkIteration);
// what differs between the three is Array append vs Set.Add vs Object.Insert.
// Only the largest size is kept, and it is named, so the chart says which size
// it is rather than leaving the reader to guess.
func BenchmarkBenchlabArrayIteration(b *testing.B) {
	b.Run("10000", func(b *testing.B) {
		benchmarkIteration(b, test.ArrayIterationBenchmarkModule(10000))
	})
}

func BenchmarkBenchlabSetIteration(b *testing.B) {
	b.Run("10000", func(b *testing.B) {
		benchmarkIteration(b, test.SetIterationBenchmarkModule(10000))
	})
}

func BenchmarkBenchlabObjectIteration(b *testing.B) {
	b.Run("10000", func(b *testing.B) {
		benchmarkIteration(b, test.ObjectIterationBenchmarkModule(10000))
	})
}

func BenchmarkBenchlabEnumerateRandomAccess(b *testing.B) {
	BenchmarkEnumerateRandomAccess(b)
}

// evalTerm.enumerate's Object and Set cases, which the *Iteration families
// above do not reach.
func BenchmarkBenchlabEnumerateInputObject(b *testing.B) {
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `package test

total := c if { c := count([v | some _, v in input.obj]) }`,
	})
	query := ast.MustParseBody(`data.test.total`)

	const n = 1000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		input := ast.ObjectTerm(ast.Item(ast.StringTerm("obj"), wideObject(n)))
		for b.Loop() {
			if _, err := NewQuery(query).WithCompiler(compiler).WithInput(input).Run(b.Context()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkBenchlabEnumerateInputSet(b *testing.B) {
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `package test

hits contains v if {
	some v in input.s
	v > 5
}`,
	})
	query := ast.MustParseBody(`data.test.hits`)

	const n = 1000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		elems := make([]*ast.Term, n)
		for i := range n {
			elems[i] = ast.InternedTerm(i)
		}
		input := ast.ObjectTerm(ast.Item(ast.StringTerm("s"), ast.SetTerm(elems...)))

		for b.Loop() {
			if _, err := NewQuery(query).WithCompiler(compiler).WithInput(input).Run(b.Context()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// --- comprehensions -----------------------------------------------------

// The comprehension index cache, over an array and an object comprehension.
// The set case shares the object case's index machinery, and the smaller sizes
// re-measure the same path.
func BenchmarkBenchlabComprehensionIndexing(b *testing.B) {
	cases := []struct{ note, module, query string }{
		{
			note: "arrays",
			module: `package test

bench_array if {
	v := data.items[_]
	ks := [k | some k; v == data.items[k]]
}`,
			query: `data.test.bench_array = true`,
		},
		{
			note: "objects",
			module: `package test

bench_object if {
	v := data.items[_]
	ks := {k: 1 | some k; v == data.items[k]}
}`,
			query: `data.test.bench_object = true`,
		},
	}

	const n = 1000
	ctx := b.Context()

	for _, tc := range cases {
		b.Run(fmt.Sprintf("%v_%v", tc.note, n), func(b *testing.B) {
			store := inmem.NewFromObject(genComprehensionIndexingData(n))
			compiler := ast.MustCompileModules(map[string]string{"test.rego": tc.module})
			query, err := compiler.QueryCompiler().Compile(ast.MustParseBody(tc.query))
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for b.Loop() {
				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
					m := metrics.New()
					q := NewQuery(query).
						WithStore(store).
						WithCompiler(compiler).
						WithTransaction(txn).
						WithInstrumentation(NewInstrumentation(m))
					rs, err := q.Run(ctx)
					if m.Counter(evalOpComprehensionCacheMiss).Value().(uint64) > 0 {
						b.Fatal("expected zero cache misses")
					}
					if err != nil || len(rs) != 1 {
						b.Fatal("unexpected result:", rs, "err:", err)
					}
					return nil
				})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Regal-shaped: some..in over nested input with sprintf into a partial set,
// and an object-comprehension head with count(). ShortLines is dropped as the
// third spelling of the same walk.
func BenchmarkBenchlabComprehensionHighFrequency(b *testing.B) {
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `package test

violations contains msg if {
	some file in input.files
	some line in file.lines
	line.length > 80
	msg := sprintf("Line too long in %s", [file.name])
}

file_stats := {file.name: len |
	some file in input.files
	len := count(file.lines)
}`,
	})

	inputData := map[string]any{
		"files": []map[string]any{
			{
				"name": "file1.rego",
				"lines": []map[string]any{
					{"length": 50}, {"length": 90}, {"length": 70},
				},
			},
			{
				"name": "file2.rego",
				"lines": []map[string]any{
					{"length": 60}, {"length": 85},
				},
			},
		},
	}
	store := inmem.NewFromObject(inputData)
	input := ast.NewTerm(ast.MustInterfaceToValue(inputData))

	for _, tc := range []struct{ name, query string }{
		{"Violations", "data.test.violations"},
		{"FileStats", "data.test.file_stats"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			query := ast.MustParseBody(tc.query)
			for b.Loop() {
				q := NewQuery(query).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(input)
				if _, err := q.Run(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A comprehension nested inside a comprehension -- the only nested case at any
// size. The 20-element Array/Set variants it ships with are the flat shape the
// indexing cases above already cover.
func BenchmarkBenchlabComprehensionLargeIteration(b *testing.B) {
	b.Run("NestedComp", func(b *testing.B) {
		compiler := ast.MustCompileModules(map[string]string{
			"test.rego": `package test
arr := [1, 2, 3, 4, 5]
result := [z | x := arr[_]; y := [a | a := arr[_]; a > x][_]; z := x + y]`,
		})
		store := inmem.New()
		query := ast.MustParseBody("data.test.result")

		for b.Loop() {
			q := NewQuery(query).WithCompiler(compiler).WithStore(store)
			if _, err := q.Run(b.Context()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// --- rules and the rule index -------------------------------------------

// Bindings growth measured through real eval: the ends of the sweep give the
// slope.
func BenchmarkBenchlabRuleBindings(b *testing.B) {
	input := ast.NewTerm(ast.MustInterfaceToValue(map[string]any{
		"user":       "admin",
		"role":       "superuser",
		"department": "engineering",
		"status":     "active",
		"level":      10,
		"active":     true,
	}))

	for _, tc := range []struct{ name, module string }{
		{"Rule_1Expr", `package test

allow if {
	input.user == "admin"
}`},
		{"Rule_10Exprs", `package test

allow if {
	input.user == "admin"
	input.role == "superuser"
	input.department == "engineering"
	input.status == "active"
	input.level == 10
	input.active == true
	a := input.user
	c := input.department
	e := input.level
	a != c
	e > 5
}`},
	} {
		b.Run(tc.name, func(b *testing.B) {
			compiler := ast.MustCompileModules(map[string]string{"test.rego": tc.module})
			store := inmem.New()
			query := ast.MustParseBody("data.test.allow")

			b.ResetTimer()
			for b.Loop() {
				q := NewQuery(query).
					WithCompiler(compiler).
					WithStore(store).
					WithInput(input)
				if _, err := q.Run(b.Context()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A comprehension inside a rule. Subsumes the RuleWithComprehensions family,
// which is the same shape.
func BenchmarkBenchlabComplexRules(b *testing.B) {
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `package test

analyze_access if {
	users := [u | some u in input.users; u.active]
	admins := {u.name | some u in users; u.role == "admin"}
	count(admins) > 0
}`,
	})
	store := inmem.New()
	query := ast.MustParseBody("data.test.analyze_access")
	input := ast.NewTerm(ast.MustInterfaceToValue(map[string]any{
		"users": []map[string]any{
			{"name": "alice", "role": "admin", "active": true},
			{"name": "bob", "role": "user", "active": true},
			{"name": "charlie", "role": "admin", "active": false},
		},
	}))

	b.Run("AnalyzeAccess", func(b *testing.B) {
		for b.Loop() {
			q := NewQuery(query).
				WithCompiler(compiler).
				WithStore(store).
				WithInput(input)
			if _, err := q.Run(b.Context()); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Only direct probe of biunify.
func BenchmarkBenchlabBiunifyArrays(b *testing.B) {
	q := NewQuery(ast.MustParseBody("[1,x,3] = [y,5,6]"))
	ctx := b.Context()

	for b.Loop() {
		if _, err := q.Run(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBenchlabFunctionArgumentIndex(b *testing.B) {
	const n = 1000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		compiler := ast.MustCompileModules(map[string]string{"test.rego": moduleWithDefs(n)})
		body := ast.MustParseBody(fmt.Sprintf("data.test.f(%d, x)", n))
		ctx := b.Context()

		for b.Loop() {
			if _, err := NewQuery(body).WithCompiler(compiler).WithIndexing(true).Run(ctx); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// The rule index trie on a miss, with and without indexing: the pair is what
// makes the number interpretable. prefixes=1000 rather than 10000, which needs
// gigabytes of policy source to build.
func BenchmarkBenchlabRuleIndexPrefixMatch(b *testing.B) {
	const (
		rules   = 250
		perRule = 1000
	)

	for _, shape := range prefixMatchShapes {
		if shape.note != "any_prefix_match" {
			continue
		}
		b.Run(shape.note, func(b *testing.B) {
			for _, indexing := range []bool{true, false} {
				b.Run(fmt.Sprintf("indexing=%v", indexing), func(b *testing.B) {
					benchmarkPrefixMatch(b, shape, rules, perRule, indexing, "/no/such/path/at/all", false)
				})
			}
		})
	}
}

// --- partial eval -------------------------------------------------------

func BenchmarkBenchlabPartialEval(b *testing.B) {
	b.Run("1000", func(b *testing.B) { runPartialEvalBenchmark(b, 1000) })
}

func BenchmarkBenchlabPartialEvalCompile(b *testing.B) {
	b.Run("1000", func(b *testing.B) { runPartialEvalCompileBenchmark(b, 1000) })
}

// A designed pair: with every policy conditioned on the unknown the cost should
// be flat in the policy count, and without that it should not be. Only the
// largest size is kept, which is where the two come apart.
func BenchmarkBenchlabPartialEvalDynamicComposition(b *testing.B) {
	const n = 5000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		runDynamicCompositionBenchmark(b, generateDynamicCompositionPolicies(n, n, `input.attribute == "yes"`))
	})
}

func BenchmarkBenchlabPartialEvalDynamicCompositionKnownRules(b *testing.B) {
	const n = 5000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		runDynamicCompositionBenchmark(b, generateDynamicCompositionPolicies(n, 100, `input.type == "no-match"`))
	})
}

// The save-set path, emitting one saved query per base document element. 300000
// is left out: it spends the whole budget on a handful of samples.
func BenchmarkBenchlabInliningFullScan(b *testing.B) {
	const n = 10000
	b.Run(strconv.Itoa(n), func(b *testing.B) {
		ctx := b.Context()
		body := ast.MustParseBody("data.test.p = true")
		unknowns := []*ast.Term{ast.MustParseTerm("input")}
		compiler := ast.MustCompileModules(map[string]string{
			"test.rego": `package test

p if {
	data.a[i] == input
}`,
		})
		store := inmem.NewFromObject(generateInlineFullScanBenchmarkData(n))

		b.ResetTimer()
		for b.Loop() {
			err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {
				q := NewQuery(body).
					WithCompiler(compiler).
					WithStore(store).
					WithTransaction(txn).
					WithUnknowns(unknowns)
				_, _, err := q.PartialRun(ctx)
				return err
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// --- bindings -----------------------------------------------------------

// The floor and the ceiling of the sweep, with and without the size hint. The
// 16/20 rows are the array-to-hashmap boundary, which the transition benchmark
// below isolates properly.
func BenchmarkBenchlabBindingsAllocation(b *testing.B) {
	const maxBindings = 50

	var keys, vals [maxBindings]*ast.Term
	for j := range maxBindings {
		keys[j] = ast.VarTerm(fmt.Sprintf("x%d", j))
		vals[j] = ast.InternedTerm(j)
	}

	u := &undo{}

	for _, tt := range []struct {
		name     string
		bindings int
	}{
		{"1_binding", 1},
		{"50_bindings", maxBindings},
	} {
		b.Run(tt.name+"_without_hint", func(b *testing.B) {
			for b.Loop() {
				bi := newBindings(nil)
				for j := range tt.bindings {
					bi.bind(keys[j], vals[j], nil, u)
				}
			}
		})

		b.Run(tt.name+"_with_hint", func(b *testing.B) {
			for b.Loop() {
				bi := newBindingsWithSize(0, nil, tt.bindings)
				for j := range tt.bindings {
					bi.bind(keys[j], vals[j], nil, u)
				}
			}
		})
	}
}

func BenchmarkBenchlabBindingsArrayHashmapTransition(b *testing.B) {
	BenchmarkBindingsArrayHashmapTransition(b)
}

// --- cache --------------------------------------------------------------

func BenchmarkBenchlabVirtualCache(b *testing.B) {
	BenchmarkVirtualCache(b)
}
