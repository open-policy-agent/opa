// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"slices"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/storage"
	inmem "github.com/open-policy-agent/opa/v1/storage/inmem/test"
)

const stackTraceTestModule = `package ex

p contains x if {
	q[x]
}

q contains x if {
	r[x]
}

r contains x if {
	x := 1 / 0
}

comprehension if {
	xs := [y | y := 1 / 0]
	xs != []
}

every_expr if {
	every y in [1] {
		y == 1 / 0
	}
}

with_expr if {
	q with input as {}
}

fn(x) if {
	x / 0
}

call_fn if {
	fn(1)
}

conflicting := 1

conflicting := 2

read_conflicting if {
	conflicting
}

short_p if short_q

short_q if 1 / 0

call_fn_var if {
	x0 := 1
	fn(x0)
}

reciprocals contains y if {
	some x in {2, 1, 0}
	y := reciprocal(x)
}

reciprocal(x) := 1 / x
`

func TestStackTraceFrames(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note     string
		query    string
		expected []string
	}{
		{
			note:  "rule chain",
			query: "data.ex.p",
			expected: []string{
				"stack.rego:12: 1 / 0",
				"stack.rego:8: r[x]",
				"stack.rego:4: q[x]",
				"1:1: data.ex.p",
			},
		},
		{
			note:  "comprehension body",
			query: "data.ex.comprehension",
			expected: []string{
				"stack.rego:16: 1 / 0",
				"stack.rego:16: xs := [y | y := 1 / 0]",
				"1:1: data.ex.comprehension",
			},
		},
		{
			note:  "every body",
			query: "data.ex.every_expr",
			expected: []string{
				"stack.rego:22: 1 / 0",
				"stack.rego:21: [1]", // the query enumerating the domain
				"stack.rego:21: every y in [1] { y == 1 / 0 }",
				"1:1: data.ex.every_expr",
			},
		},
		{
			note:  "with statement",
			query: "data.ex.with_expr",
			expected: []string{
				"stack.rego:12: 1 / 0",
				"stack.rego:8: r[x]",
				"stack.rego:27: q with input as {}",
				"1:1: data.ex.with_expr",
			},
		},
		{
			note:  "function call",
			query: "data.ex.call_fn",
			expected: []string{
				"stack.rego:31: x / 0",
				"stack.rego:35: fn(1)",
				"1:1: data.ex.call_fn",
			},
		},
		{
			note:  "conflicting complete rules",
			query: "data.ex.read_conflicting",
			expected: []string{
				"stack.rego:40: 2",
				"stack.rego:43: conflicting",
				"1:1: data.ex.read_conflicting",
			},
		},
		{
			note:  "rules written without a braced body",
			query: "data.ex.short_p",
			expected: []string{
				"stack.rego:48: 1 / 0",
				"stack.rego:46: short_q",
				"1:1: data.ex.short_p",
			},
		},
		{
			// A frame quotes the source, so the argument shows as the variable
			// the policy passed, not the 1 it was bound to.
			note:  "function called with a variable",
			query: "data.ex.call_fn_var",
			expected: []string{
				"stack.rego:31: x / 0",
				"stack.rego:52: fn(x0)",
				"1:1: data.ex.call_fn_var",
			},
		},
		{
			// Likewise, the frame doesn't say which x of the domain failed.
			note:  "function called for each element of a domain",
			query: "data.ex.reciprocals",
			expected: []string{
				"stack.rego:60: 1 / x",
				"stack.rego:57: reciprocal(x)",
				"1:1: data.ex.reciprocals",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			err := runStackTraceQuery(t, tc.query, true)
			tdErr, ok := err.(*Error)
			if !ok {
				t.Fatalf("expected *Error, got %v (%[1]T)", err)
			}

			frames := make([]string, 0, len(tdErr.StackTrace))
			for _, f := range tdErr.StackTrace {
				frames = append(frames, f.String())
			}

			if !slices.Equal(frames, tc.expected) {
				t.Fatalf("expected frames\n%s\n\ngot\n%s", strings.Join(tc.expected, "\n"), strings.Join(frames, "\n"))
			}
		})
	}
}

func TestStackTraceQueryIDsMatchParentChain(t *testing.T) {
	t.Parallel()

	err := runStackTraceQuery(t, "data.ex.p", true)
	tdErr := err.(*Error)

	if len(tdErr.StackTrace) != 4 {
		t.Fatalf("expected 4 frames, got %d", len(tdErr.StackTrace))
	}

	// Query IDs are handed out in creation order, so an enclosing query always
	// has the lower ID.
	for i := range len(tdErr.StackTrace) - 1 {
		if inner, outer := tdErr.StackTrace[i].QueryID, tdErr.StackTrace[i+1].QueryID; inner <= outer {
			t.Fatalf("expected frame %d (query %d) to be nested in frame %d (query %d)", i, inner, i+1, outer)
		}
	}
}

func TestStackTraceDisabledByDefault(t *testing.T) {
	t.Parallel()

	err := runStackTraceQuery(t, "data.ex.p", false)
	tdErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %v (%[1]T)", err)
	}

	if tdErr.StackTrace != nil {
		t.Fatalf("expected no stack trace, got %v", tdErr.StackTrace)
	}
}

func TestStackTraceNotInErrorMessage(t *testing.T) {
	t.Parallel()

	withTrace := runStackTraceQuery(t, "data.ex.p", true)
	withoutTrace := runStackTraceQuery(t, "data.ex.p", false)

	if withTrace.Error() != withoutTrace.Error() {
		t.Fatalf("expected enabling stack traces to leave the message unchanged, got %q and %q",
			withTrace.Error(), withoutTrace.Error())
	}
}

func TestStackTraceOnCollectedBuiltinErrors(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	compiler := compileStackTraceModule(t)
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	var errs []Error
	q := NewQuery(ast.MustParseBody("data.ex.p")).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithBuiltinErrorList(&errs).
		WithStackTraces(true)

	if _, err := q.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(errs) != 1 {
		t.Fatalf("expected 1 built-in error, got %d", len(errs))
	}

	expected := []string{
		"stack.rego:12: 1 / 0",
		"stack.rego:8: r[x]",
		"stack.rego:4: q[x]",
		"1:1: data.ex.p",
	}

	frames := make([]string, 0, len(errs[0].StackTrace))
	for _, f := range errs[0].StackTrace {
		frames = append(frames, f.String())
	}

	if !slices.Equal(frames, expected) {
		t.Fatalf("expected frames\n%s\n\ngot\n%s", strings.Join(expected, "\n"), strings.Join(frames, "\n"))
	}
}

func TestStackTraceDoesNotMutateSharedErrors(t *testing.T) {
	t.Parallel()

	// errInScopeWithStmt is a package-level value returned by resolverTrie, so
	// annotating it in place would leak one query's stack into every later
	// occurrence of the error, process-wide.
	shared := errInScopeWithStmt

	e := &eval{stackTraces: true, queryID: 1}
	annotated := e.withStackTrace(shared)

	if shared.StackTrace != nil {
		t.Fatalf("expected the shared error to be untouched, got %v", shared.StackTrace)
	}
	if annotated == error(shared) {
		t.Fatal("expected a copy, got the shared error itself")
	}

	tdErr, ok := annotated.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %v (%[1]T)", annotated)
	}
	if tdErr.StackTrace == nil {
		t.Fatal("expected the copy to carry a stack trace")
	}
	if tdErr.Code != shared.Code || tdErr.Message != shared.Message {
		t.Fatalf("copy lost fields: %+v", tdErr)
	}
}

func TestStackTraceString(t *testing.T) {
	t.Parallel()

	st := StackTrace{
		{QueryID: 2, Location: ast.NewLocation([]byte("1 / 0"), "test.rego", 12, 7)},
		{QueryID: 1, Location: ast.NewLocation([]byte("data.ex.p"), "", 1, 1)},
		{QueryID: 0},
	}

	expected := `  test.rego:12: 1 / 0
  1:1: data.ex.p
  <unknown>`

	if actual := st.String(); actual != expected {
		t.Fatalf("expected\n%s\n\ngot\n%s", expected, actual)
	}

	if actual := (StackTrace{}).String(); actual != "" {
		t.Fatalf("expected empty string for empty trace, got %q", actual)
	}
}

func TestFrameText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note     string
		src      string
		expected string
	}{
		{note: "empty", src: "", expected: ""},
		{note: "whitespace only", src: " \n\t ", expected: ""},
		{note: "single line", src: "x := 1 / 0", expected: "x := 1 / 0"},
		{note: "leading and trailing space", src: "\tx := 1\n", expected: "x := 1"},
		{note: "multi line collapsed", src: "every y in [1] {\n\ty == 1\n}", expected: "every y in [1] { y == 1 }"},
		{
			note:     "truncated",
			src:      strings.Repeat("a", maxStackFrameTextLength+10),
			expected: strings.Repeat("a", maxStackFrameTextLength) + "...",
		},
		{
			note:     "exact length not truncated",
			src:      strings.Repeat("a", maxStackFrameTextLength),
			expected: strings.Repeat("a", maxStackFrameTextLength),
		},
		{note: "multi byte runes counted as one", src: "x == \"日本語\"", expected: `x == "日本語"`},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			if actual := frameText([]byte(tc.src)); actual != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, actual)
			}
		})
	}
}

func runStackTraceQuery(t *testing.T, query string, stackTraces bool) error {
	t.Helper()

	ctx := t.Context()
	compiler := compileStackTraceModule(t)
	store := inmem.New()
	txn := storage.NewTransactionOrDie(ctx, store)
	defer store.Abort(ctx, txn)

	q := NewQuery(ast.MustParseBody(query)).
		WithCompiler(compiler).
		WithStore(store).
		WithTransaction(txn).
		WithStrictBuiltinErrors(true).
		WithStackTraces(stackTraces)

	_, err := q.Run(ctx)
	if err == nil {
		t.Fatalf("expected error evaluating %v", query)
	}

	return err
}

func compileStackTraceModule(t *testing.T) *ast.Compiler {
	t.Helper()

	module, err := ast.ParseModuleWithOpts("stack.rego", stackTraceTestModule, ast.ParserOptions{})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	c := ast.NewCompiler()
	if c.Compile(map[string]*ast.Module{"stack.rego": module}); c.Failed() {
		t.Fatalf("unexpected compile error: %v", c.Errors)
	}

	return c
}
