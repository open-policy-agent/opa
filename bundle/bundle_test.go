// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package bundle

import (
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
)

func TestRead_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note    string
		module  string
		expErrs []string
	}{
		{
			note: "v0", // v0 is the default rego-version
			module: `package example

p[x] {
	x := "a"
}`,
		},
		{
			note: "rego.v1 import",
			module: `package example
import rego.v1

p contains x if {
	x := "a"
}`,
		},
		{
			note: "v1",
			module: `package example

p contains x if {
	x := "a"
}`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			module := tc.module
			files := [][2]string{
				{"test.rego", module},
			}

			buf := archive.MustWriteTarGz(files)
			loader := NewTarballLoaderWithBaseURL(buf, "")
			br := NewCustomReader(loader)

			bundle, err := br.Read()

			if len(tc.expErrs) > 0 {
				if err == nil {
					t.Fatalf("Expected error(s):\n\n%v\n\nbut got nil", tc.expErrs)
				}

				for _, expErr := range tc.expErrs {
					if !strings.Contains(err.Error(), expErr) {
						t.Fatalf("Expected error:\n\n%s\n\nbut got:\n\n%s", expErr, err)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %s", err)
				}

				if len(bundle.Modules) != 1 {
					t.Fatalf("expected 1 module but got %d", len(bundle.Modules))
				}
			}
		})
	}
}
