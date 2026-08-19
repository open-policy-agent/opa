package topdown

import (
	"strconv"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

func BenchmarkInliningFullScan(b *testing.B) {

	ctx := b.Context()
	body := ast.MustParseBody("data.test.p = true")
	unknowns := []*ast.Term{ast.MustParseTerm("input")}
	compiler := ast.MustCompileModules(map[string]string{
		"test.rego": `
		package test

		p if {
			data.a[i] == input
		}
		`,
	})

	sizes := []int{1000, 10000, 300000}

	for _, n := range sizes {

		b.Run(strconv.Itoa(n), func(b *testing.B) {

			store := inmem.NewFromObject(generateInlineFullScanBenchmarkData(n))

			b.ResetTimer()

			for b.Loop() {

				err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

					q := NewQuery(body).
						WithCompiler(compiler).
						WithStore(store).
						WithTransaction(txn).
						WithUnknowns(unknowns)

					queries, support, err := q.PartialRun(ctx)
					if err != nil {
						b.Fatal(err)
					}

					if len(queries) != n {
						b.Fatal("Expected", n, "queries")
					} else if len(support) != 0 {
						b.Fatal("Unexpected support")
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

func generateInlineFullScanBenchmarkData(n int) map[string]any {

	sl := make([]any, n)
	for i := range sl {
		sl[i] = strconv.Itoa(i)
	}

	return map[string]any{
		"a": sl,
	}
}

// Was
// BenchmarkPartialEvalDynamicComposition/10-16           	   40417	     28230 ns/op	   19373 B/op	     439 allocs/op
// BenchmarkPartialEvalDynamicComposition/100-16          	    9672	    126033 ns/op	   60020 B/op	     481 allocs/op
// BenchmarkPartialEvalDynamicComposition/1000-16         	     898	   1297354 ns/op	  651551 B/op	     560 allocs/op
// BenchmarkPartialEvalDynamicComposition/5000-16         	     180	   6603512 ns/op	 2661954 B/op	     742 allocs/op
//
// Now
// BenchmarkPartialEvalDynamicComposition/10-16           	   63439	     18205 ns/op	   15795 B/op	     386 allocs/op
// BenchmarkPartialEvalDynamicComposition/100-16          	   58730	     21472 ns/op	   15851 B/op	     386 allocs/op
// BenchmarkPartialEvalDynamicComposition/1000-16         	   41414	     24438 ns/op	   15743 B/op	     386 allocs/op
// BenchmarkPartialEvalDynamicComposition/5000-16         	   44376	     24398 ns/op	   15713 B/op	     386 allocs/op
// BenchmarkPartialEvalDynamicComposition partially evaluates an entrypoint that
// composes policies dynamically, i.e. the packages to evaluate are selected by
// (known) input values. Only a single policy applies to any given input, so the
// cost of partial evaluation should be mostly independent of how many policies
// are loaded.
func BenchmarkPartialEvalDynamicComposition(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			// One policy per type/subtype pair, and every policy conditioned on
			// the unknown, so the applicable rule requires saving.
			runDynamicCompositionBenchmark(b, generateDynamicCompositionPolicies(n, n, `input.attribute == "yes"`))
		})
	}
}

// Was
// BenchmarkPartialEvalDynamicCompositionKnownRules/10-16 	   39566	     30391 ns/op	   13041 B/op	     408 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules/100-16         	    5332	    224829 ns/op	   47862 B/op	    2028 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules/1000-16        	     271	   4378521 ns/op	  975648 B/op	   45430 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules/5000-16        	      13	  82562061 ns/op	17643343 B/op	  826324 allocs/op
//
// Now
// BenchmarkPartialEvalDynamicCompositionKnownRules/10-16 	   44154	     27070 ns/op	   12510 B/op	     382 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules/100-16         	    6354	    189338 ns/op	   41534 B/op	    1732 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules/1000-16        	     804	   1515485 ns/op	  335291 B/op	   15467 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules/5000-16        	     142	   8450281 ns/op	 1639188 B/op	   76507 allocs/op
// BenchmarkPartialEvalDynamicCompositionKnownRules is the counterpart of
// BenchmarkPartialEvalDynamicComposition for policies that don't depend on the
// unknown: partial evaluation has to consider every rule that could apply before
// concluding that none of them needs to be saved, so the applicable packages
// being few is all that keeps the cost down.
func BenchmarkPartialEvalDynamicCompositionKnownRules(b *testing.B) {
	sizes := []int{10, 100, 1000, 5000}

	for _, n := range sizes {
		b.Run(strconv.Itoa(n), func(b *testing.B) {
			// Spread over 100 type/subtype pairs, so a hundredth of the policies
			// apply to the input and the entrypoint's rule is evaluated for each.
			runDynamicCompositionBenchmark(b, generateDynamicCompositionPolicies(n, 100, `input.type == "no-match"`))
		})
	}
}

func runDynamicCompositionBenchmark(b *testing.B, modules map[string]string) {

	ctx := b.Context()
	body := ast.MustParseBody("data.main.allow = true")
	unknowns := []*ast.Term{ast.MustParseTerm("input.attribute")}
	input := ast.MustParseTerm(`{"type": "1", "subtype": "1"}`)

	compiler := ast.MustCompileModules(modules)
	store := inmem.New()

	b.ResetTimer()

	for b.Loop() {

		err := storage.Txn(ctx, store, storage.TransactionParams{}, func(txn storage.Transaction) error {

			q := NewQuery(body).
				WithCompiler(compiler).
				WithStore(store).
				WithTransaction(txn).
				WithInput(input).
				WithUnknowns(unknowns)

			queries, support, err := q.PartialRun(ctx)
			if err != nil {
				b.Fatal(err)
			}

			if len(queries) != 1 {
				b.Fatal("Expected exactly one query but got:", queries)
			} else if len(support) != 0 {
				b.Fatal("Unexpected support:", support)
			}

			return nil
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// generateDynamicCompositionPolicies spreads n policies over the given number of
// type/subtype pairs, giving each policy a rule conditioned on ruleBody.
func generateDynamicCompositionPolicies(n, pairs int, ruleBody string) map[string]string {

	modules := map[string]string{
		"main.rego": `package main

		denies contains x if {
			x := data.policies[input.type][input.subtype][_].denies[_]
		}

		allow if not any_denies

		any_denies if denies[_]
		`,
	}

	for i := range n {
		key := strconv.Itoa(i % pairs)
		modules[strconv.Itoa(i)+".rego"] = `package policies["` + key + `"]["` + key + `"].policy` + strconv.Itoa(i) + `

		denies contains "denied" if ` + ruleBody + `
		`
	}

	return modules
}
