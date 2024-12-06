// Copyright 2024 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package repl

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/util"
)

func TestOneShot_DefaultRegoVersion(t *testing.T) {
	type action struct {
		line      string
		expOutput string
		expErrs   []string
	}

	tests := []struct {
		note    string
		actions []action
	}{
		{
			note: "v0 rule, v1 compile-time violation",
			actions: []action{
				{
					line:      "b { data := 1; data == 1 }",
					expOutput: "Rule 'b' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "v1 keywords used",
			actions: []action{
				{
					line: "a contains 2 if { true }",
					expErrs: []string{
						"rego_unsafe_var_error: var a is unsafe",
					},
				},
			},
		},
		{
			note: "v1 keywords not used",
			actions: []action{
				{
					line:      "a[2] { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "v1 keywords imported",
			actions: []action{
				{
					line: "import future.keywords",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "rego.v1 imported",
			actions: []action{
				{
					line: "import rego.v1",
				},
				{
					line:      "a contains 2 if { true }",
					expOutput: "Rule 'a' defined in package repl. Type 'show' to see rules.\n",
				},
			},
		},
		{
			note: "v1 keywords",
			actions: []action{
				{
					line: "a contains 2 if { true }",
					expErrs: []string{
						"rego_unsafe_var_error: var a is unsafe",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			ctx := context.Background()
			store := newTestStore()
			var buffer bytes.Buffer
			repl := newRepl(store, &buffer)

			for _, action := range tc.actions {
				err := repl.OneShot(ctx, action.line)

				if len(action.expErrs) != 0 {
					if err == nil {
						t.Fatalf("Expected error but got: %s", buffer.String())
					}

					for _, e := range action.expErrs {
						if !strings.Contains(err.Error(), e) {
							t.Fatalf("Expected error to contain:\n\n%q\n\nbut got:\n\n%v", e, err)
						}
					}
				} else {
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					expectOutput(t, buffer.String(), action.expOutput)
				}
			}
		})
	}
}

func expectOutput(t *testing.T, output string, expected string) {
	t.Helper()
	if output != expected {
		t.Errorf("Repl output: expected %#v but got %#v", expected, output)
	}
}

func newRepl(store storage.Store, buffer *bytes.Buffer) *REPL {
	repl := New(store, "", buffer, "", 0, "")
	return repl
}

func newTestStore() storage.Store {
	input := `
    {
        "a": [
            {
                "b": {
                    "c": [true,2,false]
                }
            },
            {
                "b": {
                    "c": [false,true,1]
                }
            }
        ]
    }
    `
	var data map[string]interface{}
	err := util.UnmarshalJSON([]byte(input), &data)
	if err != nil {
		panic(err)
	}
	return inmem.NewFromObject(data)
}
