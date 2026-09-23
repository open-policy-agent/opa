package topdown_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func TestErrorWrapping(t *testing.T) {
	t.Parallel()

	isHalt := func(err error) bool {
		_, ok := errors.AsType[topdown.Halt](err)
		return ok
	}

	builtinErr := errors.New("builtin error")
	loc := location.Location{
		File: "b.rego",
		Col:  10,
		Row:  12,
	}

	e0 := (&topdown.Error{Code: topdown.BuiltinErr,
		Message:  "builtin error",
		Location: &loc,
	}).Wrap(builtinErr)

	tests := []struct {
		note  string
		err   error
		check func(error) bool
	}{
		{
			note:  "plain",
			err:   &topdown.Error{},
			check: topdown.IsError,
		},
		{
			note:  "wrapped",
			err:   fmt.Errorf("meh: %w", &topdown.Error{}),
			check: topdown.IsError,
		},
		{
			note:  "wrapped in Halt",
			err:   topdown.Halt{Err: &topdown.Error{}},
			check: topdown.IsError,
		},
		{
			note:  "check for Halt",
			err:   topdown.Halt{Err: &topdown.Error{}},
			check: isHalt,
		},
		{
			note:  "check for Halt, wrapped",
			err:   fmt.Errorf("meh: %w", topdown.Halt{Err: &topdown.Error{}}),
			check: isHalt,
		},
		{
			note:  "plain cancel",
			err:   &topdown.Error{Code: topdown.CancelErr},
			check: topdown.IsCancel,
		},
		{
			note:  "wrapped cancel",
			err:   fmt.Errorf("meh: %w", &topdown.Error{Code: topdown.CancelErr}),
			check: topdown.IsCancel,
		},
		{
			note: "wrapped builtin error",
			err:  e0,
			check: func(err error) bool {
				return errors.Is(err, builtinErr)
			},
		},
		{
			note: "matching errors, code",
			err:  e0,
			check: func(err error) bool {
				return errors.Is(err, &topdown.Error{Code: topdown.BuiltinErr})
			},
		},
		{
			note: "matching errors, code and message",
			err:  e0,
			check: func(err error) bool {
				return errors.Is(err, &topdown.Error{Code: topdown.BuiltinErr, Message: "builtin error"})
			},
		},
		{
			note: "matching errors, code, message and location",
			err:  e0,
			check: func(err error) bool {
				return errors.Is(err, &topdown.Error{Code: topdown.BuiltinErr, Message: "builtin error", Location: &loc})
			},
		},
		{
			note: "matching errors, code, message, location and builtin error",
			err:  e0,
			check: func(err error) bool {
				return errors.Is(err, e0)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			if !tc.check(tc.err) {
				t.Error("unexpected 'false'")
			}
		})
	}
}

func TestErrorMarshalJSON(t *testing.T) {
	t.Parallel()

	loc := &location.Location{File: "b.rego", Row: 12, Col: 10}

	tests := []struct {
		note string
		err  *topdown.Error
		exp  string
	}{
		{
			note: "no stack trace",
			err:  &topdown.Error{Code: topdown.BuiltinErr, Message: "div: divide by zero", Location: loc},
			exp:  `{"code":"eval_builtin_error","message":"div: divide by zero","location":{"file":"b.rego","row":12,"col":10}}`,
		},
		{
			note: "no location",
			err:  &topdown.Error{Code: topdown.BuiltinErr, Message: "div: divide by zero"},
			exp:  `{"code":"eval_builtin_error","message":"div: divide by zero"}`,
		},
		{
			note: "stack trace",
			err: &topdown.Error{
				Code:     topdown.BuiltinErr,
				Message:  "div: divide by zero",
				Location: loc,
				StackTrace: topdown.StackTrace{
					{QueryID: 1, Location: loc},
					{QueryID: 0},
				},
			},
			exp: `{"code":"eval_builtin_error","message":"div: divide by zero","location":{"file":"b.rego","row":12,"col":10},` +
				`"stack_trace":[{"query_id":1,"location":{"file":"b.rego","row":12,"col":10}},{"query_id":0}]}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			bs, err := json.Marshal(tc.err)
			if err != nil {
				t.Fatal(err)
			}

			if got := string(bs); got != tc.exp {
				t.Errorf("expected %s, got %s", tc.exp, got)
			}
		})
	}
}

// 101.7 ns/op	     144 B/op	       5 allocs/op // using fmt.Sprintf
// 18.78 ns/op	      48 B/op	       1 allocs/op // using []byte + Location.AppendText
func BenchmarkErrorError(b *testing.B) {
	loc := &location.Location{
		File: "b.rego",
		Col:  10,
		Row:  12,
	}
	err := &topdown.Error{
		Code:     topdown.BuiltinErr,
		Message:  "builtin error",
		Location: loc,
	}

	for b.Loop() {
		_ = err.Error()
	}
}

func TestConflictErrorListsConflictingRules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		note    string
		modules map[string]string
		query   string
		input   string
		strict  bool
		message string
	}{
		{
			note: "complete rules",
			modules: map[string]string{"policy.rego": `package ex

allow := "one" if input.a

allow := "two" if input.b

allow := "three" if input.c`},
			query: "data.ex.allow",
			input: `{"a": true, "b": true}`,
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5",
		},
		{
			note: "complete rules, all conflicting rules",
			modules: map[string]string{"policy.rego": `package ex

allow := "one" if input.a

allow := "two" if input.b

allow := "three" if input.c`},
			query: "data.ex.allow",
			input: `{"a": true, "b": true, "c": true}`,
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5\n" +
				"  rule at policy.rego:7",
		},
		{
			note:    "complete rules, limit",
			modules: map[string]string{"policy.rego": distinctCompleteRules(12)},
			query:   "data.ex.allow",
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5\n" +
				"  rule at policy.rego:7\n" +
				"  rule at policy.rego:9\n" +
				"  rule at policy.rego:11\n" +
				"  rule at policy.rego:13\n" +
				"  rule at policy.rego:15\n" +
				"  rule at policy.rego:17\n" +
				"  rule at policy.rego:19\n" +
				"  rule at policy.rego:21\n" +
				"  ...",
		},
		{
			note: "complete rules, error after conflict",
			modules: map[string]string{"policy.rego": `package ex

allow := 1

allow := 2

allow := 1 / 0`},
			query:  "data.ex.allow",
			strict: true,
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5",
		},
		{
			note: "complete rules, error after conflict, non-strict",
			modules: map[string]string{"policy.rego": `package ex

allow := 1

allow := 2

allow := 1 / 0`},
			query: "data.ex.allow",
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5",
		},
		{
			note: "complete rules across files",
			modules: map[string]string{
				"a.rego": "package ex\n\nallow := 1",
				"b.rego": "package ex\n\nallow := 2",
			},
			query: "data.ex.allow",
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at a.rego:3\n" +
				"  rule at b.rego:3",
		},
		{
			note: "complete rules, else branch",
			modules: map[string]string{"policy.rego": `package ex

allow := 1 if input.a
else := 2

allow := 3 if input.b`},
			query: "data.ex.allow",
			input: `{"b": true}`,
			message: "rule data.ex.allow produced conflicting values:\n" +
				"  rule at policy.rego:4\n" +
				"  rule at policy.rego:6",
		},
		{
			note: "complete rule, single rule",
			modules: map[string]string{"policy.rego": `package ex

allow := x if some x in [1, 2]`},
			query:   "data.ex.allow",
			message: "rule data.ex.allow produced conflicting values",
		},
		{
			note: "functions",
			modules: map[string]string{"policy.rego": `package ex

f(x) := 1 if x > 0

f(x) := 2 if x > 1`},
			query: "data.ex.f(2)",
			message: "function data.ex.f produced conflicting values for the same inputs:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5",
		},
		{
			note: "functions, all conflicting rules",
			modules: map[string]string{"policy.rego": `package ex

f(x) := 1 if x > 0

f(x) := 2 if x > 1

f(x) := 3 if x > 2`},
			query: "data.ex.f(3)",
			message: "function data.ex.f produced conflicting values for the same inputs:\n" +
				"  rule at policy.rego:3\n" +
				"  rule at policy.rego:5\n" +
				"  rule at policy.rego:7",
		},
		{
			note: "functions, single rule",
			modules: map[string]string{"policy.rego": `package ex

f(_) := x if some x in [1, 2]`},
			query:   "data.ex.f(2)",
			message: "function data.ex.f produced conflicting values for the same inputs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			q := topdown.NewQuery(ast.MustParseBody(tc.query)).
				WithCompiler(ast.MustCompileModules(tc.modules)).
				WithStrictBuiltinErrors(tc.strict)
			var builtinErrs []topdown.Error
			if !tc.strict {
				q = q.WithBuiltinErrorList(&builtinErrs)
			}
			if tc.input != "" {
				q = q.WithInput(ast.MustParseTerm(tc.input))
			}

			_, err := q.Run(t.Context())
			tdErr, ok := errors.AsType[*topdown.Error](err)
			if !ok {
				t.Fatalf("expected topdown error, got: %v", err)
			}
			if tdErr.Code != topdown.ConflictErr {
				t.Fatalf("expected code %q, got %q", topdown.ConflictErr, tdErr.Code)
			}
			if tdErr.Message != tc.message {
				t.Fatalf("expected message:\n%s\ngot:\n%s", tc.message, tdErr.Message)
			}
			if len(builtinErrs) > 0 {
				t.Fatalf("expected no built-in errors, got: %v", builtinErrs)
			}
		})
	}
}

func distinctCompleteRules(n int) string {
	s := strings.Builder{}
	s.WriteString("package ex\n")
	for i := range n {
		fmt.Fprintf(&s, "\nallow := %d\n", i)
	}
	return s.String()
}
