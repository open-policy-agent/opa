// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package corpusgen

import (
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestRegoVersion(t *testing.T) {
	tests := []struct {
		in      string
		want    ast.RegoVersion
		wantErr bool
	}{
		{in: "", want: ast.RegoV1},
		{in: "v1", want: ast.RegoV1},
		{in: "v0", want: ast.RegoV0},
		{in: "v0-compat-v1", want: ast.RegoV0CompatV1},
		{in: "v2", wantErr: true},
		{in: "V1", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := RegoVersion(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected %q to be rejected, got %v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestRegoVersionRejected(t *testing.T) {
	tests := []struct {
		note      string
		version   string
		supported []ast.RegoVersion
		want      bool
	}{
		{
			note:    "no supported versions filters nothing",
			version: "v0",
			want:    false,
		},
		{
			note:      "an absent version is v1",
			version:   "",
			supported: []ast.RegoVersion{ast.RegoV1},
			want:      false,
		},
		{
			note:      "an absent version is not v0",
			version:   "",
			supported: []ast.RegoVersion{ast.RegoV0},
			want:      true,
		},
		{
			note:      "matched exactly",
			version:   "v0",
			supported: []ast.RegoVersion{ast.RegoV0},
			want:      false,
		},
		{
			note:      "one of several",
			version:   "v0",
			supported: []ast.RegoVersion{ast.RegoV1, ast.RegoV0},
			want:      false,
		},
		{
			// v0-compat-v1 is its own parsing mode, so neither of the versions
			// its name mentions implies support for it.
			note:      "v0-compat-v1 is not implied by v1",
			version:   "v0-compat-v1",
			supported: []ast.RegoVersion{ast.RegoV1},
			want:      true,
		},
		{
			note:      "v0-compat-v1 is not implied by v0",
			version:   "v0-compat-v1",
			supported: []ast.RegoVersion{ast.RegoV0},
			want:      true,
		},
		{
			note:      "v0-compat-v1 does not imply v1",
			version:   "v1",
			supported: []ast.RegoVersion{ast.RegoV0CompatV1},
			want:      true,
		},
		{
			note:      "an unknown version is rejected rather than run as v1",
			version:   "v2",
			supported: []ast.RegoVersion{ast.RegoV1},
			want:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if got := RegoVersionRejected(tc.version, tc.supported); got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}
