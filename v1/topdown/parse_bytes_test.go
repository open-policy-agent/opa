package topdown

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestBuiltinNumBytesTimeout(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		timeout     time.Duration
		expectedErr string
	}{
		{
			name:        "timeout due to large number",
			input:       "10000000000e100000000EiB",
			timeout:     time.Second,
			expectedErr: "context deadline exceeded",
		},
		{
			name:    "no timeout",
			input:   "10000000000e10000EiB",
			timeout: time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			iter := func(*ast.Term) error { return nil }
			ctx := BuiltinContext{
				Context: t.Context(),
			}

			if tc.timeout != 0 {
				var cancel func()
				ctx.Context, cancel = context.WithTimeout(ctx.Context, tc.timeout)
				defer cancel()
			}

			operands := []*ast.Term{
				ast.StringTerm(tc.input),
			}

			err := builtinNumBytes(ctx, operands, iter)
			if tc.expectedErr != "" && err == nil {
				t.Fatalf("expected error %s, got nil", tc.expectedErr)
			}
			if err != nil && !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("expected error %s, got %s", tc.expectedErr, err)
			}
		})
	}

}
