// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package conformance

import (
	"slices"
	"strings"
	"testing"
)

func TestMatchErrors(t *testing.T) {
	unsafe := Error{Code: "rego_unsafe_var_error", Row: 3, Col: 2, Message: "var x is unsafe"}
	other := Error{Code: "rego_type_error", Row: 4, Col: 1, Message: "undefined function f"}

	tests := []struct {
		note           string
		want           []Error
		got            []Error
		exhaustive     bool
		wantMissing    []Error
		wantUnexpected []Error
	}{
		{
			note: "match",
			want: []Error{unsafe},
			got:  []Error{unsafe},
		},
		{
			note: "order is not part of the contract",
			want: []Error{unsafe, other},
			got:  []Error{other, unsafe},
		},
		{
			note: "subset is accepted when not exhaustive",
			want: []Error{unsafe},
			got:  []Error{unsafe, other},
		},
		{
			note:           "subset is rejected when exhaustive",
			want:           []Error{unsafe},
			got:            []Error{unsafe, other},
			exhaustive:     true,
			wantUnexpected: []Error{other},
		},
		{
			note:        "missing",
			want:        []Error{unsafe, other},
			got:         []Error{unsafe},
			wantMissing: []Error{other},
		},
		{
			note: "zero col matches any col",
			want: []Error{{Code: unsafe.Code, Row: unsafe.Row, Message: unsafe.Message}},
			got:  []Error{unsafe},
		},
		{
			note:        "non-zero col must match",
			want:        []Error{{Code: unsafe.Code, Row: unsafe.Row, Col: 9, Message: unsafe.Message}},
			got:         []Error{unsafe},
			wantMissing: []Error{{Code: unsafe.Code, Row: unsafe.Row, Col: 9, Message: unsafe.Message}},
		},
		{
			note:        "the default module name matches an unnamed one",
			want:        []Error{{Module: DefaultModuleName, Code: unsafe.Code, Row: unsafe.Row, Message: unsafe.Message}},
			got:         []Error{unsafe, {Module: ModuleName(1), Code: unsafe.Code, Row: unsafe.Row, Message: unsafe.Message}},
			wantMissing: nil,
		},
		{
			note:           "a repeated diagnostic is matched once per occurrence",
			want:           []Error{unsafe, unsafe},
			got:            []Error{unsafe, unsafe, unsafe},
			exhaustive:     true,
			wantUnexpected: []Error{unsafe},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			missing, unexpected := MatchErrors(tc.want, tc.got, tc.exhaustive)

			if !slices.Equal(missing, tc.wantMissing) {
				t.Errorf("missing: expected %v, got %v", tc.wantMissing, missing)
			}
			if !slices.Equal(unexpected, tc.wantUnexpected) {
				t.Errorf("unexpected: expected %v, got %v", tc.wantUnexpected, unexpected)
			}
		})
	}
}

func TestUnmarshalRejectsUnknownFields(t *testing.T) {
	type testCase struct {
		Note string `json:"note"`
	}

	tests := []struct {
		note    string
		corpus  string
		wantErr string
	}{
		{
			note:   "a declared field",
			corpus: "cases:\n  - note: a\n",
		},
		{
			note:    "a field the type does not declare",
			corpus:  "cases:\n  - note: a\n    stale: true\n",
			wantErr: `unknown field "stale"`,
		},
		{
			note:    "a field at the top level",
			corpus:  "cases:\n  - note: a\nextra: 1\n",
			wantErr: `unknown field "extra"`,
		},
		{
			note:   "JSON is decoded the same way",
			corpus: `{"cases":[{"note":"a"}]}`,
		},
		{
			note:    "JSON with an unknown field",
			corpus:  `{"cases":[{"note":"a","stale":1}]}`,
			wantErr: `unknown field "stale"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var x struct {
				Cases []testCase `json:"cases"`
			}

			err := Unmarshal([]byte(tc.corpus), &x)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got none", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected an error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
