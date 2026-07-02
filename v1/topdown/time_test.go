// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"testing"
	"time"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestTimeSeeding(t *testing.T) {
	t.Parallel()

	query := `time.now_ns(x)`
	clock := time.Now()
	q := NewQuery(ast.MustParseBody(query)).WithTime(clock).WithCompiler(ast.NewCompiler())

	ctx := t.Context()

	qrs, err := q.Run(ctx)
	if err != nil {
		t.Fatal(err)
	} else if len(qrs) != 1 {
		t.Fatal("expected exactly one result but got:", qrs)
	}

	exp := ast.MustParseTerm(fmt.Sprintf(`
		{
			{
				x: %v
			}
		}
	`, clock.UnixNano()))

	result := queryResultSetToTerm(qrs)

	if !result.Equal(exp) {
		t.Fatalf("expected %v but got %v", exp, result)
	}

}

func TestParseDurationNanos_BadInput(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expErr string
	}{
		{
			name:   "no known suffix",
			input:  `badinput`,
			expErr: "time: invalid duration \"badinput\"",
		},
		{
			name:   "bad digits with d suffix",
			input:  `badinputd`,
			expErr: "time: invalid duration \"badinputd\"",
		},
		{
			name:   "bad digits with w suffix",
			input:  `abcw`,
			expErr: "time: invalid duration \"abcw\"",
		},
		{
			name:   "bad digits with y suffix",
			input:  `xyz.y`,
			expErr: "time: invalid duration \"xyz.y\"",
		},
		{
			name:   "overflow days",
			input:  `99999999999d`,
			expErr: `time: invalid duration "99999999999d"`,
		},
		{
			name:   "overflow weeks",
			input:  `99999999999w`,
			expErr: `time: invalid duration "99999999999w"`,
		},
		{
			name:   "overflow years",
			input:  `99999999999y`,
			expErr: `time: invalid duration "99999999999y"`,
		},
		{
			name:   "invalid multi-unit",
			input:  `1d2x`,
			expErr: `time: unknown unit "x" in duration "1d2x"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := builtinParseDurationNanos(BuiltinContext{}, []*ast.Term{
				ast.StringTerm(tc.input),
			}, func(a *ast.Term) error {
				return nil
			})
			if err.Error() != tc.expErr {
				t.Fatalf("expected error %q but got %q", tc.expErr, err.Error())
			}
		})
	}
}

func TestParseDurationNanos_ExtendedUnits(t *testing.T) {
	tests := []struct {
		name  string
		input string
		expNs int64
	}{
		{
			name:  "fractional days",
			input: "1.5d",
			expNs: int64(36 * time.Hour),
		},
		{
			name:  "fractional weeks",
			input: "0.5w",
			expNs: int64(84 * time.Hour),
		},
		{
			name:  "negative days",
			input: "-1d",
			expNs: int64(-24 * time.Hour),
		},
		{
			name:  "zero days",
			input: "0d",
			expNs: 0,
		},
		{
			name:  "zero weeks",
			input: "0w",
			expNs: 0,
		},
		{
			name:  "zero years",
			input: "0y",
			expNs: 0,
		},
		{
			name:  "days and hours",
			input: "1d2h",
			expNs: int64(26 * time.Hour),
		},
		{
			name:  "hours and days",
			input: "2h1d",
			expNs: int64(26 * time.Hour),
		},
		{
			name:  "days hours minutes",
			input: "1d2h30m",
			expNs: int64(26*time.Hour + 30*time.Minute),
		},
		{
			name:  "weeks and days",
			input: "2w3d",
			expNs: int64((2*7*24 + 3*24) * time.Hour),
		},
		{
			name:  "days and seconds",
			input: "1d30s",
			expNs: int64(24*time.Hour + 30*time.Second),
		},
		{
			name:  "negative multi-unit",
			input: "-1d2h",
			expNs: int64(-26 * time.Hour),
		},
		{
			name:  "days and milliseconds",
			input: "1d100ms",
			expNs: int64(24*time.Hour + 100*time.Millisecond),
		},
		{
			name:  "days and nanoseconds",
			input: "1d500ns",
			expNs: int64(24*time.Hour + 500),
		},
		{
			name:  "days and microseconds",
			input: "1d200us",
			expNs: int64(24*time.Hour + 200*time.Microsecond),
		},
		{
			name:  "days and microseconds µs",
			input: "1d200µs",
			expNs: int64(24*time.Hour + 200*time.Microsecond),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got int64
			err := builtinParseDurationNanos(BuiltinContext{}, []*ast.Term{
				ast.StringTerm(tc.input),
			}, func(a *ast.Term) error {
				v, ok := a.Value.(ast.Number).Int64()
				if !ok {
					t.Fatal("expected int64 result")
				}
				got = v
				return nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expNs {
				t.Fatalf("expected %d but got %d", tc.expNs, got)
			}
		})
	}
}
