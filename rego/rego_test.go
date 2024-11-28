// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package rego

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestRegoEval_DefaultRegoVersion(t *testing.T) {
	tests := []struct {
		note      string
		module    string
		expResult interface{}
		expErrs   []string
	}{
		{
			note: "v0", // v0 is the default version
			module: `package test

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note: "v0, v1 compile-time violations",
			module: `package test
import data.foo
import data.bar as foo

p[x] {
	x = ["a", "b", "c"][_]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note: "import rego.v1",
			module: `package test
import rego.v1

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expResult: []string{"a", "b", "c"},
		},
		{
			note: "v0 import rego.v1, v1 compile-time violations",
			module: `package test
import rego.v1

import data.foo
import data.bar as foo

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"test.rego:5: rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note: "v1", // v1 is NOT the default version
			module: `package test

p contains x if {
	some x in ["a", "b", "c"]
}`,
			expErrs: []string{
				"test.rego:4: rego_parse_error: unexpected identifier token",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(root string) {
				ctx := context.Background()

				pq, err := New(
					Load([]string{root}, nil),
					Query("data.test.p"),
				).PrepareForEval(ctx)

				if tc.expErrs != nil {
					if err == nil {
						t.Fatalf("Expected error but got nil")
					}

					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("Expected error to contain %q but got: %v", expErr, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					rs, err := pq.Eval(ctx)
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}

					if len(rs) != 1 {
						t.Fatalf("Expected exactly one result but got: %v", rs)
					}

					if reflect.DeepEqual(rs[0].Expressions[0].Value, tc.expResult) {
						t.Fatalf("Expected %v but got: %v", tc.expResult, rs[0].Expressions[0].Value)
					}
				}
			})
		})
	}
}
