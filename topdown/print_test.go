package topdown

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/topdown/print"
)

func TestTopDownPrint(t *testing.T) {

	cases := []struct {
		note   string
		module string
		exp    string
	}{
		{
			note: "empty",
			module: `
				package test

				p { print() }
			`,
			exp: "\n",
		},
		{
			note: "strings",
			module: `
				package test

				p {
					x := "world"
					print("hello", x)
				}
			`,
			exp: "hello world\n",
		},
		{
			note: "collections",
			module: `
				package test

				xs := [1,2]

				p {
					print("the value of xs is:", xs)
				}
			`,
			exp: "the value of xs is: [1, 2]\n",
		},
		{
			note: "undefined - does not affect rule evaluation and output contains marker",
			module: `
				package test

				p {
					print("the value of foo is:", input.foo)
				}
			`,
			exp: "the value of foo is: <undefined>\n",
		},
		{
			note: "undefined nested term does not affect rule evaluation and output contains marker",
			module: `
				package test

				p {
					print("the value of foo is:", [input.foo])
				}
			`,
			exp: "the value of foo is: <undefined>\n",
		},
		{
			note: "built-in error as undefined",
			module: `
				package test

				p {
					print("div by zero:", 1/0) # divide by zero will be undefined unless strict-builtin-errors are enabled
				}
			`,
			exp: "div by zero: <undefined>\n",
		},
		{
			note: "cross-product",
			module: `
				package test

				xs := {1}
				ys := {"a"}

				p {
					print(walk(xs), walk(ys))
				}
			`,
			exp: `[[], {1}] [[], {"a"}]
[[], {1}] [["a"], "a"]
[[1], 1] [[], {"a"}]
[[1], 1] [["a"], "a"]
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {

			c := ast.MustCompileModulesWithOpts(map[string]string{"test.rego": tc.module}, ast.CompileOpts{EnablePrintStatements: true})
			buf := bytes.NewBuffer(nil)
			q := NewQuery(ast.MustParseBody("data.test.p = x")).
				WithPrintHook(NewPrintHook(buf)).
				WithCompiler(c)

			qrs, err := q.Run(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			if buf.String() != tc.exp {
				t.Fatalf("expected: %q but got: %q", tc.exp, buf.String())
			}

			exp := ast.MustParseTerm(`{{x: true}}`)

			if !queryResultSetToTerm(qrs).Equal(exp) {
				t.Fatal("expected:", exp, "got:", qrs)
			}
		})
	}
}

func TestTopDownPrintInternalError(t *testing.T) {

	buf := bytes.NewBuffer(nil)

	q := NewQuery(ast.MustParseBody("internal.print([1])")).WithPrintHook(NewPrintHook(buf))

	_, err := q.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	asTopDownErr, ok := err.(*Error)
	if !ok {
		t.Fatal("expected topdown error but got:", err)
	} else if asTopDownErr.Code != InternalErr || asTopDownErr.Message != "illegal argument type: number" {
		t.Fatal("unexpected code or reason:", err)
	}
}

func TestTopDownPrintHookNotSupplied(t *testing.T) {

	// NOTE(tsandall): The built-in function implementation expects all inputs
	// to be _sets_, even scalar values are wrapped. This expectation comes from
	// the fact that the compiler rewrites all of the operands by wrapping them
	// in set comprehensions to avoid short-circuiting on undefined.
	q := NewQuery(ast.MustParseBody(`x = 1; internal.print({1})`))

	qrs, err := q.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	result := queryResultSetToTerm(qrs)
	exp := ast.MustParseTerm(`{{x: 1}}`)

	if result.Value.Compare(exp.Value) != 0 {
		t.Fatal("expected:", exp, "but got:", result)
	}
}

func TestTopDownPrintWithStrictBuiltinErrors(t *testing.T) {

	buf := bytes.NewBuffer(nil)

	// NOTE(tsandall): See comment above about wrapping operands in sets.
	q := NewQuery(ast.MustParseBody(`x = {1 | div(1, 0, y)}; internal.print([{"the value of 1/0 is:"}, x])`)).
		WithPrintHook(NewPrintHook(buf)).
		WithStrictBuiltinErrors(true).
		WithCompiler(ast.NewCompiler())

	_, err := q.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	asTopDownErr, ok := err.(*Error)
	if !ok {
		t.Fatal("expected topdown error but got:", err)
	} else if asTopDownErr.Code != BuiltinErr || asTopDownErr.Message != "div: divide by zero" {
		t.Fatal("unexpected code or reason:", err)
	}

	exp := "the value of 1/0 is: <undefined>\n"

	if buf.String() != exp {
		t.Fatalf("expected: %q but got: %q", exp, buf.String())
	}

}

type erroringPrintHook struct{}

func (erroringPrintHook) Print(print.Context, string) error {
	return errors.New("print hook error")
}

func TestTopDownPrintHookErrorPropagation(t *testing.T) {

	// NOTE(tsandall): See comment above about wrapping operands in sets.
	q := NewQuery(ast.MustParseBody(`internal.print([{"some message"}])`)).
		WithPrintHook(erroringPrintHook{}).
		WithStrictBuiltinErrors(true).
		WithCompiler(ast.NewCompiler())

	_, err := q.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	} else if !strings.Contains(err.Error(), "print hook error") {
		t.Fatal("expected print hook error but got:", err)
	}

}
