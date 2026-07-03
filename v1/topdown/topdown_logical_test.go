// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/types"
)

var touchCounters sync.Map // map[string]*atomic.Int64

func init() {
	ast.RegisterBuiltin(&ast.Builtin{
		Name: "test.touch",
		Decl: types.NewFunction(
			types.Args(types.S),
			types.B,
		),
	})

	RegisterBuiltinFunc("test.touch", func(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
		label := string(operands[0].Value.(ast.String))
		counter, _ := touchCounters.LoadOrStore(label, &atomic.Int64{})
		counter.(*atomic.Int64).Add(1)
		return iter(ast.BooleanTerm(true))
	})
}

func touchCount(label string) int {
	counter, ok := touchCounters.Load(label)
	if !ok {
		return 0
	}
	return int(counter.(*atomic.Int64).Load())
}

// logicalParserOptions opts in to the experimental `and` / `or` keywords.
func logicalParserOptions() ast.ParserOptions {
	return ast.ParserOptions{
		Capabilities:   ast.CapabilitiesForThisVersion(ast.CapabilitiesExperimentalKeywords(true)),
		FutureKeywords: []string{"and", "or"},
	}
}

func TestTopDownLogicalAnd(t *testing.T) {
	t.Parallel()

	n := func(ns ...string) []string { return ns }

	tests := []struct {
		note   string
		module string
		notes  []string
		fail   bool
	}{
		{
			note: "both succeed",
			module: `package test
				p if { 
					true and true
				}`,
		},
		{
			note: "lhs fails",
			module: `package test
				p if {
					false and true
				}`,
			fail: true,
		},
		{
			note: "rhs fails",
			module: `package test
				p if {
					true and false
				}`,
			fail: true,
		},
		{
			note: "lhs fails: rhs not evaluated (short-circuit)",
			module: `package test
				p if {
					false and print("rhs")
				}`,
			notes: n(),
			fail:  true,
		},
		{
			note: "lhs succeeds: rhs evaluated",
			module: `package test
				p if {
					print("lhs") and print("rhs")
				}`,
			notes: n("lhs", "rhs"),
		},
		{
			note: "explicit body operands",
			module: `package test
				p if {
					{x := 1; x > 0} and {y := 2; y > 0}
				}`,
		},
		{
			note: "explicit body: each operand has its own scope",
			module: `package test
				p if {
					{x := 1; x > 0} and {x := 2; x > 1}
				}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			runLogicalCase(t, tc.module, tc.notes, tc.fail)
		})
	}
}

func TestTopDownLogicalOr(t *testing.T) {
	t.Parallel()

	n := func(ns ...string) []string { return ns }

	tests := []struct {
		note   string
		module string
		notes  []string
		fail   bool
	}{
		{
			note: "both succeeds",
			module: `package test
				p if {
					true or true
				}`,
		},
		{
			note: "lhs succeeds",
			module: `package test
				p if {
					true or false
				}`,
		},
		{
			note: "lhs fails, rhs succeeds",
			module: `package test
				p if {
					false or true
				}`,
		},
		{
			note: "both fail",
			module: `package test
				p if {
					false or false
				}`,
			fail: true,
		},
		{
			note: "lhs succeeds: rhs not evaluated (short-circuit)",
			module: `package test
				p if {
					print("lhs") or print("rhs")
				}`,
			notes: n("lhs"),
		},
		{
			note: "lhs fails: rhs evaluated",
			module: `package test
				p if {
					false or print("rhs")
				}`,
			notes: n("rhs"),
		},
		{
			note: "explicit body operands",
			module: `package test
				p if {
					{x := 0; x > 0} or {y := 2; y > 0}
				}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			runLogicalCase(t, tc.module, tc.notes, tc.fail)
		})
	}
}

// TestTopDownLogicalOrSingleResult locks in the cardinality rule:
// `or` produces exactly one success even when both operands would succeed.
func TestTopDownLogicalOrSingleResult(t *testing.T) {
	t.Parallel()

	module := `package test
		q contains "x" if {
			true or true
		}`

	ctx := t.Context()
	c := ast.NewCompiler()
	mod := ast.MustParseModuleWithOpts(module, logicalParserOptions())
	c.Compile(map[string]*ast.Module{"test": mod})
	if c.Failed() {
		t.Fatal(c.Errors)
	}

	tr := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.q = x")).
		WithCompiler(c).
		WithQueryTracer(tr)

	res, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d: %v", len(res), res)
	}

	set, ok := res[0]["x"].Value.(ast.Set)
	if !ok {
		t.Fatalf("expected set value, got %T: %v", res[0]["x"].Value, res[0]["x"])
	}
	if set.Len() != 1 {
		t.Errorf("expected set of size 1 (single-result `or`), got %d: %v", set.Len(), set)
	}

	if t.Failed() || testing.Verbose() {
		PrettyTrace(os.Stderr, *tr)
	}
}

func runLogicalCase(t *testing.T, module string, expectedNotes []string, expectFail bool) {
	t.Helper()

	ctx := t.Context()
	c := ast.NewCompiler().WithEnablePrintStatements(true)
	mod := ast.MustParseModuleWithOpts(module, logicalParserOptions())
	c.Compile(map[string]*ast.Module{"test": mod})
	if c.Failed() {
		t.Fatal(c.Errors)
	}
	if testing.Verbose() {
		t.Log(c.Modules)
	}

	buf := bytes.Buffer{}
	tr := NewBufferTracer()
	ph := NewPrintHook(&buf)
	query := NewQuery(ast.MustParseBody("data.test.p = x")).
		WithCompiler(c).
		WithPrintHook(ph).
		WithQueryTracer(tr)

	res, err := query.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !expectFail {
		if len(res) == 0 {
			t.Errorf("unexpected failure, empty query result set")
		}
	} else {
		if len(res) > 0 {
			t.Errorf("unexpected results: %v, expected empty query result set", res)
		}
	}

	notes := strings.Split(buf.String(), "\n")
	notes = notes[:len(notes)-1] // last is empty after trailing "\n"
	if len(expectedNotes) != 0 || len(notes) != 0 {
		if !slices.Equal(notes, expectedNotes) {
			t.Errorf("unexpected prints, expected %q, got %q", expectedNotes, notes)
		}
	}

	if t.Failed() || testing.Verbose() {
		PrettyTrace(os.Stderr, *tr)
	}
}

func TestTopDownLogicalAndShortCircuit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note      string
		label     string // unique per case so subcases can run in parallel
		module    string
		wantTouch int
	}{
		{
			note:  "lhs false: rhs builtin not invoked",
			label: "and-shortcircuit-lhs-false",
			module: `package test
				import future.keywords.and
				p if {
					false and test.touch("and-shortcircuit-lhs-false")
				}`,
			wantTouch: 0,
		},
		{
			note:  "lhs true: rhs builtin invoked exactly once",
			label: "and-shortcircuit-lhs-true",
			module: `package test
				import future.keywords.and
				p if {
					true and test.touch("and-shortcircuit-lhs-true")
				}`,
			wantTouch: 1,
		},
		{
			note:  "lhs undefined ref: rhs builtin not invoked",
			label: "and-shortcircuit-lhs-undefined",
			module: `package test
				import future.keywords.and
				p if {
					input.missing and test.touch("and-shortcircuit-lhs-undefined")
				}`,
			wantTouch: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			runTouchCase(t, tc.label, tc.module, tc.wantTouch)
		})
	}
}

func TestTopDownLogicalOrShortCircuit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note      string
		label     string
		module    string
		wantTouch int
	}{
		{
			note:  "lhs true: rhs builtin not invoked",
			label: "or-shortcircuit-lhs-true",
			module: `package test
				import future.keywords.or
				p if {
					true or test.touch("or-shortcircuit-lhs-true")
				}`,
			wantTouch: 0,
		},
		{
			note:  "lhs false: rhs builtin invoked exactly once",
			label: "or-shortcircuit-lhs-false",
			module: `package test
				import future.keywords.or
				p if {
					false or test.touch("or-shortcircuit-lhs-false")
				}`,
			wantTouch: 1,
		},
		{
			note:  "lhs undefined ref: rhs builtin invoked exactly once",
			label: "or-shortcircuit-lhs-undefined",
			module: `package test
				import future.keywords.or
				p if {
					input.missing or test.touch("or-shortcircuit-lhs-undefined")
				}`,
			wantTouch: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()
			runTouchCase(t, tc.label, tc.module, tc.wantTouch)
		})
	}
}

func runTouchCase(t *testing.T, label, module string, wantTouch int) {
	t.Helper()

	ctx := t.Context()
	c := ast.NewCompiler()
	mod := ast.MustParseModuleWithOpts(module, logicalParserOptions())
	c.Compile(map[string]*ast.Module{"test": mod})
	if c.Failed() {
		t.Fatal(c.Errors)
	}

	tr := NewBufferTracer()
	query := NewQuery(ast.MustParseBody("data.test.p = x")).
		WithCompiler(c).
		WithQueryTracer(tr)

	if _, err := query.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := touchCount(label); got != wantTouch {
		t.Errorf("test.touch(%q) call count: got %d, want %d", label, got, wantTouch)
	}

	if t.Failed() || testing.Verbose() {
		PrettyTrace(os.Stderr, *tr)
	}
}
