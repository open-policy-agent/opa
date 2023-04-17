// Copyright 2021 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// +build opa_wasm

package cmd

import (
	"testing"
)

func TestEvalWithJSONInputFile(t *testing.T) {

	input := `{
		"foo": "a",
		"b": [
			{
				"a": 1,
				"b": [1, 2, 3],
				"c": null
			}
		]
}`
	query := "input.b[0].a == 1"

	for _, tgt := range []string{"rego", "wasm"} {
		t.Run(tgt, func(t *testing.T) {
			params := newEvalCommandParams()
			params.target.Set(tgt)
			err := testEvalWithInputFile(t, input, query, params)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}

func TestEvalWithYAMLInputFile(t *testing.T) {
	input := `
foo: a
b:
  - a: 1
    b: [1, 2, 3]
    c:
`
	query := "input.b[0].a == 1"

	for _, tgt := range []string{"rego", "wasm"} {
		t.Run(tgt, func(t *testing.T) {
			params := newEvalCommandParams()
			params.target.Set(tgt)
			err := testEvalWithInputFile(t, input, query, params)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
}
