// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package conformance

import (
	"slices"
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
