// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"path"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/util/test"
)

func TestCheckRespectsCapabilities(t *testing.T) {
	tests := []struct {
		note       string
		caps       string
		policy     string
		err        string
		bundleMode bool // check with "-b" flag
	}{
		{
			note: "builtin defined in caps",
			caps: `{
			"builtins": [
				{
					"name": "is_foo",
					"decl": {
						"args": [
							{
								"type": "string"
							}
						],
						"result": {
							"type": "boolean"
						},
						"type": "function"
					}
				}
			]
		}`,
			policy: `package test
p { is_foo("bar") }`,
		},
		{
			note: "future kw NOT defined in caps",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.FutureKeywords = []string{"in"}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import future.keywords.if
import future.keywords.in
p if "opa" in input.tools`,
			err: "rego_parse_error: unexpected keyword, must be one of [in]",
		},
		{
			note: "future kw are defined in caps",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.FutureKeywords = []string{"in", "if"}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import future.keywords.if
import future.keywords.in
p if "opa" in input.tools`,
		},
		{
			note: "rego.v1 imported but NOT defined in capabilities",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.Features = []string{}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import rego.v1`,
			err: "rego_parse_error: invalid import, `rego.v1` is not supported by current capabilities",
		},
		{
			note: "rego.v1 imported AND defined in capabilities",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.Features = []string{ast.FeatureRegoV1Import}
				j, err := json.Marshal(c)
				if err != nil {
					panic(err)
				}
				return string(j)
			}(),
			policy: `package test
import rego.v1`,
		},
	}

	// add same tests for bundle-mode == true:
	for i := range tests {
		tc := tests[i]
		tc.bundleMode = true
		tc.note = tc.note + " (as bundle)"
		tests = append(tests, tc)
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"capabilities.json": tc.caps,
				"test.rego":         tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				caps := newcapabilitiesFlag()
				if err := caps.Set(path.Join(root, "capabilities.json")); err != nil {
					t.Fatal(err)
				}
				params := newCheckParams()
				params.capabilities = caps
				params.bundleMode = tc.bundleMode

				err := checkModules(params, []string{root})
				switch {
				case err != nil && tc.err != "":
					if !strings.Contains(err.Error(), tc.err) {
						t.Fatalf("expected err %v, got %v", tc.err, err)
					}
					return // don't read back bundle below
				case err != nil && tc.err == "":
					t.Fatalf("unexpected error: %v", err)
				case err == nil && tc.err != "":
					t.Fatalf("expected error %v, got nil", tc.err)
				}
			})
		})
	}
}

func testCheckWithSchemasAnnotationButNoSchemaFlag(policy string) error {
	files := map[string]string{
		"test.rego": policy,
	}

	var err error
	test.WithTempFS(files, func(path string) {
		params := newCheckParams()

		err = checkModules(params, []string{path})
	})

	return err
}

func TestCheckIgnoresNonRegoFiles(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test`,
		"test.json": `{"foo": "bar"}`,
		"test.yaml": `foo: bar`,
	}

	test.WithTempFS(files, func(root string) {
		params := newCheckParams()

		err := checkModules(params, []string{root})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCheckFailsOnInvalidRego(t *testing.T) {
	files := map[string]string{
		"test.rego": `package test
{}`,
		"test.json": `{"foo": "bar"}`,
	}
	expectedError := "rego_parse_error: object cannot be used for rule name"

	test.WithTempFS(files, func(root string) {
		params := newCheckParams()

		err := checkModules(params, []string{root})
		if err == nil {
			t.Fatalf("expected error %v but received none", expectedError)
		}
		if !strings.Contains(err.Error(), expectedError) {
			t.Fatalf("expected error %v but received %v", expectedError, err)
		}
	})
}

// Assert that 'schemas' annotations with schema refs are only informing the type checker when the --schema flag is used
func TestCheckWithSchemasAnnotationButNoSchemaFlag(t *testing.T) {
	policyWithSchemaRef := `
package test
# METADATA
# schemas:
#   - input: schema["input"]
p { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation 
	input.foo == 42 # type mismatch with schema that should be ignored
}`

	err := testCheckWithSchemasAnnotationButNoSchemaFlag(policyWithSchemaRef)
	if err != nil {
		t.Fatalf("unexpected error from eval with schema ref: %v", err)
	}

	policyWithInlinedSchema := `
package test
# METADATA
# schemas:
#   - input.foo: {"type": "boolean"}
p { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation 
	input.foo == 42 # type mismatch with schema that should be ignored
}`

	err = testCheckWithSchemasAnnotationButNoSchemaFlag(policyWithInlinedSchema)
	// We expect an error here, as inlined schemas are always used for type checking
	if !strings.Contains(err.Error(), "rego_type_error: match error") {
		t.Fatalf("unexpected error from eval with inlined schema, got: %v", err)
	}
}
