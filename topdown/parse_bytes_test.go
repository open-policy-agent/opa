// Copyright 2016 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestNumBytes(t *testing.T) {
	t.Run("SuccessfulParse", func(t *testing.T) {
		tests := []struct {
			note, rule string
			expected   int64
		}{
			{"zero", `0`, 0},
			{"raw number", `12345`, 12345},
			{"10 kilobytes uppercase", `10KB`, 10 * kb},
			{"10 KiB uppercase", `10KIB`, 10 * ki},
			{"10 KB lowercase", `10kb`, 10 * kb},
			{"10 KiB mixed case", `10Kib`, 10 * ki},
			{"200 megabytes as mb", `200mb`, 200 * mb},
			{"300 GiB", `300GiB`, 300 * gi},
			{"100 kilobytes as k", `100k`, 100 * kb},
			{"100 kilobytes as kb", `100kb`, 100 * kb},
			{"100 kibibytes as ki", `100ki`, 100 * ki},
			{"100 kibibytes as kib", `100kib`, 100 * ki},
			{"100 megabytes as m", `100m`, 100 * mb},
			{"100 megabytes as mb", `100mb`, 100 * mb},
			{"100 mebibytes as mi", `100mi`, 100 * mi},
			{"100 mebibytes as mib", `100mib`, 100 * mi},
			{"100 gigabytes as g", `100g`, 100 * gb},
			{"100 gigabytes as gb", `100gb`, 100 * gb},
			{"100 gibibytes as gi", `100gi`, 100 * gi},
			{"100 gibibytes as gib", `100gib`, 100 * gi},
			{"100 terabytes as t", `100t`, 100 * tb},
			{"100 terabytes as tb", `100tb`, 100 * tb},
			{"100 tebibytes as ti", `100ti`, 100 * ti},
			{"100 tebibytes as tib", `100tib`, 100 * ti},
		}

		for _, tc := range tests {
			runNumBytesParseTest(t, tc.note, tc.rule, tc.expected)
		}
	})

	t.Run("Compare", func(t *testing.T) {
		tests := []struct {
			rule1 string
			rule2 string
			op    string
		}{
			{"8kb", `7kb`, ">"},
			{"8gb", `8mb`, ">"},
			{"1234kb", "1gb", "<"},
			{"1024", "1KiB", "=="},
			{"2MiB", "2097152", "=="},
			{"3MiB", "3MB", ">"},
			{"2MiB", "2Mi", "=="},
			{"4Mi", "4M", ">"},
			{"128Gi", "137438953472", "=="},
		}

		for _, tc := range tests {
			runComparisonTest(t, tc.rule1, tc.rule2, tc.op)
		}
	})

	t.Run("ExpectFailure", func(t *testing.T) {
		tests := []struct {
			rule        string
			expectedErr error
		}{
			{"", errNoAmount},
			{"GB", errNoAmount},
			{"foo", errNoAmount},
			{"10.25", errIntConv},
			{"0.00", errIntConv},
			{"100.1GB", errIntConv},
			{"100 kb", errIncludesSpaces},
			{" 327MiB ", errIncludesSpaces},
		}

		for _, tc := range tests {
			runExpectedFailureTest(t, tc.rule, tc.expectedErr)
		}
	})
}

func runNumBytesParseTest(t *testing.T, note, rule string, expected int64) {
	t.Helper()

	num := parseIntFromString(t, rule)

	if num != expected {
		t.Fatalf(`numbytes failure on "%s": expected value %d does not match %d`, note, expected, num)
	}
}

func runComparisonTest(t *testing.T, rule1, rule2, op string) {
	t.Helper()

	val1, val2 := parseIntFromString(t, rule1), parseIntFromString(t, rule2)

	assertComparisonSucceeds := func(t *testing.T, exp bool, op string) {
		if !exp {
			t.Fatalf("numbytes err: unexpected return on %s %s %s", rule1, op, rule2)
		}
	}

	switch op {
	case "==":
		assertComparisonSucceeds(t, val1 == val2, op)
	case ">":
		assertComparisonSucceeds(t, val1 > val2, op)
	case "<":
		assertComparisonSucceeds(t, val1 < val2, op)
	default:
		t.Fatalf("unexpected input to comparison test: %s", op)
	}
}

func runExpectedFailureTest(t *testing.T, s string, expectedErr error) {
	sVal := ast.StringTerm(s).Value
	val, err := builtinNumBytes(sVal)

	if val != nil {
		t.Fatal(`numbytes err: expected returned value to be nil`)
	}

	if err == nil {
		t.Fatalf(`numbytes err: test rule %s should return error`, s)
	}

	if err.Error() != expectedErr.Error() {
		t.Fatalf(`numbytes err: test rule %s should produce error %v but got %v`, s, expectedErr, err)
	}
}

func parseIntFromString(t *testing.T, s string) int64 {
	sVal := ast.StringTerm(s).Value
	val, err := builtinNumBytes(sVal)

	if err != nil {
		t.Fatalf(`numbytes err: could not parse "%s" into int: %v`, s, err)
	}

	i := val.(ast.Number)
	num, ok := i.Int64()
	if !ok {
		t.Fatalf("numbytes err: could not parse value %s into int", val.String())
	}

	return num
}
