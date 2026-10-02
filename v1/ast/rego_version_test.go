package ast

import (
	"strconv"
	"testing"
)

func TestRegoVersion_IntString(t *testing.T) {
	tests := []struct {
		v      RegoVersion
		expInt int
		expStr string
	}{
		{v: RegoUndefined, expInt: 0, expStr: "unknown"},
		{v: RegoV0, expInt: 0, expStr: "v0"},
		{v: RegoV0CompatV1, expInt: 0, expStr: "v0v1"},
		{v: RegoV1, expInt: 1, expStr: "v1"},
		{v: RegoV2, expInt: 2, expStr: "v2"},
		{v: RegoVersion(255), expInt: 0, expStr: "unknown"},
	}

	for _, tc := range tests {
		t.Run(strconv.Itoa(int(tc.v)), func(t *testing.T) {
			if act := tc.v.Int(); act != tc.expInt {
				t.Errorf("Int(): expected %d, got %d", tc.expInt, act)
			}
			if act := tc.v.String(); act != tc.expStr {
				t.Errorf("String(): expected %q, got %q", tc.expStr, act)
			}
		})
	}
}

func TestRegoVersionFromInt(t *testing.T) {
	tests := []struct {
		i   int
		exp RegoVersion
	}{
		{i: -1, exp: RegoV0},
		{i: 0, exp: RegoV0},
		{i: 1, exp: RegoV1},
		{i: 2, exp: RegoV2},
		{i: 3, exp: RegoV0},
	}

	for _, tc := range tests {
		if act := RegoVersionFromInt(tc.i); act != tc.exp {
			t.Errorf("RegoVersionFromInt(%d): expected %s, got %s", tc.i, tc.exp, act)
		}
	}
}

type expSpan struct {
	min, max  RegoVersion
	str       string
	legacy    RegoVersion
	i         int
	undefined bool
}

func assertSpan(t *testing.T, s RegoVersionSpan, exp expSpan) {
	t.Helper()

	if s.Min() != exp.min {
		t.Errorf("Min(): expected %s, got %s", exp.min, s.Min())
	}
	if s.Max() != exp.max {
		t.Errorf("Max(): expected %s, got %s", exp.max, s.Max())
	}
	if s.String() != exp.str {
		t.Errorf("String(): expected %q, got %q", exp.str, s.String())
	}
	if s.RegoVersion() != exp.legacy {
		t.Errorf("RegoVersion(): expected %s, got %s", exp.legacy, s.RegoVersion())
	}
	if s.Int() != exp.i {
		t.Errorf("Int(): expected %d, got %d", exp.i, s.Int())
	}
	if s.IsUndefined() != exp.undefined {
		t.Errorf("IsUndefined(): expected %t, got %t", exp.undefined, s.IsUndefined())
	}
}

var undefinedSpan = expSpan{min: RegoUndefined, max: RegoUndefined, str: "unknown", legacy: RegoUndefined, undefined: true}

func TestNewRegoVersionSpan(t *testing.T) {
	tests := []struct {
		note   string
		lo, hi RegoVersion
		exp    expSpan
		expErr string
	}{
		{
			note: "both undefined",
			lo:   RegoUndefined, hi: RegoUndefined,
			exp: undefinedSpan,
		},
		{
			note: "undefined min",
			lo:   RegoUndefined, hi: RegoV1,
			expErr: "invalid rego-version span [unknown, v1]: undefined min",
		},
		{
			note: "undefined min, v0v1 max",
			lo:   RegoUndefined, hi: RegoV0CompatV1,
			expErr: "invalid rego-version span [unknown, v0v1]: undefined min",
		},
		{
			note: "v0, undefined max",
			lo:   RegoV0, hi: RegoUndefined,
			exp: expSpan{min: RegoV0, max: RegoV0, str: "v0", legacy: RegoV0, i: 0},
		},
		{
			note: "v1, undefined max",
			lo:   RegoV1, hi: RegoUndefined,
			exp: expSpan{min: RegoV1, max: RegoV1, str: "v1", legacy: RegoV1, i: 1},
		},
		{
			note: "v2, undefined max",
			lo:   RegoV2, hi: RegoUndefined,
			exp: expSpan{min: RegoV2, max: RegoV2, str: "v2", legacy: RegoV2, i: 2},
		},
		{
			note: "v0, v0",
			lo:   RegoV0, hi: RegoV0,
			exp: expSpan{min: RegoV0, max: RegoV0, str: "v0", legacy: RegoV0, i: 0},
		},
		{
			note: "v0, v1",
			lo:   RegoV0, hi: RegoV1,
			exp: expSpan{min: RegoV0, max: RegoV1, str: "v0v1", legacy: RegoV0CompatV1, i: 0},
		},
		{
			note: "v0, v2",
			lo:   RegoV0, hi: RegoV2,
			exp: expSpan{min: RegoV0, max: RegoV2, str: "v0v2", legacy: RegoV0CompatV1, i: 0},
		},
		{
			note: "v1, v1",
			lo:   RegoV1, hi: RegoV1,
			exp: expSpan{min: RegoV1, max: RegoV1, str: "v1", legacy: RegoV1, i: 1},
		},
		{
			note: "v1, v2",
			lo:   RegoV1, hi: RegoV2,
			exp: expSpan{min: RegoV1, max: RegoV2, str: "v1v2", legacy: RegoV1, i: 1},
		},
		{
			note: "v2, v2",
			lo:   RegoV2, hi: RegoV2,
			exp: expSpan{min: RegoV2, max: RegoV2, str: "v2", legacy: RegoV2, i: 2},
		},
		{
			note: "v1, v0",
			lo:   RegoV1, hi: RegoV0,
			expErr: "invalid rego-version span [v1, v0]: min > max",
		},
		{
			note: "v2, v0",
			lo:   RegoV2, hi: RegoV0,
			expErr: "invalid rego-version span [v2, v0]: min > max",
		},
		{
			note: "v2, v1",
			lo:   RegoV2, hi: RegoV1,
			expErr: "invalid rego-version span [v2, v1]: min > max",
		},
		{
			note: "v0v1 min",
			lo:   RegoV0CompatV1, hi: RegoV2,
			expErr: "invalid rego-version span [v0v1, v2]: bounds must be v0, v1 or v2",
		},
		{
			note: "v0v1 min, undefined max",
			lo:   RegoV0CompatV1, hi: RegoUndefined,
			expErr: "invalid rego-version span [v0v1, v0v1]: bounds must be v0, v1 or v2",
		},
		{
			note: "v0v1 max",
			lo:   RegoV0, hi: RegoV0CompatV1,
			expErr: "invalid rego-version span [v0, v0v1]: bounds must be v0, v1 or v2",
		},
		{
			note: "unknown min",
			lo:   RegoVersion(255), hi: RegoV1,
			expErr: "invalid rego-version span [unknown, v1]: bounds must be v0, v1 or v2",
		},
		{
			note: "unknown max",
			lo:   RegoV1, hi: RegoVersion(255),
			expErr: "invalid rego-version span [v1, unknown]: bounds must be v0, v1 or v2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			s, err := NewRegoVersionSpan(tc.lo, tc.hi)
			if tc.expErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got span %s", tc.expErr, s)
				}
				if err.Error() != tc.expErr {
					t.Fatalf("expected error %q, got %q", tc.expErr, err.Error())
				}
				assertSpan(t, s, undefinedSpan)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			assertSpan(t, s, tc.exp)
		})
	}
}

func TestRegoVersionSpanOf(t *testing.T) {
	tests := []struct {
		v   RegoVersion
		exp expSpan
	}{
		{v: RegoUndefined, exp: undefinedSpan},
		{v: RegoV0, exp: expSpan{min: RegoV0, max: RegoV0, str: "v0", legacy: RegoV0, i: 0}},
		{v: RegoV0CompatV1, exp: expSpan{min: RegoV0, max: RegoV1, str: "v0v1", legacy: RegoV0CompatV1, i: 0}},
		{v: RegoV1, exp: expSpan{min: RegoV1, max: RegoV1, str: "v1", legacy: RegoV1, i: 1}},
		{v: RegoV2, exp: expSpan{min: RegoV2, max: RegoV2, str: "v2", legacy: RegoV2, i: 2}},
	}

	for _, tc := range tests {
		t.Run(tc.v.String(), func(t *testing.T) {
			assertSpan(t, RegoVersionSpanOf(tc.v), tc.exp)
		})
	}
}

func TestRegoVersionSpanZeroValue(t *testing.T) {
	assertSpan(t, RegoVersionSpan{}, undefinedSpan)
}
