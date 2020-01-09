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
			expected   int
		}{
			{"zero", `0`, 0},
			{"raw number", `12345`, 12345},
			{"10 kilobytes uppercase", `10KB`, 10 * kb},
			{"10 KiB uppercase", `10KIB`, 10 * ki},
			{"10 KB lowercase", `10kb`, 10 * kb},
			{"10 KiB mixed case", `10Kib`, 10 * ki},
			{"200 megabytes as mb", `200mb`, 200 * mb},
			{"300 GiB", `300GiB`, 300 * gi},
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
			{"GB", errNoAmount},
			{"foo", errNoAmount},
			{"10.25", errIntConv},
			{"0.00", errIntConv},
			{"100.1GB", errIntConv},
			{"8g", errUnitNotRecognized("g")},
			{"8m", errUnitNotRecognized("m")},
			{"100 kb", errIncludesSpaces},
			{" 327MiB ", errIncludesSpaces},
		}

		for _, tc := range tests {
			runExpectedFailureTest(t, tc.rule, tc.expectedErr)
		}
	})
}

func runNumBytesParseTest(t *testing.T, note, rule string, expected int) {
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

func parseIntFromString(t *testing.T, s string) int {
	sVal := ast.StringTerm(s).Value
	val, err := builtinNumBytes(sVal)

	if err != nil {
		t.Fatalf(`numbytes err: could not parse "%s" into int: %v`, s, err)
	}

	i := val.(ast.Number)
	num, ok := i.Int()
	if !ok {
		t.Fatalf("numbytes err: could not parse value %s into int", val.String())
	}

	return num
}

func TestCompareMemory(t *testing.T) {

	t.Run("SuccessfulCompare", func(t *testing.T) {
		tests := []struct {
			note, first, second string
			expected            int
		}{
			{"zero", `0`, `0`, 0},
			{"zeroNoNumber", `Gi`, `G`, 0},
			{"zeroNoNumberCrossUnit", `Gi`, `k`, 0},
			{"RawNumber", `12345`, `2`, 1},
			{"RawNumberFloat", `10.25`, `10.25`, 0},
			{"RawNumberFloatTwo", `10.25M`, `10.25`, 1},
			{"RawNumberScientificGreater", `12345`, `2e3`, 1},
			{"RawNumberScientificLess", `123`, `2e3`, -1},
			{"RawNumberScientificEqual", `2000`, `2e3`, 0},
			{"CrossUnitLess", `123Mi`, `1G`, -1},
			{"CrossUnitGreaterOne", `1235Mi`, `1G`, 1},
			{"CrossUnitGreaterTwo", `1025Mi`, `1G`, 1},
			{"CrossUnitGreaterThree", `1G`, `999M`, 1},
			{"CrossUnitSameOne", `1024`, `1Ki`, 0},
			{"CrossUnitSameTwo", `1000`, `1k`, 0},
			{"CrossUnitSameThree", `1024Mi`, `1Gi`, 0},
			{"CrossUnitSameFour", `1024Pi`, `1Ei`, 0},
			{"CrossUnitSameFive", `129M`, `129e6`, 0},
			{"CrossUnitSameSix", `128974848`, `123Mi`, 0},
			{"BigNumber", `1000E`, `10000E`, -1},
			{"BigNumberTwo", `1e234`, `1e235`, -1},
		}

		for _, tc := range tests {
			runCompareMemoryTest(t, tc.note, tc.first, tc.second, tc.expected)
		}
	})

	t.Run("ExpectFailure", func(t *testing.T) {
		tests := []struct {
			first, second string
			expectedErr   error
		}{
			{"xGi", "123", errParsingMemory},
			{"foo", "123", errParsingMemory},
			{"123", "xGi", errParsingMemory},
			{"123", "foo", errParsingMemory},
		}

		for _, tc := range tests {
			runCompareMemoryFailureTest(t, tc.first, tc.second, tc.expectedErr)
		}
	})
}

func runCompareMemoryFailureTest(t *testing.T, first string, second string, expectedErr error) {
	firstVal := ast.StringTerm(first).Value
	secondVal := ast.StringTerm(second).Value
	res, err := builtinCompareMemory(firstVal, secondVal)

	if res != nil {
		t.Fatalf(`compare memory err: expected returned value for %s and %s to be nil`, first, second)
	}

	if err == nil {
		t.Fatalf(`compare memory err: compare %s and %s should return error`, first, second)
	}

	if err.Error() != expectedErr.Error() {
		t.Fatalf(`compare memory err: compare %s and %s should produce error %v but got %v`, first, second, expectedErr, err)
	}
}

func runCompareMemoryTest(t *testing.T, note, first string, second string, expected int) {
	t.Helper()

	num := compareMemory(t, first, second)

	if num != expected {
		t.Fatalf(`compare memory failure on "%s": expected value %d, but got %d`, note, expected, num)
	}
}

func compareMemory(t *testing.T, first string, second string) int {
	firstVal := ast.StringTerm(first).Value
	secondVal := ast.StringTerm(second).Value
	res, err := builtinCompareMemory(firstVal, secondVal)

	if err != nil {
		t.Fatalf(`compare memory err: could not parse "%s" or "%s" into quantity: %v`, firstVal, secondVal, err)
	}

	i := res.(ast.Number)
	num, ok := i.Int()
	if !ok {
		t.Fatalf("compare memory err: could not parse value %s into int", res.String())
	}

	return num
}
