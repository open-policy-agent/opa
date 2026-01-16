package topdown_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/topdown"
)

func TestErrorWrapping(t *testing.T) {
	t.Parallel()

	isHalt := func(err error) bool {
		return errors.As(err, &topdown.Halt{})
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
				t.Errorf("unexpected 'false'")
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
