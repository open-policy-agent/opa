package topdown_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

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
