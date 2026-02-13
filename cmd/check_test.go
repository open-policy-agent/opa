// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package cmd

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/util/test"
)

func TestCheckRespectsCapabilities(t *testing.T) {
	//nolint:prealloc // test slice is extended dynamically, initial values are clearer as slice literal
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
				c.Features = []string{}
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
			note: "future kw NOT defined in caps, rego-v1 feature",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.FutureKeywords = []string{"in"}
				c.Features = []string{ast.FeatureRegoV1}
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
		{
			note: "rego.v1 imported AND rego-v1 in capabilities",
			caps: func() string {
				c := ast.CapabilitiesForThisVersion()
				c.Features = []string{ast.FeatureRegoV1}
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
		tc.note += " (as bundle)"
		tests = append(tests, tc)
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"capabilities.json": tc.caps,
				"test.rego":         tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				caps := newCapabilitiesFlag()
				if err := caps.Set(path.Join(root, "capabilities.json")); err != nil {
					t.Fatal(err)
				}
				params := newCheckParams()
				params.capabilities = caps
				params.bundleMode = tc.bundleMode
				// Capabilities in test cases is pre v1
				params.v0Compatible = true

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

func TestCheckIgnoreBundleMode(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"ignore.rego":  `invalid rego`,
		"include.rego": `package valid`,
	}

	test.WithTempFS(files, func(root string) {
		params := newCheckParams()

		params.ignore = []string{"ignore.rego"}
		params.bundleMode = true

		err := checkModules(params, []string{root})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCheckBundleReportsPolicyVsDataConflict(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"policy.rego": "package p\nallow := false\n",
		"data.json":   `{"p":{"allow":false}}`,
	}

	test.WithTempFS(files, func(root string) {
		params := newCheckParams()
		// Bundle mode required as the check command *should* ignore data entirely otherwise
		params.bundleMode = true

		err := checkModules(params, []string{root})
		if err == nil {
			t.Fatal("expected error but received none")
		}

		exp := fmt.Sprintf(
			"1 error occurred: %s:2: rego_compile_error: conflicting rule for data path p/allow found",
			filepath.Join(root, "policy.rego"),
		)
		if err.Error() != exp {
			t.Fatalf("expected error %q, got %q", exp, err.Error())
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
	policiesWithSchemaRef := []string{`
package test
import rego.v1
# METADATA
# schemas:
#   - input: schema["input"]
p if { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation 
	input.foo == 42 # type mismatch with schema that should be ignored
}`,
		`
package p

# METADATA
# schemas:
# - data.p.x: schema["nope"]
bug := data.p.x
`}

	for i, pol := range policiesWithSchemaRef {
		err := testCheckWithSchemasAnnotationButNoSchemaFlag(pol)
		if err != nil {
			t.Fatalf("unexpected error from eval policy %d with schema ref: %v", i, err)
		}
	}

	policyWithInlinedSchema := `
package test
import rego.v1
# METADATA
# schemas:
#   - input.foo: {"type": "boolean"}
p if { 
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation 
	input.foo == 42 # type mismatch with schema that should be ignored
}`

	err := testCheckWithSchemasAnnotationButNoSchemaFlag(policyWithInlinedSchema)
	// We expect an error here, as inlined schemas are always used for type checking
	if !strings.Contains(err.Error(), "rego_type_error: match error") {
		t.Fatalf("unexpected error from eval with inlined schema, got: %v", err)
	}
}

func TestCheckRegoV1(t *testing.T) {
	cases := []struct {
		note    string
		policy  string
		expErrs []string
	}{
		{
			note: "rego.v1 imported, v1 compliant",
			policy: `package test
import rego.v1
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note: "rego.v1 imported, NOT v1 compliant (parser)",
			policy: `package test
import rego.v1
p contains x {
	x := [1,2,3]
}

q.r`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:7: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "rego.v1 imported, NOT v1 compliant (compiler)",
			policy: `package test
import rego.v1

import data.foo
import data.bar as foo
`,
			expErrs: []string{
				"test.rego:5: rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note: "keywords imported, v1 compliant",
			policy: `package test
import future.keywords.if
import future.keywords.contains
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note: "keywords imported, NOT v1 compliant",
			policy: `package test
import future.keywords.contains
p contains x {
	x := [1,2,3]
}

q.r`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:7: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "keywords imported, NOT v1 compliant (compiler)",
			policy: `package test
import future.keywords.if

input := 1 if {
	1 == 2
}`,
			expErrs: []string{
				"test.rego:4: rego_compile_error: rules must not shadow input (use a different rule name)",
			},
		},
		{
			note: "no imports, v1 compliant",
			policy: `package test
p := 1
`,
		},
		{
			note: "no imports, NOT v1 compliant but v0 compliant (compiler)",
			policy: `package test
p.x`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "no imports, v1 compliant but NOT v0 compliant",
			policy: `package test
p contains x if {
	x := [1,2,3]
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: var cannot be used for rule name", // This error actually appears three times: once for 'p'; once for 'contains'; and once for 'x'. All are interpreted as [invalid] rule declarations with no value and body.
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				params := newCheckParams()
				params.regoV1 = true

				err := checkModules(params, []string{root})
				switch {
				case err != nil && len(tc.expErrs) > 0:
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected err:\n\n%v\n\ngot:\n\n%v", expErr, err)
						}
					}
					return // don't read back bundle below
				case err != nil && len(tc.expErrs) == 0:
					t.Fatalf("unexpected error: %v", err)
				case err == nil && len(tc.expErrs) > 0:
					t.Fatalf("expected error:\n\n%v\n\ngot: none", tc.expErrs)
				}
			})
		})
	}
}

func TestCheck_DefaultRegoVersion(t *testing.T) {
	cases := []struct {
		note    string
		policy  string
		expErrs []string
	}{
		{
			note: "v0 module",
			policy: `package test
a[x] {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1 module",
			policy: `package test
a contains x if {
	x := 42
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				params := newCheckParams()

				err := checkModules(params, []string{root})
				switch {
				case err != nil && len(tc.expErrs) > 0:
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected err:\n\n%v\n\ngot:\n\n%v", expErr, err)
						}
					}
					return // don't read back bundle below
				case err != nil && len(tc.expErrs) == 0:
					t.Fatalf("unexpected error: %v", err)
				case err == nil && len(tc.expErrs) > 0:
					t.Fatalf("expected error:\n\n%v\n\ngot: none", tc.expErrs)
				}
			})
		})
	}
}

func TestCheckWithRegoV1Capability(t *testing.T) {
	cases := []struct {
		note         string
		v0Compatible bool
		capabilities *ast.Capabilities
		policy       string
		expErrs      []string
	}{
		{
			note:         "v0 module, v0-compatible, no capabilities",
			v0Compatible: true,
			policy: `package test
a[x] {
	x := 42
}`,
		},
		{
			note:         "v0 module, v0-compatible, v0 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			policy: `package test
a[x] {
	x := 42
}`,
		},
		{
			note:         "v0 module, v0-compatible, v1 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			policy: `package test
a[x] {
	x := 42
}`,
		},

		{
			note: "v0 module, not v0-compatible, no capabilities",
			policy: `package test
a[x] {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v0 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			policy: `package test
a[x] {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v0 capabilities without rego_v1 feature",
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			policy: `package test
a[x] {
	x := 42
}`,
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v0 module, not v0-compatible, v1 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			policy: `package test
a[x] {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		{
			note:         "v1 module, v0-compatible, no capabilities",
			v0Compatible: true,
			policy: `package test
a contains x if {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v1 module, v0-compatible, v0 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			policy: `package test
a contains x if {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v1 module, v0-compatible, v1 capabilities",
			v0Compatible: true,
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			policy: `package test
a contains x if {
	x := 42
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: var cannot be used for rule name",
			},
		},

		{
			note: "v1 module, not v0-compatible, no capabilities",
			policy: `package test
a contains x if {
	x := 42
}`,
		},
		{
			note:         "v1 module, not v0-compatible, v0 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV0)),
			policy: `package test
a contains x if {
	x := 42
}`,
		},
		{
			note:         "v1 module, not v0-compatible, v0 capabilities without rego_v1 feature",
			capabilities: capsWithoutFeat(ast.RegoV0, ast.FeatureRegoV1),
			policy: `package test
a contains x if {
	x := 42
}`,
			expErrs: []string{
				"rego_parse_error: illegal capabilities: rego_v1 feature required for parsing v1 Rego",
			},
		},
		{
			note:         "v1 module, not v0-compatible, v1 capabilities",
			capabilities: ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(ast.RegoV1)),
			policy: `package test
a contains x if {
	x := 42
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				params := newCheckParams()
				params.v0Compatible = tc.v0Compatible
				params.capabilities.C = tc.capabilities

				err := checkModules(params, []string{root})
				switch {
				case err != nil && len(tc.expErrs) > 0:
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected err:\n\n%v\n\ngot:\n\n%v", expErr, err)
						}
					}
					return // don't read back bundle below
				case err != nil && len(tc.expErrs) == 0:
					t.Fatalf("unexpected error: %v", err)
				case err == nil && len(tc.expErrs) > 0:
					t.Fatalf("expected error:\n\n%v\n\ngot: none", tc.expErrs)
				}
			})
		})
	}
}

func TestCheckCompatibleFlags(t *testing.T) {
	cases := []struct {
		note         string
		v0Compatible bool
		v1Compatible bool
		policy       string
		expErrs      []string
	}{
		{
			note:         "v0, no illegal keywords",
			v0Compatible: true,
			policy: `package test
p[x] {
	x := [1,2,3]
}`,
		},
		{
			note:         "v0, illegal keywords",
			v0Compatible: true,
			policy: `package test
p contains x if {
	x := [1,2,3]
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v0, future.keywords imported",
			v0Compatible: true,
			policy: `package test
import future.keywords
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note:         "v0, rego.v1 imported",
			v0Compatible: true,
			policy: `package test
import rego.v1
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note:         "v1, rego.v1 imported, v1 compliant",
			v1Compatible: true,
			policy: `package test
import rego.v1
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note:         "v1, rego.v1 imported, NOT v1 compliant (parser)",
			v1Compatible: true,
			policy: `package test
import rego.v1
p contains x {
	x := [1,2,3]
}

q.r`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:7: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, rego.v1 imported, NOT v1 compliant (compiler)",
			v1Compatible: true,
			policy: `package test
import rego.v1

import data.foo
import data.bar as foo
`,
			expErrs: []string{
				"test.rego:5: rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note:         "v1, keywords imported, v1 compliant",
			v1Compatible: true,
			policy: `package test
import future.keywords.if
import future.keywords.contains
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note:         "v1, keywords imported, NOT v1 compliant",
			v1Compatible: true,
			policy: `package test
import future.keywords.contains
p contains x {
	x := [1,2,3]
}

q.r`,
			expErrs: []string{
				"test.rego:3: rego_parse_error: `if` keyword is required before rule body",
				"test.rego:7: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, keywords imported, NOT v1 compliant (compiler)",
			v1Compatible: true,
			policy: `package test
import future.keywords.if

input := 1 if {
	1 == 2
}`,
			expErrs: []string{
				"test.rego:4: rego_compile_error: rules must not shadow input (use a different rule name)",
			},
		},
		{
			note:         "v1, no imports, v1 compliant",
			v1Compatible: true,
			policy: `package test
p := 1
`,
		},
		{
			note:         "v1, no imports, NOT v1 compliant but v0 compliant (compiler)",
			v1Compatible: true,
			policy: `package test
p.x`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note:         "v1, no imports, v1 compliant but NOT v0 compliant",
			v1Compatible: true,
			policy: `package test
p contains x if {
	x := [1,2,3]
}`,
		},
		// v0 takes precedence over v1
		{
			note:         "v0+v1, no illegal keywords",
			v0Compatible: true,
			v1Compatible: true,
			policy: `package test
p[x] {
	x := [1,2,3]
}`,
		},
		{
			note:         "v0+v1, illegal keywords",
			v0Compatible: true,
			v1Compatible: true,
			policy: `package test
p contains x if {
	x := [1,2,3]
}`,
			expErrs: []string{
				"test.rego:2: rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note:         "v0+v1, future.keywords imported",
			v0Compatible: true,
			v1Compatible: true,
			policy: `package test
import future.keywords
p contains x if {
	x := [1,2,3]
}`,
		},
		{
			note:         "v0+v1, rego.v1 imported",
			v0Compatible: true,
			v1Compatible: true,
			policy: `package test
import rego.v1
p contains x if {
	x := [1,2,3]
}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(root string) {
				params := newCheckParams()
				params.v0Compatible = tc.v0Compatible
				params.v1Compatible = tc.v1Compatible

				err := checkModules(params, []string{root})
				switch {
				case err != nil && len(tc.expErrs) > 0:
					for _, expErr := range tc.expErrs {
						if !strings.Contains(err.Error(), expErr) {
							t.Fatalf("expected err:\n\n%v\n\ngot:\n\n%v", expErr, err)
						}
					}
					return // don't read back bundle below
				case err != nil && len(tc.expErrs) == 0:
					t.Fatalf("unexpected error: %v", err)
				case err == nil && len(tc.expErrs) > 0:
					t.Fatalf("expected error:\n\n%v\n\ngot: none", tc.expErrs)
				}
			})
		})
	}
}

func TestCheckWithBundleRegoVersion(t *testing.T) {
	cases := []struct {
		note    string
		files   map[string]string
		expErrs []string
	}{
		{
			note: "v0.x bundle, illegal keywords",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
p contains x if {
	x := [1,2,3]
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
		{
			note: "v0.x bundle, rego.v1 imported, v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import rego.v1
p contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v0.x bundle, rego.v1 imported, NOT v1 compliant (parser)",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import rego.v1
p contains x {
	x := [1,2,3]
}

q.r`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v0.x bundle, rego.v1 imported, NOT v1 compliant (compiler)",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import rego.v1

import data.foo
import data.bar as foo
`,
			},
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note: "v0.x bundle, keywords imported, v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
import future.keywords.if
import future.keywords.contains
p contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v0.x bundle, no imports, v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
p := 1
`,
			},
		},
		{
			note: "v0 bundle, v1 per-file overrides, compliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[x] {
	x := [1,2,3]
}`,
				"policy2.rego": `package test
q contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file overrides (glob), compliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"*/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[x] {
	x := [1,2,3]
}`,
				"policy2.rego": `package test
q contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v0 bundle, v1 per-file overrides, incompliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[x] {
	x := [1,2,3]
}`,
				"policy2.rego": `package test
q[x] {
	x := [1,2,3]
}`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},

		{
			note: "v1.0 bundle, keywords used but not imported",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v1.0 bundle, rego.v1 imported, v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import rego.v1
p contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v1.0 bundle, rego.v1 imported, NOT v1 compliant (parser)",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import rego.v1
p contains x {
	x := [1,2,3]
}

q.r`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1.0 bundle, rego.v1 imported, NOT v1 compliant (compiler)",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import rego.v1

import data.foo
import data.bar as foo
`,
			},
			expErrs: []string{
				"rego_compile_error: import must not shadow import data.foo",
			},
		},
		{
			note: "v1.0 bundle, keywords imported, v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import future.keywords.if
import future.keywords.contains
p contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v1.0 bundle, keywords imported, NOT v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import future.keywords.contains
p contains x {
	x := [1,2,3]
}

q.r`,
			},
			expErrs: []string{
				"rego_parse_error: `if` keyword is required before rule body",
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1.0 bundle, keywords imported, NOT v1 compliant (compiler)",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import future.keywords.if

input := 1 if {
	1 == 2
}`,
			},
			expErrs: []string{
				"rego_compile_error: rules must not shadow input (use a different rule name)",
			},
		},
		{
			note: "v1.0 bundle, no imports, v1 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p := 1
`,
			},
		},
		{
			note: "v1.0 bundle, no imports, NOT v1 compliant but v0 compliant (compiler)",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p.x`,
			},
			expErrs: []string{
				"rego_parse_error: `contains` keyword is required for partial set rules",
			},
		},
		{
			note: "v1.0 bundle, no imports, v1 compliant but NOT v0 compliant",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
p contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file overrides, compliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p[x] {
	x := [1,2,3]
}`,
				"policy2.rego": `package test
q contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file overrides (glob), compliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"*/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p[x] {
	x := [1,2,3]
}`,
				"policy2.rego": `package test
q contains x if {
	x := [1,2,3]
}`,
			},
		},
		{
			note: "v1 bundle, v0 per-file overrides, incompliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p contains x if {
	x := [1,2,3]
}`,
				"policy2.rego": `package test
q contains x if {
	x := [1,2,3]
}`,
			},
			expErrs: []string{
				"rego_parse_error: var cannot be used for rule name",
			},
		},
	}

	bundleTypeCases := []struct {
		note string
		tar  bool
	}{
		{
			"bundle dir", false,
		},
		{
			"bundle tar", true,
		},
	}

	v1CompatibleFlagCases := []struct {
		note string
		used bool
	}{
		{
			"no --v1-compatible", false,
		},
		{
			"--v1-compatible", true,
		},
	}

	for _, bundleType := range bundleTypeCases {
		for _, v1CompatibleFlag := range v1CompatibleFlagCases {
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s, %s, %s", bundleType.note, v1CompatibleFlag.note, tc.note), func(t *testing.T) {
					files := map[string]string{}

					if bundleType.tar {
						files["bundle.tar.gz"] = ""
					} else {
						maps.Copy(files, tc.files)
					}

					test.WithTempFS(files, func(root string) {
						p := root
						if bundleType.tar {
							p = filepath.Join(root, "bundle.tar.gz")
							files := make([][2]string, 0, len(tc.files))
							for k, v := range tc.files {
								files = append(files, [2]string{k, v})
							}
							buf := archive.MustWriteTarGz(files)
							bf, err := os.Create(p)
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
							_, err = bf.Write(buf.Bytes())
							if err != nil {
								t.Fatalf("Unexpected error: %v", err)
							}
						}

						params := newCheckParams()
						params.bundleMode = true
						params.v1Compatible = v1CompatibleFlag.used

						err := checkModules(params, []string{p})
						switch {
						case err != nil && len(tc.expErrs) > 0:
							for _, expErr := range tc.expErrs {
								if !strings.Contains(err.Error(), expErr) {
									t.Fatalf("expected err:\n\n%v\n\ngot:\n\n%v", expErr, err)
								}
							}
							return // don't read back bundle below
						case err != nil && len(tc.expErrs) == 0:
							t.Fatalf("unexpected error: %v", err)
						case err == nil && len(tc.expErrs) > 0:
							t.Fatalf("expected error:\n\n%v\n\ngot: none", tc.expErrs)
						}
					})
				})
			}
		}
	}
}
