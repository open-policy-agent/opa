// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cover

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/open-policy-agent/opa/v1/topdown"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
)

func TestCover(t *testing.T) {

	cover := New()

	module := `package test

import data.deadbeef # expect not reported

foo if {
	bar
	p
	not baz
}

bar if {
	a := 1
	b := 2
	a != b
}

baz if {     # expect no exit
	true
	false # expect eval but fail
	true  # expect not covered
}

p if {
	some bar # should not be included in coverage report
	bar = 1
	bar + 1 == 2
}
`

	parsedModule, err := ast.ParseModuleWithOpts("test.rego", module, ast.ParserOptions{AllFutureKeywords: true})
	if err != nil {
		t.Fatal(err)
	}

	eval := rego.New(
		rego.ParsedModule(parsedModule),
		rego.Query("data.test.foo"),
		rego.QueryTracer(cover),
	)

	ctx := t.Context()
	_, err = eval.Eval(ctx)

	if err != nil {
		t.Fatal(err)
	}

	report := cover.Report(map[string]*ast.Module{
		"test.rego": parsedModule,
	})

	fr, ok := report.Files["test.rego"]
	if !ok {
		t.Fatal("Expected file report for test.rego")
	}

	expectedCovered := []Position{
		{Row: 5},                     // foo head
		{Row: 6}, {Row: 7}, {Row: 8}, // foo body
		{Row: 11},                       // bar head
		{Row: 12}, {Row: 13}, {Row: 14}, // bar body
		{Row: 18}, {Row: 19}, // baz body hits
		{Row: 23},            // p head
		{Row: 25}, {Row: 26}, // p body
	}

	expectedNotCovered := []Position{
		{Row: 17}, // baz head
		{Row: 20}, // baz body miss
	}

	for _, exp := range expectedCovered {
		if !fr.IsCovered(exp.Row) {
			t.Errorf("Expected %v to be covered", exp)
		}
	}

	for _, exp := range expectedNotCovered {
		if !fr.IsNotCovered(exp.Row) {
			t.Errorf("Expected %v to NOT be covered", exp)
		}
	}

	if len(expectedCovered) != fr.locCovered() {
		t.Errorf(
			"Expected %d loc to be covered, got %d instead",
			len(expectedCovered),
			fr.locCovered())
	}

	if len(expectedNotCovered) != fr.locNotCovered() {
		t.Errorf(
			"Expected %d loc to not be covered, got %d instead",
			len(expectedNotCovered),
			fr.locNotCovered())
	}

	expectedCoveragePercentage := 100.0 * float64(len(expectedCovered)) / float64(len(expectedCovered)+len(expectedNotCovered))
	if expectedCoveragePercentage != fr.Coverage {
		t.Errorf("Expected coverage %v != %v", expectedCoveragePercentage, fr.Coverage)
	}

	// there's just one file, hence the overall coverage is equal to the
	// one of the only file report we have
	if expectedCoveragePercentage != report.Coverage {
		t.Errorf("Expected report coverage %f != %f",
			expectedCoveragePercentage,
			report.Coverage)
	}

	if t.Failed() {
		bs, err := json.MarshalIndent(fr, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(string(bs))
	}
}

func TestCoverRangeCases(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		module     string
		query      string
		input      any
		reportKey  string  // defaults to "test.rego" if empty
		seeds      []int64 // when set, seeds rand.* via one shared reader (one int64 drawn per rand evaluation)
		covered    []Range
		notCovered []Range
	}{
		"rule head and value expression on same row counted once": {
			module: `package test

# Both a rule and an expression, but should not be counted twice
foo := 1

allow if { true }
`,
			query: "data.test.allow",
			covered: []Range{
				{Start: Position{Row: 6, Col: 1}, End: Position{Row: 6, Col: 6}},   // allow head
				{Start: Position{Row: 6, Col: 12}, End: Position{Row: 6, Col: 16}}, // true
			},
			notCovered: []Range{
				{Start: Position{Row: 4, Col: 1}, End: Position{Row: 4, Col: 9}}, // foo := 1 head
				{Start: Position{Row: 4, Col: 8}, End: Position{Row: 4, Col: 9}}, // its generated value expr
			},
		},
		"inline rule head not covered": {
			module: `package test

foo if false

test_foo if {
	not foo
}
`,
			query: "data.test.test_foo",
			covered: []Range{
				{Start: Position{Row: 3, Col: 8}, End: Position{Row: 3, Col: 13}}, // false expr
				{Start: Position{Row: 5, Col: 1}, End: Position{Row: 5, Col: 9}},  // test_foo head
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 9}},  // not foo
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}}, // foo head
			},
		},
		"index-excluded rule body is not covered": {
			module: `package test

allow if {
	input.action == "read"
}

allow if {
	input.action in {"delete", "update"}
}
`,
			query: "data.test.allow",
			input: map[string]any{"action": "write"},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}, Kinds: []Kind{KindIndexExcluded}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}, Kinds: []Kind{KindIndexExcluded}},
			},
		},
		"index exclusions inside a with scope": {
			// with pushes a new eval frame and exclusions there must still be reported
			module: `package test

allow if {
	input.action == "read"
}

allow if {
	input.action in {"delete", "update"}
}

test_allow_write if {
	allow with input as {"action": "write"}
}
`,
			query: "data.test.test_allow_write",
			covered: []Range{
				{Start: Position{Row: 12, Col: 2}, End: Position{Row: 12, Col: 41}}, // the with expr
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}, Kinds: []Kind{KindIndexExcluded}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}, Kinds: []Kind{KindIndexExcluded}},
				// both allow bodies were excluded, so the test rule never exits
				{Start: Position{Row: 11, Col: 1}, End: Position{Row: 11, Col: 17}},
			},
		},
		"index-excluded rule body is not covered, bundle key mismatch": {
			// Lookup must key on loc.File, not the modules map key, since bundle
			// modules key differently from their parse-time Location.File.
			module: `package test

allow if {
	input.action == "read"
}

allow if {
	input.action in {"delete", "update"}
}
`,
			query:     "data.test.allow",
			input:     map[string]any{"action": "write"},
			reportKey: "bundle/test.rego",
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}, Kinds: []Kind{KindIndexExcluded}},
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 38}, Kinds: []Kind{KindIndexExcluded}},
			},
		},
		"index-excluded root rule: else branch also excluded": {
			module: `package test

allow if {
	input.action == "read"
} else if {
	input.action == "admin"
}

allow if {
	input.action in {"delete", "update"}
}
`,
			query: "data.test.allow",
			input: map[string]any{"action": "write"},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},                                      // allow head (root)
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 24}, Kinds: []Kind{KindIndexExcluded}},   // input.action == "read"
				{Start: Position{Row: 5, Col: 3}, End: Position{Row: 5, Col: 7}},                                      // else head
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 25}, Kinds: []Kind{KindIndexExcluded}},   // input.action == "admin"
				{Start: Position{Row: 9, Col: 1}, End: Position{Row: 9, Col: 6}},                                      // allow head (second)
				{Start: Position{Row: 10, Col: 2}, End: Position{Row: 10, Col: 38}, Kinds: []Kind{KindIndexExcluded}}, // input.action in ...
			},
		},
		"else rule promoted to root by indexer: short-circuited else body is not falsely index-excluded": {
			// Lookup can promote the else-rule into the "root" slot; input.foo is
			// merely short-circuited by 1 == 2, not index-excluded.
			module: `package test

allow if {
	input.undef
} else if {
	1 == 2
	input.foo
}
`,
			query: "data.test.allow",
			input: map[string]any{"foo": true},
			covered: []Range{
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 8}}, // 1 == 2
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 13}, Kinds: []Kind{KindIndexExcluded}}, // input.undef
				{Start: Position{Row: 5, Col: 3}, End: Position{Row: 5, Col: 7}},                                    // else head: reached, just never exits
				{Start: Position{Row: 7, Col: 2}, End: Position{Row: 7, Col: 11}},                                   // input.foo: short-circuited, NOT index-excluded
			},
		},
		"else body included in index but short-circuited by earlier branch": {
			module: `package test

allow if {        # covered
	input.foo     # covered
} else if {       # not_covered
	input.foo     # not_covered: root branch already succeeded
	not input.bar # not_covered
}
`,
			query: "data.test.allow",
			input: map[string]any{"foo": true},
			covered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 11}},
			},
			// None of the else rows carry a Kind: the whole chain was selected
			// by the index, the else body just never ran.
			notCovered: []Range{
				{Start: Position{Row: 5, Col: 3}, End: Position{Row: 5, Col: 7}},
				{Start: Position{Row: 6, Col: 2}, End: Position{Row: 6, Col: 11}},
				{Start: Position{Row: 7, Col: 2}, End: Position{Row: 7, Col: 15}},
			},
		},
		"early-exit skips the second matching rule and its dependency": {
			// allow's two rules both yield the same (implicit) value, so the
			// engine can stop after the first match; the second rule and
			// the helper it alone calls never run under the default config.
			module: `package test

allow if { true }
allow if { extra }

extra if { true }
`,
			query: "data.test.allow",
			covered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 3, Col: 12}, End: Position{Row: 3, Col: 16}},
			},
			notCovered: []Range{
				{Start: Position{Row: 4, Col: 1}, End: Position{Row: 4, Col: 6}, Kinds: []Kind{KindEarlyExit}},
				{Start: Position{Row: 4, Col: 12}, End: Position{Row: 4, Col: 17}, Kinds: []Kind{KindEarlyExit}},
				{Start: Position{Row: 6, Col: 1}, End: Position{Row: 6, Col: 6}, Kinds: []Kind{KindEarlyExit}},
				{Start: Position{Row: 6, Col: 12}, End: Position{Row: 6, Col: 16}, Kinds: []Kind{KindEarlyExit}},
			},
		},
		"multi-line expression is one range spanning its rows": {
			// Ranges are per AST node, so a head is always separate from its
			// body exprs, but an expr written over several rows is one range.
			module: `package test

allow if {
	input.action in {
		"delete",
		"update",
	}
}
`,
			query: "data.test.allow",
			input: map[string]any{"action": "delete"},
			covered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},  // allow head, single row
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 14}}, // `input.action`, single row
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 7, Col: 3}},  // whole `in` expr, rows 4-7
			},
		},
		"negated expression rule is not covered without indexer exclusion": {
			module: `package test

allow if {
	not input.blocked
}

other if {
	true
}
`,
			query: "data.test.other",
			covered: []Range{
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 6}},
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 6}},
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 6}},
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 19}},
			},
		},
		"semicolon-separated expressions short-circuit": {
			module: `package test

foo if {
	true; true; false; false
}

test_foo if {
	not foo
}
`,
			query: "data.test.test_foo",
			// Row 4: `\ttrue; true; false; false`
			covered: []Range{
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 6}},   // true
				{Start: Position{Row: 4, Col: 8}, End: Position{Row: 4, Col: 12}},  // true
				{Start: Position{Row: 4, Col: 14}, End: Position{Row: 4, Col: 19}}, // false (caused failure)
				{Start: Position{Row: 7, Col: 1}, End: Position{Row: 7, Col: 9}},   // test_foo head
				{Start: Position{Row: 8, Col: 2}, End: Position{Row: 8, Col: 9}},   // not foo
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 4}},   // foo head
				{Start: Position{Row: 4, Col: 21}, End: Position{Row: 4, Col: 26}}, // false (never evaluated)
			},
		},
		"nondeterministic value gating an early-exit skip is pinned in supplementary passes": {
			// A rand.intn value gates whether q is reached. q has two definitions
			// yielding the same value, so early exit skips the second. The shared
			// non-deterministic builtin cache pins sel == 0 for every pass, so q
			// is reached and the early_exit kinds are stable; without it the
			// no-early-exit pass would draw sel == 1, never reach q, and drop the
			// kinds.
			module: `package test

p if {
	sel == 0
	q
}

q if {
	true
}

q if {
	true
}

sel := rand.intn("k", 2)
`,
			query: "data.test.p",
			input: map[string]any{"x": "a"},
			// The baseline draws the first seed (0 -> rand.intn(_,2) == 0). The
			// shared cache makes the other passes replay it, so the later seeds
			// (-> 1) are only drawn if a pass recomputes rand - the divergence
			// this guards against.
			seeds: []int64{0, 1, 1},
			covered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 2}},    // p head
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 10}},   // sel == 0
				{Start: Position{Row: 5, Col: 2}, End: Position{Row: 5, Col: 3}},    // q
				{Start: Position{Row: 8, Col: 1}, End: Position{Row: 8, Col: 2}},    // q (first) head
				{Start: Position{Row: 9, Col: 2}, End: Position{Row: 9, Col: 6}},    // true
				{Start: Position{Row: 16, Col: 1}, End: Position{Row: 16, Col: 25}}, // sel := rand.intn("k", 2)
				{Start: Position{Row: 16, Col: 8}, End: Position{Row: 16, Col: 25}}, // rand.intn("k", 2)
			},
			notCovered: []Range{
				{Start: Position{Row: 12, Col: 1}, End: Position{Row: 12, Col: 2}, Kinds: []Kind{KindEarlyExit}}, // q (second) head
				{Start: Position{Row: 13, Col: 2}, End: Position{Row: 13, Col: 6}, Kinds: []Kind{KindEarlyExit}}, // true
			},
		},
		"nondeterministic value gating an index-excluded rule is pinned in supplementary passes": {
			// A rand.intn value gates whether q (whose definitions the index
			// excludes for this input) is reached. The shared non-deterministic
			// builtin cache pins sel == 0 for every pass, so q is reached and its
			// index_excluded kinds are stable; without it the no-index pass would
			// draw sel == 1, never reach q, and drop the kinds.
			module: `package test

p if {
	sel == 0
	q
}

q if {
	input.x == "read"
}

sel := rand.intn("k", 2)
`,
			query: "data.test.p",
			input: map[string]any{"x": "a"},
			// The baseline draws the first seed (0 -> rand.intn(_,2) == 0). The
			// shared cache makes the other passes replay it, so the later seeds
			// (-> 1) are only drawn if a pass recomputes rand - the divergence
			// this guards against.
			seeds: []int64{0, 1, 1},
			// sel is pinned to 0, so p enters its body and reaches the q
			// reference on line 5 (covered). q's own body is then index-excluded
			// (see notCovered) - had sel been 1, p would stop at sel == 0, q
			// would never be reached, and there would be no exclusion to report.
			covered: []Range{
				{Start: Position{Row: 4, Col: 2}, End: Position{Row: 4, Col: 10}},   // sel == 0
				{Start: Position{Row: 5, Col: 2}, End: Position{Row: 5, Col: 3}},    // q
				{Start: Position{Row: 12, Col: 1}, End: Position{Row: 12, Col: 25}}, // sel := rand.intn("k", 2)
				{Start: Position{Row: 12, Col: 8}, End: Position{Row: 12, Col: 25}}, // rand.intn("k", 2)
			},
			notCovered: []Range{
				{Start: Position{Row: 3, Col: 1}, End: Position{Row: 3, Col: 2}},                                    // p head
				{Start: Position{Row: 8, Col: 1}, End: Position{Row: 8, Col: 2}},                                    // q head
				{Start: Position{Row: 9, Col: 2}, End: Position{Row: 9, Col: 19}, Kinds: []Kind{KindIndexExcluded}}, // input.x == "read"
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			baseline := New()
			noIndex := New()
			noEarlyExit := New()
			baseline.AddRun(KindIndexExcluded, noIndex)
			baseline.AddRun(KindEarlyExit, noEarlyExit)

			parsedModule, err := ast.ParseModule("test.rego", tc.module)
			if err != nil {
				t.Fatalf("failed to parse module: %v", err)
			}

			args := []func(*rego.Rego){
				rego.ParsedModule(parsedModule),
				rego.Query(tc.query),
			}
			if tc.input != nil {
				args = append(args, rego.Input(tc.input))
			}

			ctx := t.Context()
			pq, err := rego.New(args...).PrepareForEval(ctx)
			if err != nil {
				t.Fatalf("failed to prepare: %v", err)
			}

			ndbc := builtins.NDBCache{}
			baselineOpts := []rego.EvalOption{rego.EvalQueryTracer(baseline), rego.EvalNDBuiltinCache(ndbc)}
			// One shared seed reader across all passes. The shared ndbc makes
			// the supplementary passes replay the baseline's cached value, so
			// they never draw from the reader and every pass takes the same
			// path. Without the ndbc each pass would spend the reader, draw a
			// different seed, and diverge.
			var seedReader io.Reader
			if tc.seeds != nil {
				seedReader = bytes.NewReader(seedBytes(tc.seeds...))
				baselineOpts = append(baselineOpts, rego.EvalSeed(seedReader))
			}
			if _, err := pq.Eval(ctx, baselineOpts...); err != nil {
				t.Fatalf("failed to evaluate: %v", err)
			}
			noIndexOpts := NoIndexingEvalOptions(noIndex, ndbc)
			noEarlyExitOpts := NoEarlyExitEvalOptions(noEarlyExit, ndbc)
			if seedReader != nil {
				noIndexOpts = append(noIndexOpts, rego.EvalSeed(seedReader))
				noEarlyExitOpts = append(noEarlyExitOpts, rego.EvalSeed(seedReader))
			}
			if _, err := pq.Eval(ctx, noIndexOpts...); err != nil {
				t.Fatalf("failed to evaluate (no indexing): %v", err)
			}
			if _, err := pq.Eval(ctx, noEarlyExitOpts...); err != nil {
				t.Fatalf("failed to evaluate (no early exit): %v", err)
			}

			reportKey := tc.reportKey
			if reportKey == "" {
				reportKey = "test.rego"
			}

			report := baseline.Report(map[string]*ast.Module{reportKey: parsedModule})
			fr, ok := report.Files[reportKey]
			if !ok {
				t.Fatalf("expected file report for %q", reportKey)
			}

			assertRanges(t, "covered", tc.covered, fr.Covered)
			assertRanges(t, "not_covered", tc.notCovered, fr.NotCovered)
		})
	}
}

// assertRanges compares an expected range list against the full list reported
// for a file, so both extents and membership are pinned.
func assertRanges(t *testing.T, label string, expected, actual []Range) {
	t.Helper()
	less := func(a, b Range) bool { return a.Compare(b) < 0 }
	if diff := cmp.Diff(expected, actual, cmpopts.SortSlices(less), cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("%s ranges mismatch (-expected +actual):\n%s", label, diff)
	}
}

func seedBytes(seeds ...int64) []byte {
	buf := make([]byte, 8*len(seeds))
	for i, s := range seeds {
		binary.BigEndian.PutUint64(buf[i*8:], uint64(s))
	}
	return buf
}

func TestCoverQueryTracerInterface(t *testing.T) {
	ct := topdown.QueryTracer(New())
	conf := ct.Config()
	expected := topdown.TraceConfig{
		PlugLocalVars: false,
	}

	if diff := cmp.Diff(expected, conf); diff != "" {
		t.Fatalf("Expected config: (-expected +actual)\n%s", diff)
	}
}
