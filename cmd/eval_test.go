// Copyright 2018 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// nolint: goconst // string duplication is for test readability.
package cmd

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/internal/presentation"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/topdown"
	"github.com/open-policy-agent/opa/util"
	"github.com/open-policy-agent/opa/util/test"
)

func TestEvalExitCode(t *testing.T) {
	params := newEvalCommandParams()
	params.fail = true

	tests := []struct {
		note        string
		query       string
		wantDefined bool
		wantErr     bool
	}{
		{"defined result", "true=true", true, false},
		{"undefined result", "true = false", false, false},
		{"on error", `{k: v | k = ["a", "a"][_]; v = [0,1][_]}`, false, true},
	}

	var b bytes.Buffer
	writer := bufio.NewWriter(&b)
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			defined, err := eval([]string{tc.query}, params, writer)
			if tc.wantErr && err == nil {
				t.Fatal("wanted error but got success")
			} else if !tc.wantErr && err != nil {
				t.Fatal("wanted success but got error:", err)
			} else if (tc.wantDefined && !defined) || (!tc.wantDefined && defined) {
				t.Fatalf("wanted defined %v but got defined %v", tc.wantDefined, defined)
			}
		})
	}
}

func TestEvalWithShowBuiltinErrors(t *testing.T) {
	files := map[string]string{
		"x.rego": `package x

p {
	1/0
}

q {
	1/0
}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.showBuiltinErrors = true
		params.dataPaths = newrepeatedStringFlag([]string{path})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.x"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("unexpected undefined or error: %v", err)
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		if len(output.Errors) != 2 {
			t.Fatalf("Expected 2 errors in result, got:%v", len(output.Errors))
		}

		expectedCode := "eval_builtin_error"
		expectedMessage := "div: divide by zero"

		if code := output.Errors[0].Code; code != expectedCode {
			t.Fatalf("expected code '%v', got '%v'", expectedCode, code)
		}
		if msg := output.Errors[0].Message; msg != expectedMessage {
			t.Fatalf("expected message '%v', got '%v'", expectedMessage, msg)
		}

		if code := output.Errors[1].Code; code != expectedCode {
			t.Fatalf("expected code '%v', got '%v'", expectedCode, code)
		}
		if msg := output.Errors[1].Message; msg != expectedMessage {
			t.Fatalf("expected message '%v', got '%v'", expectedMessage, msg)
		}

		loc1 := output.Errors[0].Location
		if loc1 == nil {
			t.Fatal("unexpected nil location")
		}

		loc2 := output.Errors[1].Location
		if loc2 == nil {
			t.Fatal("unexpected nil location")
		}

		if loc1.Row == loc2.Row {
			t.Fatal("expected 2 distinct error occurrences in policy")
		}
	})
}

func TestEvalWithProfiler(t *testing.T) {
	files := map[string]string{
		"x.rego": `package x

p {
	a := 1
	b := 2
	c := 3
	x = a + b * c
}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.profile = true
		params.profileCriteria = newrepeatedStringFlag([]string{"line"})
		params.dataPaths = newrepeatedStringFlag([]string{path})

		var buf bytes.Buffer

		defined, err := eval([]string{"data"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		if len(output.Profile) == 0 {
			t.Fatal("Expected profile output to be non-empty")
		}

		expectedNumEval := []int{3, 1, 1, 1, 1}
		expectedNumRedo := []int{3, 1, 1, 1, 1}
		expectedRow := []int{7, 6, 5, 4, 1}
		expectedNumGenExpr := []int{3, 1, 1, 1, 1}

		for idx, actualExprStat := range output.Profile {
			if actualExprStat.NumEval != expectedNumEval[idx] {
				t.Fatalf("Index %v: Expected number of evals %v but got %v", idx, expectedNumEval[idx], actualExprStat.NumEval)
			}

			if actualExprStat.NumRedo != expectedNumRedo[idx] {
				t.Fatalf("Index %v: Expected number of redos %v but got %v", idx, expectedNumRedo[idx], actualExprStat.NumRedo)
			}

			if actualExprStat.Location.Row != expectedRow[idx] {
				t.Fatalf("Index %v: Expected row %v but got %v", idx, expectedRow[idx], actualExprStat.Location.Row)
			}

			if actualExprStat.NumGenExpr != expectedNumGenExpr[idx] {
				t.Fatalf("Index %v: Expected number of generated expressions %v but got %v", idx, expectedNumGenExpr[idx], actualExprStat.NumGenExpr)
			}
		}
	})
}

func TestEvalWithCoverage(t *testing.T) {

	files := map[string]string{
		"x.rego": `package x

p = 1`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.coverage = true
		params.dataPaths = newrepeatedStringFlag([]string{path})

		var buf bytes.Buffer

		defined, err := eval([]string{"data"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		if output.Coverage == nil || output.Coverage.Coverage != 100.0 {
			t.Fatalf("Expected coverage in output but got: %v", buf.String())
		}
	})
}

func TestEvalWithOptimizeErrors(t *testing.T) {
	files := map[string]string{
		"x.rego": `package x

p = 1`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		if err := params.bundlePaths.Set(path); err != nil {
			t.Fatal(err)
		}

		err := validateEvalParams(&params, []string{"data"})
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		expected := "specify either --data or --bundle flag with optimization level greater than 0"
		if err.Error() != expected {
			t.Fatalf("Expected error %v but got %v", expected, err.Error())
		}

		params = newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})

		var buf bytes.Buffer

		_, err = eval([]string{"data.test"}, params, &buf)
		if err == nil {
			t.Fatal("Expected error but got nil")
		}

		expected = "bundle optimizations require at least one entrypoint"
		if err.Error() != expected {
			t.Fatalf("Expected error %v but got %v", expected, err.Error())
		}
	})
}

func TestEvalWithOptimize(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			default p = false
			p { q }
			q { input.x = data.foo }`,
		"data.json": `
			{"foo": 1}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"test/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.test.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

// Ensure that entrypoint annotations don't cause panics when using
// higher levels of optimization.
// Reference: https://github.com/open-policy-agent/opa/issues/5368
func TestEvalIssue5368(t *testing.T) {
	files := map[string]string{
		"test.rego": `
package system

object_key_exists(object, key) {
	_ = object[key]
}

default main = false

# METADATA
# entrypoint: true
main := results {
	object_key_exists(input, "queries")
	results := {key: result |
		result := input.queries[key]
	}
}`,
		"input.json": `{}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 2
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.inputPath = filepath.Join(path, "input.json")

		var buf bytes.Buffer

		defined, err := eval([]string{"data.system.main"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestEvalWithOptimizeBundleData(t *testing.T) {
	files := map[string]string{
		"test.rego": `
			package test
			default p = false
			p { q }
			q { input.x = data.foo }`,
		"data.json": `
			{"foo": 1}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		if err := params.bundlePaths.Set(path); err != nil {
			t.Fatal(err)
		}
		params.entrypoints = newrepeatedStringFlag([]string{"test/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.test.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func testEvalWithInputFile(t *testing.T, input string, query string, params evalCommandParams) error {
	files := map[string]string{
		"input.json": input,
	}

	var err error
	test.WithTempFS(files, func(path string) {

		params.inputPath = filepath.Join(path, "input.json")

		var buf bytes.Buffer
		var defined bool
		defined, err = eval([]string{query}, params, &buf)
		if !defined || err != nil {
			err = fmt.Errorf("Unexpected error or undefined from evaluation: %v", err)
			return
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		rs := output.Result
		if exp, act := true, rs.Allowed(); exp != act {
			t.Errorf("expected %v, got %v", exp, act)
		}
	})

	return err
}

func TestEvalWithInvalidInputFile(t *testing.T) {
	input := `{badjson`
	query := "input.b[0].a == 1"
	err := testEvalWithInputFile(t, input, query, newEvalCommandParams())
	if err == nil {
		t.Fatalf("expected error but err == nil")
	}
}

func testEvalWithSchemaFile(t *testing.T, input string, query string, schema string, policy string, expTypeErr bool) error {
	files := map[string]string{
		"input.json":  input,
		"schema.json": schema,
	}

	policyFilePresent := policy != ""
	if policyFilePresent {
		files["policy.rego"] = policy
	}

	var err error
	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.inputPath = filepath.Join(path, "input.json")
		if policyFilePresent {
			params.dataPaths = newrepeatedStringFlag([]string{path})
		}
		params.schema = &schemaFlags{path: filepath.Join(path, "schema.json")}

		var buf bytes.Buffer
		defined, evalErr := eval([]string{query}, params, &buf)
		if !expTypeErr && (!defined || evalErr != nil) {
			err = fmt.Errorf("unexpected error or undefined from evaluation: %v", evalErr)
			return
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		if expTypeErr {
			if len(output.Errors) != 1 || output.Errors[0].Code != "rego_type_error" {
				err = fmt.Errorf("expected type conflict, got %v", output.Errors)
			}
			return
		}

		rs := output.Result
		if exp, act := true, rs.Allowed(); exp != act {
			t.Errorf("expected %v, got %v", exp, act)
		}
	})

	return err
}

func testEvalWithInvalidSchemaFile(input string, query string, schema string) error {
	files := map[string]string{
		"input.json":  input,
		"schema.json": schema,
	}

	var err error
	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.inputPath = filepath.Join(path, "input.json")
		params.schema = &schemaFlags{path: filepath.Join(path, "schemaBad.json")}

		var buf bytes.Buffer
		var defined bool
		defined, err = eval([]string{query}, params, &buf)
		if !defined || err != nil {
			err = fmt.Errorf("Unexpected error or undefined from evaluation: %v", err)
			return
		}
	})

	return err
}

func testEvalWithSchemasAnnotationButNoSchemaFlag(policy string) error {
	query := "data.test.p"

	files := map[string]string{
		"input.json": `{
				"foo": 42
			}`,
		"test.rego": policy,
	}

	var err error
	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.inputPath = filepath.Join(path, "input.json")
		params.dataPaths = newrepeatedStringFlag([]string{path})

		var buf bytes.Buffer
		var defined bool
		defined, err = eval([]string{query}, params, &buf)
		if !defined || err != nil {
			err = fmt.Errorf(buf.String())
		}
	})

	return err
}

// Assert that 'schemas' annotations with schema refs are only informing the type checker when the --schema flag is used
func TestEvalWithSchemasAnnotationButNoSchemaFlag(t *testing.T) {
	policyWithSchemaRef := `
package test
# METADATA
# schemas:
#   - input: schema["input"]
p {
	rego.metadata.rule() # presence of rego.metadata.* calls must not trigger unwanted schema evaluation
	input.foo == 42 # type mismatch with schema that should be ignored
}`

	err := testEvalWithSchemasAnnotationButNoSchemaFlag(policyWithSchemaRef)
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
	input.foo == 42 # type mismatch with schema that should NOT be ignored since it is an inlined schema format
}`

	err = testEvalWithSchemasAnnotationButNoSchemaFlag(policyWithInlinedSchema)
	// We expect an error here, as inlined schemas are always used for type checking
	if !strings.Contains(err.Error(), `"code": "rego_type_error"`) {
		t.Fatalf("unexpected error from eval with inlined schema, got: %v", err)
	}
}

func testReadParamWithSchemaDir(input string, inputSchema string) error {
	files := map[string]string{
		"input.json":                          input,
		"schemas/input.json":                  inputSchema,
		"schemas/kubernetes/data-schema.json": inputSchema,
	}

	var err error
	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.inputPath = filepath.Join(path, "input.json")
		params.schema = &schemaFlags{path: filepath.Join(path, "schemas")}

		// Don't assign over "err" or "err =" does nothing.
		schemaSet, errSchema := loader.Schemas(params.schema.path)
		if errSchema != nil {
			err = fmt.Errorf("Unexpected error or undefined from evaluation: %v", errSchema)
			return
		}

		if schemaSet == nil {
			err = fmt.Errorf("Schema set is empty")
			return
		}

		if schemaSet.Get(ast.MustParseRef("schema.input")) == nil {
			err = fmt.Errorf("Expected schema for input in schemaSet but got none")
			return
		}

		if schemaSet.Get(ast.MustParseRef(`schema.kubernetes["data-schema"]`)) == nil {
			err = fmt.Errorf("Expected schemas for data in schemaSet but got none")
			return
		}

	})

	return err
}

func TestEvalWithJSONSchema(t *testing.T) {

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

	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema",
		"$id": "http://example.com/example.json",
		"type": "object",
		"title": "The root schema",
		"description": "The root schema comprises the entire JSON document.",
		"required": [
			"foo",
			"b"
		],
		"properties": {
			"foo": {
				"$id": "#/properties/foo",
				"type": "string",
				"title": "The foo schema",
				"description": "An explanation about the purpose of this instance."
			},
			"b": {
				"$id": "#/properties/b",
				"type": "array",
				"title": "The b schema",
				"description": "An explanation about the purpose of this instance.",
				"additionalItems": false,
				"items": {
					"$id": "#/properties/b/items",
					"type": "object",
					"title": "The items schema",
					"description": "An explanation about the purpose of this instance.",
					"required": [
						"a",
						"b",
						"c"
					],
					"properties": {
						"a": {
							"$id": "#/properties/b/items/properties/a",
							"type": "integer",
							"title": "The a schema",
							"description": "An explanation about the purpose of this instance."
						},
						"b": {
							"$id": "#/properties/b/items/properties/b",
							"type": "array",
							"title": "The b schema",
							"description": "An explanation about the purpose of this instance.",
							"additionalItems": false,
							"items": {
								"$id": "#/properties/b/items/properties/b/items",
								"type": "integer",
								"title": "The items schema",
								"description": "An explanation about the purpose of this instance."
							}
						},
						"c": {
							"$id": "#/properties/b/items/properties/c",
							"type": "null",
							"title": "The c schema",
							"description": "An explanation about the purpose of this instance."
						}
					},
					"additionalProperties": false
				}
			}
		},
		"additionalProperties": false
	}`

	query := "input.b[0].a == 1"
	err := testEvalWithSchemaFile(t, input, query, schema, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	policyWithSchemasAnnotation := `
package test
# METADATA
# schemas:
#   - input: schema
p {
	input.foo == 42 # type mismatch
}`
	err = testEvalWithSchemaFile(t, input, query, schema, policyWithSchemasAnnotation, true)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	policyWithInlinedSchemasAnnotation := `
package test
# METADATA
# schemas:
#   - input.foo: {"type": "boolean"}
p {
	input.foo == 42 # type mismatch
}`
	err = testEvalWithSchemaFile(t, input, query, schema, policyWithInlinedSchemasAnnotation, true)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	err = testReadParamWithSchemaDir(input, schema)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestEvalWithInvalidSchemaFile(t *testing.T) {

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

	schema := `{badjson`

	query := "input.b[0].a == 1"
	err := testEvalWithSchemaFile(t, input, query, schema, "", false)
	if err == nil {
		t.Fatalf("expected error but err == nil")
	}

	err = testEvalWithInvalidSchemaFile(input, query, schema)
	if err == nil {
		t.Fatalf("expected error but err == nil")
	}
}

func TestEvalWithSchemaFileWithRemoteRef(t *testing.T) {

	input := `{"metadata": {"clusterName": "NAME"}}`
	schemaFmt := `{
	"type": "object",
	"properties": {
	"metadata": {
		"$ref": "%s/v1.14.0/_definitions.json#/definitions/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta",
		"description": "Standard object's metadata. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata"
		}
	}
}`
	ts := kubeSchemaServer(t)
	t.Cleanup(ts.Close)

	query := "data.p.r"
	files := map[string]string{
		"input.json":  input,
		"schema.json": fmt.Sprintf(schemaFmt, ts.URL),
		"p.rego":      "package p\nr { input.metadata.clusterName == \"NAME\" }",
	}

	t.Run("all remote refs disabled", func(t *testing.T) {
		test.WithTempFS(files, func(path string) {
			params := newEvalCommandParams()
			params.inputPath = filepath.Join(path, "input.json")
			params.schema = &schemaFlags{path: filepath.Join(path, "schema.json")}
			params.capabilities.C = ast.CapabilitiesForThisVersion()
			params.capabilities.C.AllowNet = []string{}
			_ = params.dataPaths.Set(filepath.Join(path, "p.rego"))

			var buf bytes.Buffer
			_, err := eval([]string{query}, params, &buf)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var output presentation.Output
			if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
				t.Fatal(err)
			}
			if exp, act := 1, len(output.Errors); exp != act {
				t.Fatalf("expected %d errors, got %d", exp, act)
			}
			if exp, act := "rego_type_error", output.Errors[0].Code; exp != act {
				t.Errorf("expected code %v, got %v", exp, act)
			}
		})
	})

	t.Run("all remote refs enabled", func(t *testing.T) {
		test.WithTempFS(files, func(path string) {
			params := newEvalCommandParams()
			params.inputPath = filepath.Join(path, "input.json")
			params.schema = &schemaFlags{path: filepath.Join(path, "schema.json")}
			_ = params.dataPaths.Set(filepath.Join(path, "p.rego"))

			var buf bytes.Buffer
			defined, err := eval([]string{query}, params, &buf)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if exp, act := true, defined; exp != act {
				t.Errorf("expected defined %v, got %v", exp, act)
			}
		})
	})

	t.Run("required remote ref host not enabled", func(t *testing.T) {
		test.WithTempFS(files, func(path string) {
			params := newEvalCommandParams()
			params.inputPath = filepath.Join(path, "input.json")
			params.schema = &schemaFlags{path: filepath.Join(path, "schema.json")}
			params.capabilities.C = ast.CapabilitiesForThisVersion()
			params.capabilities.C.AllowNet = []string{"something.else"}
			_ = params.dataPaths.Set(filepath.Join(path, "p.rego"))

			var buf bytes.Buffer
			_, err := eval([]string{query}, params, &buf)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var output presentation.Output
			if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
				t.Fatal(err)
			}
			if exp, act := 1, len(output.Errors); exp != act {
				t.Fatalf("expected %d errors, got %d", exp, act)
			}
			if exp, act := "rego_type_error", output.Errors[0].Code; exp != act {
				t.Errorf("expected code %v, got %v", exp, act)
			}
		})
	})

	t.Run("only required remote ref host enabled", func(t *testing.T) {
		test.WithTempFS(files, func(path string) {
			params := newEvalCommandParams()
			params.inputPath = filepath.Join(path, "input.json")
			params.schema = &schemaFlags{path: filepath.Join(path, "schema.json")}
			params.capabilities.C = ast.CapabilitiesForThisVersion()
			params.capabilities.C.AllowNet = []string{"127.0.0.1"}
			_ = params.dataPaths.Set(filepath.Join(path, "p.rego"))

			var buf bytes.Buffer
			defined, err := eval([]string{query}, params, &buf)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if exp, act := true, defined; exp != act {
				t.Errorf("expected defined %v, got %v", exp, act)
			}
		})
	})
}

func TestBuiltinsCapabilities(t *testing.T) {
	tests := []struct {
		note            string
		policy          string
		query           string
		ruleName        string
		expectedCode    string
		expectedMessage string
	}{
		{
			note:            "rego.metadata.chain() not allowed",
			policy:          "package p\n r := rego.metadata.chain()",
			query:           "data.p",
			ruleName:        "rego.metadata.chain",
			expectedCode:    "rego_type_error",
			expectedMessage: "undefined function rego.metadata.chain",
		},
		{
			note:            "rego.metadata.rule() not allowed",
			policy:          "package p\n r := rego.metadata.rule()",
			query:           "data.p",
			ruleName:        "rego.metadata.rule",
			expectedCode:    "rego_type_error",
			expectedMessage: "undefined function rego.metadata.rule",
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {

			files := map[string]string{
				"p.rego": tc.policy,
			}

			test.WithTempFS(files, func(path string) {
				params := newEvalCommandParams()
				params.capabilities.C = ast.CapabilitiesForThisVersion()
				params.capabilities.C.Builtins = removeBuiltin(params.capabilities.C.Builtins, tc.ruleName)

				_ = params.dataPaths.Set(filepath.Join(path, "p.rego"))

				var buf bytes.Buffer
				_, err := eval([]string{tc.query}, params, &buf)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var output presentation.Output
				if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
					t.Fatal(err)
				}
				if exp, act := 1, len(output.Errors); exp != act {
					t.Fatalf("expected %d errors, got %d", exp, act)
				}
				if code := output.Errors[0].Code; code != tc.expectedCode {
					t.Errorf("expected code '%v', got '%v'", tc.expectedCode, code)
				}
				if msg := output.Errors[0].Message; msg != tc.expectedMessage {
					t.Errorf("expected message '%v', got '%v'", tc.expectedMessage, msg)
				}
			})
		})
	}
}

func removeBuiltin(builtins []*ast.Builtin, name string) []*ast.Builtin {
	var cpy []*ast.Builtin
	for _, builtin := range builtins {
		if builtin.Name != name {
			cpy = append(cpy, builtin)
		}
	}
	return cpy
}

// Nearly identical to TestEvalWithOptimizeBundleData, but uses
// Rego entrypoint annotations instead of explicitly providing
// the entrypoints as CLI arguments.
func TestEvalWithRegoEntrypointAnnotations(t *testing.T) {
	files := map[string]string{
		"test.rego": `
package test
default p = false
# METADATA
# entrypoint: true
p { q }
q { input.x = data.foo }`,
		"data.json": `
{"foo": 1}`,
	}

	test.WithTempFS(files, func(path string) {
		params := newEvalCommandParams()
		if err := params.bundlePaths.Set(path); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		defined, err := eval([]string{"data.test.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestEvalReturnsRegoError(t *testing.T) {
	buf := new(bytes.Buffer)
	_, err := eval([]string{`{k: v | k = ["a", "a"][_]; v = [0,1][_]}`}, newEvalCommandParams(), buf)
	if _, ok := err.(regoError); !ok {
		t.Fatal("expected regoError but got:", err)
	}
}

func TestEvalWithBundleData(t *testing.T) {
	files := map[string]string{
		"x/x.rego":            "package x\np = 1",
		"x/data.json":         `{"b": "bar"}`,
		"other/not-data.json": `{"ignored": "data"}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		if err := params.bundlePaths.Set(path); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		defined, err := eval([]string{"data"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		assertResultSet(t, output.Result, `[[{"x": {"p": 1, "b": "bar"}}]]`)
	})
}

func TestEvalWithBundleDuplicateFileNames(t *testing.T) {
	files := map[string]string{
		// bundle a
		"a/policy.rego": "package a\np = 1",
		"a/.manifest":   `{"roots":["a"]}`,

		// bundle b
		"b/policy.rego": "package b\nq = 1",
		"b/.manifest":   `{"roots":["b"]}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		if err := params.bundlePaths.Set(filepath.Join(path, "a")); err != nil {
			t.Fatal(err)
		}
		if err := params.bundlePaths.Set(filepath.Join(path, "b")); err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer

		defined, err := eval([]string{"data"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}

		var output presentation.Output

		if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
			t.Fatal(err)
		}

		assertResultSet(t, output.Result, `[[{"a":{"p":1},"b":{"q":1}}]]`)
	})
}

func TestEvalWithStrictBuiltinErrors(t *testing.T) {
	params := newEvalCommandParams()
	params.strictBuiltinErrors = true

	var buf bytes.Buffer
	_, err := eval([]string{"1/0"}, params, &buf)
	if err == nil {
		t.Fatal("expected error")
	}

	params.strictBuiltinErrors = false
	buf.Reset()

	_, err = eval([]string{"1/0"}, params, &buf)
	if err != nil {
		t.Fatal("unexpected error:", err)
	}

	if buf.String() != "{}\n" {
		t.Fatal("expected undefined output but got:", buf.String())
	}
}

func assertResultSet(t *testing.T, rs rego.ResultSet, expected string) {
	t.Helper()
	result := []interface{}{}

	for i := range rs {
		values := []interface{}{}
		for j := range rs[i].Expressions {
			values = append(values, rs[i].Expressions[j].Value)
		}
		result = append(result, values)
	}

	parsedExpected := util.MustUnmarshalJSON([]byte(expected))
	if !reflect.DeepEqual(result, parsedExpected) {
		t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", parsedExpected, result)
	}
}

func TestEvalErrorJSONOutput(t *testing.T) {
	params := newEvalCommandParams()
	err := params.outputFormat.Set(evalJSONOutput)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	var buf bytes.Buffer

	defined, err := eval([]string{"{1,2,3} == {1,x,3}"}, params, &buf)
	if defined && err == nil {
		t.Fatalf("Expected an error")
	}

	// Only check that it *can* be loaded as valid JSON, and that the errors
	// are populated.
	var output map[string]interface{}

	if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
		t.Fatal(err)
	}

	if output["errors"] == nil {
		t.Fatalf("Expected error to be non-nil")
	}
}

func TestEvalDebugTraceJSONOutput(t *testing.T) {
	params := newEvalCommandParams()
	err := params.outputFormat.Set(evalJSONOutput)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	err = params.explain.Set(explainModeFull)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	params.disableIndexing = true

	mod := `package x

	p[a] {
		a := input.z
		a == 1
	}

	p[b] {
		b := input.y
		b == 1
	}
	`

	input := `{"z": 1}`

	files := map[string]string{
		"policy.rego": mod,
		"input.json":  input,
	}

	var buf bytes.Buffer
	var policyFile string

	test.WithTempFS(files, func(path string) {
		params.inputPath = filepath.Join(path, "input.json")
		policyFile = filepath.Join(path, "policy.rego")
		err := params.dataPaths.Set(policyFile)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}

		_, err = eval([]string{"data.x.p"}, params, &buf)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
	})

	var output struct {
		Explanation []struct {
			Op            string                   `json:"Op"`
			Node          interface{}              `json:"Node"`
			Location      *ast.Location            `json:"Location"`
			Locals        []map[string]interface{} `json:"Locals"`
			LocalMetadata map[string]struct {
				Name string `json:"name"`
			} `json:"LocalMetadata"`
		}
	}

	if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
		t.Fatal(err)
	}
	if len(output.Explanation) == 0 {
		t.Fatalf("Expected explanations to be non-nil")
	}

	type locationAndVars struct {
		location    *ast.Location
		varBindings map[string]string
	}

	var evals []locationAndVars
	for _, e := range output.Explanation {
		if e.Op == string(topdown.EvalOp) {
			bindings := map[string]string{}
			for k, v := range e.LocalMetadata {
				bindings[k] = v.Name
			}

			evals = append(evals, locationAndVars{location: e.Location, varBindings: bindings})
		}
	}

	expectedEvalLocationsAndVars := []locationAndVars{
		{
			location:    ast.NewLocation(nil, policyFile, 4, 3), // a := input.z
			varBindings: map[string]string{"__local0__": "a"},
		},
		{
			location:    ast.NewLocation(nil, policyFile, 5, 3), // a == 1
			varBindings: map[string]string{"__local0__": "a"},
		},
		{
			location:    ast.NewLocation(nil, policyFile, 9, 3), // b := input.y
			varBindings: map[string]string{"__local1__": "b"},
		},
	}

	for _, expected := range expectedEvalLocationsAndVars {
		found := false
		for _, actual := range evals {
			if expected.location.Compare(actual.location) == 0 {
				found = true
				if !reflect.DeepEqual(expected.varBindings, actual.varBindings) {
					t.Errorf("Expected var bindings:\n\n\t%+v\n\nGot\n\n\t%+v\n\n", expected.varBindings, actual.varBindings)
				}
			}
		}
		if !found {
			t.Fatalf("Missing expected eval node in trace: %+v\nGot: %+v\n", expected, evals)
		}
	}
}

func TestEvalPrettyTrace(t *testing.T) {
	tests := []struct {
		note        string
		query       string
		includeVars bool
		files       map[string]string
		expected    string
	}{
		{
			note:        "simple without vars",
			query:       "data.test.p",
			includeVars: false,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

p if {
	x := 1
	y := 2
	z := 3
	x == z - y
} 
`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%         Enter data.test.p = _
query:1 %.*%         | Eval data.test.p = _
query:1 %.*%         | Index data.test.p (matched 1 rule, early exit)
%.*%/test.rego:4     | Enter data.test.p
%.*%/test.rego:5     | | Eval x = 1
%.*%/test.rego:6     | | Eval y = 2
%.*%/test.rego:7     | | Eval z = 3
%.*%/test.rego:8     | | Eval minus(z, y, __local3__)
%.*%/test.rego:8     | | Eval x = __local3__
%.*%/test.rego:4     | | Exit data.test.p early
query:1 %.*%         | Exit data.test.p = _
query:1 %.*%         Redo data.test.p = _
query:1 %.*%         | Redo data.test.p = _
%.*%/test.rego:4     | Redo data.test.p
%.*%/test.rego:8     | | Redo x = __local3__
%.*%/test.rego:8     | | Redo minus(z, y, __local3__)
%.*%/test.rego:7     | | Redo z = 3
%.*%/test.rego:6     | | Redo y = 2
%.*%/test.rego:5     | | Redo x = 1
true
`,
		},
		{
			note:        "simple with vars",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

p if {
	x := 1
	y := 2
	z := 3
	x == z - y
} 
`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%         Enter data.test.p = _                                {}
query:1 %.*%         | Eval data.test.p = _                               {}
query:1 %.*%         | Index data.test.p (matched 1 rule, early exit)     {}
%.*%/test.rego:4     | Enter data.test.p                                  {}
%.*%/test.rego:5     | | Eval x = 1                                       {}
%.*%/test.rego:6     | | Eval y = 2                                       {}
%.*%/test.rego:7     | | Eval z = 3                                       {}
%.*%/test.rego:8     | | Eval minus(z, y, __local3__)                     {y: 2, z: 3}
%.*%/test.rego:8     | | Eval x = __local3__                              {__local3__: 1, x: 1}
%.*%/test.rego:4     | | Exit data.test.p early                           {}
query:1 %.*%         | Exit data.test.p = _                               {_: true, data.test.p: true}
query:1 %.*%         Redo data.test.p = _                                 {_: true, data.test.p: true}
query:1 %.*%         | Redo data.test.p = _                               {_: true, data.test.p: true}
%.*%/test.rego:4     | Redo data.test.p                                   {}
%.*%/test.rego:8     | | Redo x = __local3__                              {__local3__: 1, x: 1}
%.*%/test.rego:8     | | Redo minus(z, y, __local3__)                     {__local3__: 1, y: 2, z: 3}
%.*%/test.rego:7     | | Redo z = 3                                       {z: 3}
%.*%/test.rego:6     | | Redo y = 2                                       {y: 2}
%.*%/test.rego:5     | | Redo x = 1                                       {x: 1}
true
`,
		},
		{
			note:        "large var",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

v := {
		"foo": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"],
		"bar": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"],
		"baz": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"],
		"qux": ["a", "b", "c", "d", "e", "f", "g", "h", "i", "j"],
	}

p if {
	x := v

	x.foo[_] == "a"
} 
`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%          Enter data.test.p = _                                  {}
query:1 %.*%          | Eval data.test.p = _                                 {}
query:1 %.*%          | Index data.test.p (matched 1 rule, early exit)       {}
%.*%/test.rego:11     | Enter data.test.p                                    {}
%.*%/test.rego:12     | | Eval x = data.test.v                               {}
%.*%/test.rego:12     | | Index data.test.v (matched 1 rule, early exit)     {}
%.*%/test.rego:4      | | Enter data.test.v                                  {}
%.*%/test.rego:4      | | | Eval true                                        {}
%.*%/test.rego:4      | | | Exit data.test.v early                           {}
%.*%/test.rego:14     | | Eval x.foo[_] = "a"                                {x: {"bar": ["a", "b", "c", "d", ...}
%.*%/test.rego:11     | | Exit data.test.p early                             {}
query:1 %.*%          | Exit data.test.p = _                                 {_: true, data.test.p: true}
query:1 %.*%          Redo data.test.p = _                                   {_: true, data.test.p: true}
query:1 %.*%          | Redo data.test.p = _                                 {_: true, data.test.p: true}
%.*%/test.rego:11     | Redo data.test.p                                     {}
%.*%/test.rego:14     | | Redo x.foo[_] = "a"                                {_: 0, x: {"bar": ["a", "b", "c", "d", ...}
%.*%/test.rego:12     | | Redo x = data.test.v                               {data.test.v: {"bar": ["a", "b", "c", "d", ..., x: {"bar": ["a", "b", "c", "d", ...}
%.*%/test.rego:4      | | | Redo true                                        {}
true
`,
		},
		{
			note:        "func call",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

p if {
	x := 1
	y := 2
	z := 3
	z == f(x, y)
}

f(a, b) := c if {
	c := a + b
}
`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%          Enter data.test.p = _                                {}
query:1 %.*%          | Eval data.test.p = _                               {}
query:1 %.*%          | Index data.test.p (matched 1 rule, early exit)     {}
%.*%/test.rego:4      | Enter data.test.p                                  {}
%.*%/test.rego:5      | | Eval x = 1                                       {}
%.*%/test.rego:6      | | Eval y = 2                                       {}
%.*%/test.rego:7      | | Eval z = 3                                       {}
%.*%/test.rego:8      | | Eval data.test.f(x, y, __local6__)               {x: 1, y: 2}
%.*%/test.rego:8      | | Index data.test.f (matched 1 rule)               {x: 1, y: 2}
%.*%/test.rego:11     | | Enter data.test.f                                {}
%.*%/test.rego:12     | | | Eval plus(a, b, __local7__)                    {a: 1, b: 2}
%.*%/test.rego:12     | | | Eval c = __local7__                            {__local7__: 3}
%.*%/test.rego:11     | | | Exit data.test.f                               {a: 1, b: 2, c: 3}
%.*%/test.rego:8      | | Eval z = __local6__                              {__local6__: 3, z: 3}
%.*%/test.rego:4      | | Exit data.test.p early                           {}
query:1 %.*%          | Exit data.test.p = _                               {_: true, data.test.p: true}
query:1 %.*%          Redo data.test.p = _                                 {_: true, data.test.p: true}
query:1 %.*%          | Redo data.test.p = _                               {_: true, data.test.p: true}
%.*%/test.rego:4      | Redo data.test.p                                   {}
%.*%/test.rego:8      | | Redo z = __local6__                              {__local6__: 3, z: 3}
%.*%/test.rego:8      | | Redo data.test.f(x, y, __local6__)               {__local6__: 3, x: 1, y: 2}
%.*%/test.rego:12     | | | Redo c = __local7__                            {__local7__: 3, c: 3}
%.*%/test.rego:12     | | | Redo plus(a, b, __local7__)                    {__local7__: 3, a: 1, b: 2}
%.*%/test.rego:7      | | Redo z = 3                                       {z: 3}
%.*%/test.rego:6      | | Redo y = 2                                       {y: 2}
%.*%/test.rego:5      | | Redo x = 1                                       {x: 1}
true
`,
		},
		{
			note:        "every",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

p if {
	l := ["a", "b", "c"]
	every x in l {
		count(x) == 1
	}
}

f(a, b) := c if {
	c := a + b
}
`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%         Enter data.test.p = _                                                         {}
query:1 %.*%         | Eval data.test.p = _                                                        {}
query:1 %.*%         | Index data.test.p (matched 1 rule, early exit)                              {}
%.*%/test.rego:4     | Enter data.test.p                                                           {}
%.*%/test.rego:5     | | Eval l = ["a", "b", "c"]                                                  {}
%.*%/test.rego:6     | | Eval __local6__ = l                                                       {l: ["a", "b", "c"]}
%.*%/test.rego:6     | | Eval every x in __local6__ { count(x, __local7__); __local7__ = 1 }       {__local6__: ["a", "b", "c"]}
%.*%/test.rego:6     | | Enter every x in __local6__ { count(x, __local7__); __local7__ = 1 }      {__local6__: ["a", "b", "c"]}
%.*%/test.rego:6     | | | Eval __local6__[__local1__] = x                                         {__local6__: ["a", "b", "c"]}
%.*%/test.rego:7     | | | Enter count(x, __local7__); __local7__ = 1                              {x: "a"}
%.*%/test.rego:7     | | | | Eval count(x, __local7__)                                             {x: "a"}
%.*%/test.rego:7     | | | | Eval __local7__ = 1                                                   {__local7__: 1}
%.*%/test.rego:7     | | | | Exit count(x, __local7__); __local7__ = 1 early                       {__local7__: 1, x: "a"}
%.*%/test.rego:7     | | | Redo count(x, __local7__); __local7__ = 1                               {__local7__: 1, x: "a"}
%.*%/test.rego:7     | | | | Redo __local7__ = 1                                                   {__local7__: 1}
%.*%/test.rego:7     | | | | Redo count(x, __local7__)                                             {__local7__: 1, x: "a"}
%.*%/test.rego:6     | | | Redo every x in __local6__ { count(x, __local7__); __local7__ = 1 }     {__local1__: 0, __local6__: ["a", "b", "c"], x: "a"}
%.*%/test.rego:6     | | | Redo __local6__[__local1__] = x                                         {__local1__: 0, __local6__: ["a", "b", "c"], x: "a"}
%.*%/test.rego:7     | | | Enter count(x, __local7__); __local7__ = 1                              {x: "b"}
%.*%/test.rego:7     | | | | Eval count(x, __local7__)                                             {x: "b"}
%.*%/test.rego:7     | | | | Eval __local7__ = 1                                                   {__local7__: 1}
%.*%/test.rego:7     | | | | Exit count(x, __local7__); __local7__ = 1 early                       {__local7__: 1, x: "b"}
%.*%/test.rego:7     | | | Redo count(x, __local7__); __local7__ = 1                               {__local7__: 1, x: "b"}
%.*%/test.rego:7     | | | | Redo __local7__ = 1                                                   {__local7__: 1}
%.*%/test.rego:7     | | | | Redo count(x, __local7__)                                             {__local7__: 1, x: "b"}
%.*%/test.rego:6     | | | Redo every x in __local6__ { count(x, __local7__); __local7__ = 1 }     {__local1__: 1, __local6__: ["a", "b", "c"], x: "b"}
%.*%/test.rego:6     | | | Redo __local6__[__local1__] = x                                         {__local1__: 1, __local6__: ["a", "b", "c"], x: "b"}
%.*%/test.rego:7     | | | Enter count(x, __local7__); __local7__ = 1                              {x: "c"}
%.*%/test.rego:7     | | | | Eval count(x, __local7__)                                             {x: "c"}
%.*%/test.rego:7     | | | | Eval __local7__ = 1                                                   {__local7__: 1}
%.*%/test.rego:7     | | | | Exit count(x, __local7__); __local7__ = 1 early                       {__local7__: 1, x: "c"}
%.*%/test.rego:7     | | | Redo count(x, __local7__); __local7__ = 1                               {__local7__: 1, x: "c"}
%.*%/test.rego:7     | | | | Redo __local7__ = 1                                                   {__local7__: 1}
%.*%/test.rego:7     | | | | Redo count(x, __local7__)                                             {__local7__: 1, x: "c"}
%.*%/test.rego:6     | | | Redo every x in __local6__ { count(x, __local7__); __local7__ = 1 }     {__local1__: 2, __local6__: ["a", "b", "c"], x: "c"}
%.*%/test.rego:6     | | | Redo __local6__[__local1__] = x                                         {__local1__: 2, __local6__: ["a", "b", "c"], x: "c"}
%.*%/test.rego:4     | | Exit data.test.p early                                                    {}
query:1 %.*%         | Exit data.test.p = _                                                        {_: true, data.test.p: true}
query:1 %.*%         Redo data.test.p = _                                                          {_: true, data.test.p: true}
query:1 %.*%         | Redo data.test.p = _                                                        {_: true, data.test.p: true}
%.*%/test.rego:4     | Redo data.test.p                                                            {}
%.*%/test.rego:6     | | Redo every x in __local6__ { count(x, __local7__); __local7__ = 1 }       {__local6__: ["a", "b", "c"]}
%.*%/test.rego:6     | | | Exit every x in __local6__ { count(x, __local7__); __local7__ = 1 }     {__local6__: ["a", "b", "c"]}
%.*%/test.rego:6     | | Redo __local6__ = l                                                       {__local6__: ["a", "b", "c"], l: ["a", "b", "c"]}
%.*%/test.rego:5     | | Redo l = ["a", "b", "c"]                                                  {l: ["a", "b", "c"]}
true
`,
		},
		{
			note:        "rule value",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

a := 1

p if {
	a + 1 == 2
	a + 2 == 3
}
`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%         Enter data.test.p = _                                  {}
query:1 %.*%         | Eval data.test.p = _                                 {}
query:1 %.*%         | Index data.test.p (matched 1 rule, early exit)       {}
%.*%/test.rego:6     | Enter data.test.p                                    {}
%.*%/test.rego:7     | | Eval __local2__ = data.test.a                      {}
%.*%/test.rego:7     | | Index data.test.a (matched 1 rule, early exit)     {}
%.*%/test.rego:4     | | Enter data.test.a                                  {}
%.*%/test.rego:4     | | | Eval true                                        {}
%.*%/test.rego:4     | | | Exit data.test.a early                           {}
%.*%/test.rego:7     | | Eval plus(__local2__, 1, __local0__)               {__local2__: 1}
%.*%/test.rego:7     | | Eval __local0__ = 2                                {__local0__: 2}
%.*%/test.rego:8     | | Eval __local3__ = data.test.a                      {data.test.a: 1}
%.*%/test.rego:8     | | Index data.test.a (matched 1 rule, early exit)     {data.test.a: 1}
%.*%/test.rego:8     | | Eval plus(__local3__, 2, __local1__)               {__local3__: 1}
%.*%/test.rego:8     | | Eval __local1__ = 3                                {__local1__: 3}
%.*%/test.rego:6     | | Exit data.test.p early                             {}
query:1 %.*%         | Exit data.test.p = _                                 {_: true, data.test.p: true}
query:1 %.*%         Redo data.test.p = _                                   {_: true, data.test.p: true}
query:1 %.*%         | Redo data.test.p = _                                 {_: true, data.test.p: true}
%.*%/test.rego:6     | Redo data.test.p                                     {}
%.*%/test.rego:8     | | Redo __local1__ = 3                                {__local1__: 3}
%.*%/test.rego:8     | | Redo plus(__local3__, 2, __local1__)               {__local1__: 3, __local3__: 1}
%.*%/test.rego:8     | | Redo __local3__ = data.test.a                      {__local3__: 1, data.test.a: 1}
%.*%/test.rego:7     | | Redo __local0__ = 2                                {__local0__: 2}
%.*%/test.rego:7     | | Redo plus(__local2__, 1, __local0__)               {__local0__: 2, __local2__: 1}
%.*%/test.rego:7     | | Redo __local2__ = data.test.a                      {__local2__: 1, data.test.a: 1}
%.*%/test.rego:4     | | | Redo true                                        {}
true
`,
		},
		{
			note:        "input values",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

p if {
	input.x == 1
	input.x + input.y == input.z
}
`,
				"input.json": `{
	"x": 1,
	"y": 2,
	"z": 3
}`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%         Enter data.test.p = _                                 {}
query:1 %.*%         | Eval data.test.p = _                                {}
query:1 %.*%         | Index data.test.p (matched 1 rule, early exit)      {}
%.*%/test.rego:4     | Enter data.test.p                                   {}
%.*%/test.rego:5     | | Eval input.x = 1                                  {}
%.*%/test.rego:6     | | Eval __local1__ = input.x                         {}
%.*%/test.rego:6     | | Eval __local2__ = input.y                         {}
%.*%/test.rego:6     | | Eval plus(__local1__, __local2__, __local0__)     {__local1__: 1, __local2__: 2}
%.*%/test.rego:6     | | Eval __local0__ = input.z                         {__local0__: 3}
%.*%/test.rego:4     | | Exit data.test.p early                            {}
query:1 %.*%         | Exit data.test.p = _                                {_: true, data.test.p: true}
query:1 %.*%         Redo data.test.p = _                                  {_: true, data.test.p: true}
query:1 %.*%         | Redo data.test.p = _                                {_: true, data.test.p: true}
%.*%/test.rego:4     | Redo data.test.p                                    {}
%.*%/test.rego:6     | | Redo __local0__ = input.z                         {__local0__: 3}
%.*%/test.rego:6     | | Redo plus(__local1__, __local2__, __local0__)     {__local0__: 3, __local1__: 1, __local2__: 2}
%.*%/test.rego:6     | | Redo __local2__ = input.y                         {__local2__: 2}
%.*%/test.rego:6     | | Redo __local1__ = input.x                         {__local1__: 1}
%.*%/test.rego:5     | | Redo input.x = 1                                  {}
true
`,
		},
		{
			note:        "data values",
			query:       "data.test.p",
			includeVars: true,
			files: map[string]string{
				"test.rego": `package test
import rego.v1

p if {
	data.x == 1
	data.x + data.y == data.z
}
`,
				"data.json": `{
	"x": 1,
	"y": 2,
	"z": 3
}`,
			},
			expected: `%SKIP_LINE%
query:1 %.*%         Enter data.test.p = _                                 {}
query:1 %.*%         | Eval data.test.p = _                                {}
query:1 %.*%         | Index data.test.p (matched 1 rule, early exit)      {}
%.*%/test.rego:4     | Enter data.test.p                                   {}
%.*%/test.rego:5     | | Eval data.x = 1                                   {}
%.*%/test.rego:6     | | Eval __local1__ = data.x                          {}
%.*%/test.rego:6     | | Eval __local2__ = data.y                          {}
%.*%/test.rego:6     | | Eval plus(__local1__, __local2__, __local0__)     {__local1__: 1, __local2__: 2}
%.*%/test.rego:6     | | Eval __local0__ = data.z                          {__local0__: 3}
%.*%/test.rego:4     | | Exit data.test.p early                            {}
query:1 %.*%         | Exit data.test.p = _                                {_: true, data.test.p: true}
query:1 %.*%         Redo data.test.p = _                                  {_: true, data.test.p: true}
query:1 %.*%         | Redo data.test.p = _                                {_: true, data.test.p: true}
%.*%/test.rego:4     | Redo data.test.p                                    {}
%.*%/test.rego:6     | | Redo __local0__ = data.z                          {__local0__: 3}
%.*%/test.rego:6     | | Redo plus(__local1__, __local2__, __local0__)     {__local0__: 3, __local1__: 1, __local2__: 2}
%.*%/test.rego:6     | | Redo __local2__ = data.y                          {__local2__: 2}
%.*%/test.rego:6     | | Redo __local1__ = data.x                          {__local1__: 1}
%.*%/test.rego:5     | | Redo data.x = 1                                   {}
true
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			var buf bytes.Buffer

			test.WithTempFS(tc.files, func(path string) {
				params := newEvalCommandParams()
				_ = params.bundlePaths.Set(path)
				inputFile := filepath.Join(path, "input.json")
				if _, err := os.Stat(inputFile); err == nil {
					params.inputPath = inputFile
				}
				_ = params.outputFormat.Set(evalPrettyOutput)
				_ = params.explain.Set(explainModeFull)
				params.traceVarValues = tc.includeVars
				params.disableIndexing = true
				_ = params.bundlePaths.Set(path)

				_, err := eval([]string{tc.query}, params, &buf)
				if err != nil {
					t.Fatalf("Unexpected error: %s\n\n%s", err, buf.String())
				}
			})

			actual := buf.String()
			if !stringsMatch(t, tc.expected, actual) {
				t.Fatalf("Expected:\n\n%v\n\nGot:\n\n%v", tc.expected, actual)
			}
		})
	}
}

func stringsMatch(t *testing.T, expected, actual string) bool {
	t.Helper()

	var expectedLines []string
	for _, l := range strings.Split(expected, "\n") {
		if !strings.Contains(l, "%SKIP_LINE%") {
			expectedLines = append(expectedLines, l)
		}
	}

	actualLines := strings.Split(actual, "\n")

	if len(expectedLines) != len(actualLines) {
		t.Errorf("Expected %d lines but got %d", len(expectedLines), len(actualLines))
		return false
	}

	for i, expectedLine := range expectedLines {
		actualLine := actualLines[i]

		expectedParts := strings.Split(expectedLine, "%.*%")
		if len(expectedParts) == 1 {
			if expectedLine != actualLine {
				t.Errorf("Mismatch on line %d. Expected:\n\n%s\n\nGot:\n\n%s", i, expectedLine, actualLine)
				return false
			}
		} else if len(expectedParts) == 2 {
			if !strings.HasPrefix(actualLine, expectedParts[0]) {
				t.Errorf("Expected line %d to start with:\n\n%s\n\nbut got:\n\n%s", i, expectedParts[0], actualLine)
				return false
			}
			if !strings.HasSuffix(actualLine, expectedParts[1]) {
				t.Errorf("Expected line %d to end with:\n\n%s\n\nbut got:\n\n%s", i, expectedParts[1], actualLine)
				return false
			}
		} else {
			t.Fatalf("At most one .* is allowed per line but found %d on line %d:\n\n%s", len(expectedParts)-1, i, expectedLine)
			return false
		}
	}

	return true
}

func TestResetExprLocations(t *testing.T) {

	// Make sure no panic if passed nil.
	resetExprLocations(nil)

	// Run partial evaluation on this fake module and check results.
	// The content of the module is not very important it just has to generate
	// support and cases where the locaiton is unset. The default causes support
	// and exprs with no location information.
	pq, err := rego.New(rego.Query("data.test.p = x"), rego.Module("test.rego", `

		package test

		default p = false

		p {
			input.x = q[_]
		}

		q[1]
		q[2]
		`)).Partial(context.Background())

	if err != nil {
		t.Fatal(err)
	}

	resetExprLocations(pq)

	var exp int

	vis := ast.NewGenericVisitor(func(x interface{}) bool {
		if expr, ok := x.(*ast.Expr); ok {
			if expr.Location.Row != exp {
				t.Fatalf("Expected %v to have row %v but got %v", expr, exp, expr.Location.Row)
			}
			exp++
		}
		return false
	})

	for i := range pq.Queries {
		vis.Walk(pq.Queries[i])
	}

	for i := range pq.Support {
		vis.Walk(pq.Support[i])
	}

}
func kubeSchemaServer(t *testing.T) *httptest.Server {
	t.Helper()
	bs, err := os.ReadFile("../ast/testdata/_definitions.json")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write(bs)
		if err != nil {
			panic(err)
		}
	}))
	return ts
}

func TestEvalPartialFormattedOutput(t *testing.T) {

	query := `time.clock(input.x) == time.clock(input.y)`
	tests := []struct {
		format, expected string
	}{
		{
			format: evalPrettyOutput,
			expected: `+---------+------------------------------------------+
| Query 1 | time.clock(input.y, time.clock(input.x)) |
+---------+------------------------------------------+
`},
		{
			format: evalSourceOutput,
			expected: `# Query 1
time.clock(input.y, time.clock(input.x))

`},
	}

	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			buf := new(bytes.Buffer)
			params := newEvalCommandParams()
			params.partial = true
			_ = params.outputFormat.Set(tc.format)
			_, err := eval([]string{query}, params, buf)
			if err != nil {
				t.Fatal("unexpected error:", err)
			}
			if actual := buf.String(); actual != tc.expected {
				t.Errorf("expected output %q\ngot %q", tc.expected, actual)
			}
		})
	}
}

func TestEvalPartialRegoVersionOutput(t *testing.T) {
	tests := []struct {
		note                string
		regoV1ImportCapable bool
		v1Compatible        bool
		query               string
		module              string
		expected            string
	}{
		{
			note:                "v0, no future keywords",
			regoV1ImportCapable: true,
			query:               "data.test.p",
			module: `package test

p[v] {
	v := input.v
}
`,
			expected: `# Query 1
data.partial.test.p = _term_0_0
_term_0_0

# Module 1
package partial.test

import rego.v1

p contains __local0__1 if __local0__1 = input.v
`,
		},
		{
			note:                "v0, no future keywords, not rego.v1 import capable",
			regoV1ImportCapable: false,
			query:               "data.test.p",
			module: `package test

p[v] {
	v := input.v
}
`,
			expected: `# Query 1
data.partial.test.p = _term_0_0
_term_0_0

# Module 1
package partial.test

p[__local0__1] {
	__local0__1 = input.v
}
`,
		},
		{
			note:                "v0, future keywords",
			regoV1ImportCapable: true,
			query:               "data.test.p",
			module: `package test

import rego.v1

p contains v if {
	v := input.v
}
`,
			expected: `# Query 1
data.partial.test.p = _term_0_0
_term_0_0

# Module 1
package partial.test

import rego.v1

p contains __local0__1 if __local0__1 = input.v
`,
		},
		{
			note:                "v1",
			regoV1ImportCapable: true,
			v1Compatible:        true,
			query:               "data.test.p",
			module: `package test

p contains v if {
	v := input.v
}
`,
			expected: `# Query 1
data.partial.test.p = _term_0_0
_term_0_0

# Module 1
package partial.test

p contains __local0__1 if __local0__1 = input.v
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			files := map[string]string{
				"test.rego": tc.module,
			}

			test.WithTempFS(files, func(path string) {
				params := newEvalCommandParams()
				_ = params.dataPaths.Set(filepath.Join(path, "test.rego"))
				params.partial = true
				params.shallowInlining = true
				params.v1Compatible = tc.v1Compatible
				_ = params.outputFormat.Set(evalSourceOutput)

				if !tc.regoV1ImportCapable {
					caps := newcapabilitiesFlag()
					caps.C = ast.CapabilitiesForThisVersion()
					caps.C.Features = []string{
						ast.FeatureRefHeadStringPrefixes,
						ast.FeatureRefHeads,
					}
					params.capabilities = caps
				}

				buf := new(bytes.Buffer)
				_, err := eval([]string{tc.query}, params, buf)
				if err != nil {
					t.Fatal("unexpected error:", err)
				}
				if actual := buf.String(); actual != tc.expected {
					t.Errorf("expected output %q\ngot %q", tc.expected, actual)
				}
			})
		})
	}
}

func TestEvalDiscardOutput(t *testing.T) {
	tests := map[string]struct {
		query, format, expected string
		params                  evalCommandParams
	}{
		"success example": {
			query: "1*2+3",
			params: func() evalCommandParams {
				params := newEvalCommandParams()
				err := params.outputFormat.Set(evalDiscardOutput)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				return params
			}(),
			expected: `{
  "result": "discarded"
}
`},
		"error example": {
			query: "1/0",
			params: func() evalCommandParams {
				params := newEvalCommandParams()
				err := params.outputFormat.Set(evalDiscardOutput)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				return params
			}(),
			expected: `{}
`},
		"error example show built-in-errors": {
			query: "1/0",
			params: func() evalCommandParams {
				params := newEvalCommandParams()
				err := params.outputFormat.Set(evalDiscardOutput)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				params.showBuiltinErrors = true
				return params
			}(),
			expected: `{
  "errors": [
    {
      "code": "eval_builtin_error",
      "location": {
        "col": 1,
        "file": "",
        "row": 1
      },
      "message": "div: divide by zero"
    }
  ]
}
`},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := eval([]string{tc.query}, tc.params, &buf)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if actual := buf.String(); actual != tc.expected {
				t.Errorf("expected output %q\ngot %q", tc.expected, actual)
			}
		})
	}
}

func TestEvalDiscardProfilerOutput(t *testing.T) {
	params := newEvalCommandParams()
	err := params.outputFormat.Set(evalDiscardOutput)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	params.profile = true

	query := "1*2+3"

	var buf bytes.Buffer
	_, err = eval([]string{query}, params, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	var output map[string]interface{}
	if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
		t.Fatal(err)
	}

	// assert that the result is set to discarded
	result, ok := output["result"].(string)
	if !ok {
		t.Fatal("error extracting result as string from output")
	}

	if result != "discarded" {
		t.Fatal("Expected result field to be set to 'discarded'")
	}

	// assert that profile is still set
	_, ok = output["profile"]
	if !ok {
		t.Fatal("error in parsing profile output")
	}
}

func TestPolicyWithStrictFlag(t *testing.T) {
	testsShouldError := []struct {
		note            string
		policy          string
		query           string
		expectedCode    string
		expectedMessage string
	}{
		{
			note: "strict mode should error on duplicate imports",
			policy: `package x
			import future.keywords.if
			import future.keywords.if
			foo = 2`,
			query:           "data.foo",
			expectedCode:    "rego_compile_error",
			expectedMessage: "import must not shadow import future.keywords.if",
		},
		{
			note: "strict mode should error on unused imports",
			policy: `package x
			import future.keywords.if
			import data.foo
			foo = 2`,
			query:           "data.foo",
			expectedCode:    "rego_compile_error",
			expectedMessage: "import data.foo unused",
		},
		{
			note: "strict mode should error when reserved vars data or input is used",
			policy: `package x
			import future.keywords.if
			data if { x = 1}`,
			query:           "data.foo",
			expectedCode:    "rego_compile_error",
			expectedMessage: "rules must not shadow data (use a different rule name)",
		},
	}

	for _, tc := range testsShouldError {
		t.Run(tc.note, func(t *testing.T) {

			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(path string) {
				params := newEvalCommandParams()
				params.strict = true

				_ = params.dataPaths.Set(filepath.Join(path, "test.rego"))

				var buf bytes.Buffer
				_, err := eval([]string{tc.query}, params, &buf)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var output presentation.Output
				if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
					t.Fatal(err)
				}

				if code := output.Errors[0].Code; code != tc.expectedCode {
					t.Errorf("expected code '%v', got '%v'", tc.expectedCode, code)
				}
				if msg := output.Errors[0].Message; msg != tc.expectedMessage {
					t.Errorf("expected message '%v', got '%v'", tc.expectedMessage, msg)
				}
			})
		})
	}

	testsShouldPass := []struct {
		note   string
		policy string
		query  string
	}{
		{
			note: "This should not error as it is valid",
			policy: `package x
			import future.keywords.if
			foo = 2`,
			query: "data.foo",
		},
		{
			note: "Strict mode should not validate the query, only the policy, this should not error",
			policy: `package x
			import future.keywords.if
			foo = 2`,
			query: "x := data.x.foo",
		},
	}
	for _, tc := range testsShouldPass {
		t.Run(tc.note, func(t *testing.T) {

			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(path string) {
				params := newEvalCommandParams()
				params.strict = true

				var buf bytes.Buffer
				_, err := eval([]string{tc.query}, params, &buf)
				if err != nil {
					t.Errorf("Should not error, got error: '%v'", err)
				}
			})
		})
	}

}

func TestBundleWithStrictFlag(t *testing.T) {
	testsShouldError := []struct {
		note            string
		policy          string
		query           string
		expectedCode    string
		expectedMessage string
	}{
		{
			note: "strict mode should error on duplicate imports in this bundle",
			policy: `package x
			import future.keywords.if
			import future.keywords.if
			foo = 2`,
			query:           "data.foo",
			expectedCode:    "rego_compile_error",
			expectedMessage: "import must not shadow import future.keywords.if",
		},
		{
			note: "strict mode should error on unused imports in this bundle",
			policy: `package x
			import future.keywords.if
			import data.foo
			foo = 2`,
			query:           "data.foo",
			expectedCode:    "rego_compile_error",
			expectedMessage: "import data.foo unused",
		},
		{
			note: "strict mode should error when reserved vars data or input is used in this bundle",
			policy: `package x
			import future.keywords.if
			data if { x = 1}`,
			query:           "data.foo",
			expectedCode:    "rego_compile_error",
			expectedMessage: "rules must not shadow data (use a different rule name)",
		},
	}

	for _, tc := range testsShouldError {
		t.Run(tc.note, func(t *testing.T) {

			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(path string) {
				params := newEvalCommandParams()
				if err := params.bundlePaths.Set(path); err != nil {
					t.Fatal(err)
				}
				params.strict = true

				var buf bytes.Buffer
				_, err := eval([]string{tc.query}, params, &buf)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				var output presentation.Output
				if err := util.NewJSONDecoder(&buf).Decode(&output); err != nil {
					t.Fatal(err)
				}

				if code := output.Errors[0].Code; code != tc.expectedCode {
					t.Errorf("expected code '%v', got '%v'", tc.expectedCode, code)
				}
				if msg := output.Errors[0].Message; msg != tc.expectedMessage {
					t.Errorf("expected message '%v', got '%v'", tc.expectedMessage, msg)
				}
			})
		})
	}

	testsShouldPass := []struct {
		note   string
		policy string
		query  string
	}{
		{
			note: "This bundle should not error as it is valid",
			policy: `package x
			import future.keywords.if
			foo = 2`,
			query: "data.foo",
		},
		{
			note: "Strict mode should not validate the query, only the policy, this bundle should not error",
			policy: `package x
			import future.keywords.if
			foo = 2`,
			query: "x := data.x.foo",
		},
	}
	for _, tc := range testsShouldPass {
		t.Run(tc.note, func(t *testing.T) {

			files := map[string]string{
				"test.rego": tc.policy,
			}

			test.WithTempFS(files, func(path string) {
				params := newEvalCommandParams()
				if err := params.bundlePaths.Set(path); err != nil {
					t.Fatal(err)
				}
				params.strict = true

				var buf bytes.Buffer
				_, err := eval([]string{tc.query}, params, &buf)
				if err != nil {
					t.Errorf("Should not error, got error: '%v'", err)
				}
			})
		})
	}

}

func TestIfElseIfElseNoBrace(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else := 1 if false
			else := 2`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.bug.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestIfElseIfElseBrace(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else := 1 if { false }
			else := 2`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.bug.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestIfElse(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else := 1 `,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.bug.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestElseNoIf(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else = x {
				x=2
			} `,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.bug.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestElseIf(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else := x if {
				x=2
			} `,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.bug.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestElseIfElse(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else := x if {
				x=2
				1==2
			} else =x {
				x=3
			}`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		defined, err := eval([]string{"data.bug.p"}, params, &buf)
		if !defined || err != nil {
			t.Fatalf("Unexpected undefined or error: %v", err)
		}
	})
}

func TestUnexpectedElseIfElseErr(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			p if false
			else := x if {
				x=2
				1==2
			} else
				x=3
			`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		_, err := eval([]string{"data.bug.p"}, params, &buf)

		// Check if there was an error
		if err == nil {
			t.Fatalf("expected an error, but got nil")
		}

		// Check the error message
		errorMessage := err.Error()
		expectedErrorMessage := "rego_parse_error: unexpected identifier token: expected else value term or rule body"
		if !strings.Contains(errorMessage, expectedErrorMessage) {
			t.Fatalf("expected error message to contain '%s', but got '%s'", expectedErrorMessage, errorMessage)
		}
	})
}

func TestUnexpectedElseIfErr(t *testing.T) {
	files := map[string]string{
		"bug.rego": `package bug
			import future.keywords.if
			q := 1 if false
			else := 2 if
			`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.optimizationLevel = 1
		params.dataPaths = newrepeatedStringFlag([]string{path})
		params.entrypoints = newrepeatedStringFlag([]string{"bug/p"})

		var buf bytes.Buffer

		_, err := eval([]string{"data.bug.p"}, params, &buf)

		// Check if there was an error
		if err == nil {
			t.Fatalf("expected an error, but got nil")
		}

		// Check the error message
		errorMessage := err.Error()
		expectedErrorMessage := "rego_parse_error: unexpected eof token: rule body expected"
		if !strings.Contains(errorMessage, expectedErrorMessage) {
			t.Fatalf("expected error message to contain '%s', but got '%s'", expectedErrorMessage, errorMessage)
		}
	})
}

func TestEvalPolicyWithV1CompatibleFlag(t *testing.T) {
	tests := []struct {
		note         string
		v1Compatible bool
		modules      map[string]string
		query        string
		expectedErr  string
	}{
		{
			note: "default compatibility: policy with no rego.v1 or future.keywords imports",
			modules: map[string]string{
				"test.rego": `package test
				allow if {
					1 < 2
				}`,
			},
			query:       "data.test.allow",
			expectedErr: "rego_parse_error",
		},
		{
			note:         "1.0 compatibility: policy with no rego.v1 or future.keywords imports",
			v1Compatible: true,
			modules: map[string]string{
				"test.rego": `package test
				allow if {
					1 < 2
				}`,
			},
			query: "data.test.allow",
		},
		{
			note:         "1.0 compatibility: policy with rego.v1 import",
			v1Compatible: true,
			modules: map[string]string{
				"test.rego": `package test
				import rego.v1
				allow if {
					1 < 2
				}`,
			},
			query: "data.test.allow",
		},
		{
			note:         "1.0 compatibility: policy with future.keywords import",
			v1Compatible: true,
			modules: map[string]string{
				"test.rego": `package test
				import future.keywords.if
				allow if {
					1 < 2
				}`,
			},
			query: "data.test.allow",
		},
	}

	setup := []struct {
		name          string
		commandParams func(params *evalCommandParams, path string)
	}{
		{
			name: "Files",
			commandParams: func(params *evalCommandParams, path string) {
				params.dataPaths = newrepeatedStringFlag([]string{path})
			},
		},
		{
			name: "Bundle",
			commandParams: func(params *evalCommandParams, path string) {
				if err := params.bundlePaths.Set(path); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, s := range setup {
		for _, tc := range tests {
			t.Run(fmt.Sprintf("%s: %s", s.name, tc.note), func(t *testing.T) {
				test.WithTempFS(tc.modules, func(path string) {
					params := newEvalCommandParams()
					s.commandParams(&params, path)
					params.v1Compatible = tc.v1Compatible

					var buf bytes.Buffer

					defined, err := eval([]string{tc.query}, params, &buf)

					if tc.expectedErr == "" {
						if err != nil {
							t.Fatalf("Unexpected error: %v, buf: %s", err, buf.String())
						} else if !defined {
							t.Fatal("expected result to be defined")
						}
					} else {
						if err == nil {
							t.Fatal("expected error, got none")
						}

						actual := buf.String()
						if !strings.Contains(actual, tc.expectedErr) {
							t.Fatalf("expected error:\n\n%v\n\ngot\n\n%v", tc.expectedErr, actual)
						}
					}
				})
			})
		}
	}
}

func TestEvalPolicyWithBundleRegoVersion(t *testing.T) {
	tests := []struct {
		note        string
		files       map[string]string
		query       string
		expectedErr string
	}{
		{
			note: "v0.x bundle, no rego.v1 or future.keywords imports",
			files: map[string]string{
				".manifest": `{"rego_version": 0}`,
				"policy.rego": `package test
allow if {
	1 < 2
}`,
			},
			query:       "data.test.allow",
			expectedErr: "rego_parse_error",
		},
		{
			note: "v0 bundle, v1 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[1] {
	1 < 2
}
`,
				"policy2.rego": `package test
p contains 2 if {
	1 < 2
}
`,
			},
			query: "data.test.p",
		},
		{
			note: "v0 bundle, v1 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/bar/*.rego": 1
	}
}`,
				"foo/policy1.rego": `package test
p[1] {
	1 < 2
}
`,
				"bar/policy1.rego": `package test
p contains 2 if {
	1 < 2
}
`,
				"bar/policy2.rego": `package test
p contains 3 if {
	1 < 2
}
`,
			},
			query: "data.test.p",
		},
		{
			note: "v0 bundle, v1 per-file override, incompliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 0,
	"file_rego_versions": {
		"/policy2.rego": 1
	}
}`,
				"policy1.rego": `package test
p[1] {
	1 < 2
}
`,
				"policy2.rego": `package test
p[2] {
	1 < 2
}
`,
			},
			query:       "data.test.p",
			expectedErr: "rego_parse_error",
		},

		{
			note: "v1.0 bundle, no rego.v1 or future.keywords imports",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
allow if {
	1 < 2
}`,
			},
			query: "data.test.allow",
		},
		{
			note: "v1.0 bundle, policy with rego.v1 import",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import rego.v1
allow if {
	1 < 2
}`,
			},
			query: "data.test.allow",
		},
		{
			note: "v1.0 bundle, future.keywords import",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
import future.keywords.if
allow if {
	1 < 2
}`,
			},
			query: "data.test.allow",
		},
		{
			note: "v1.0 bundle, keywords not used",
			files: map[string]string{
				".manifest": `{"rego_version": 1}`,
				"policy.rego": `package test
allow {
	1 < 2
}`,
			},
			query:       "data.test.allow",
			expectedErr: "rego_parse_error",
		},
		{
			note: "v1 bundle, v0 per-file override",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/policy1.rego": 0
	}
}`,
				"policy1.rego": `package test
p[1] {
	1 < 2
}
`,
				"policy2.rego": `package test
p contains 2 if {
	1 < 2
}
`,
			},
			query: "data.test.p",
		},
		{
			note: "v1 bundle, v0 per-file override (glob)",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"/foo/*.rego": 0
	}
}`,
				"foo/policy1.rego": `package test
p[1] {
	1 < 2
}
`,
				"foo/policy2.rego": `package test
p[2] {
	1 < 2
}
`,
				"bar/policy1.rego": `package test
p contains 3 if {
	1 < 2
}
`,
			},
			query: "data.test.p",
		},
		{
			note: "v1 bundle, v0 per-file override, incompliant",
			files: map[string]string{
				".manifest": `{
	"rego_version": 1,
	"file_rego_versions": {
		"*/policy2.rego": 0
	}
}`,
				"policy1.rego": `package test
p contains 1 if {
	input.x == 1
}
`,
				"policy2.rego": `package test
p contains 2 if {
	input.x == 1
}
`,
			},
			query:       "data.test.p",
			expectedErr: "rego_parse_error",
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
			for _, tc := range tests {
				t.Run(fmt.Sprintf("%s, %s, %s", bundleType.note, v1CompatibleFlag.note, tc.note), func(t *testing.T) {
					files := map[string]string{}

					if bundleType.tar {
						files["bundle.tar.gz"] = ""
					} else {
						for k, v := range tc.files {
							files[k] = v
						}
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

						params := newEvalCommandParams()
						params.v1Compatible = v1CompatibleFlag.used
						if err := params.bundlePaths.Set(p); err != nil {
							t.Fatal(err)
						}

						var buf bytes.Buffer

						defined, err := eval([]string{tc.query}, params, &buf)

						if tc.expectedErr == "" {
							if err != nil {
								t.Fatalf("Unexpected error: %v, buf: %s", err, buf.String())
							} else if !defined {
								t.Fatal("expected result to be defined")
							}
						} else {
							if err == nil {
								t.Fatal("expected error, got none")
							}

							actual := buf.String()
							if !strings.Contains(actual, tc.expectedErr) {
								t.Fatalf("expected error:\n\n%v\n\ngot\n\n%v", tc.expectedErr, actual)
							}
						}
					})
				})
			}
		}
	}
}

func TestWithQueryImports(t *testing.T) {
	tests := []struct {
		note    string
		query   string
		imports []string
		exp     string
		expErrs []string
	}{
		{
			note:  "no imports, none required",
			query: "1 + 2",
			exp:   "3\n",
		},
		{
			note:    "future keyword used, future.keywords imported",
			query:   `"b" in ["a", "b", "c"]`,
			imports: []string{"future.keywords.in"},
			exp:     "true\n",
		},
		{
			note:    "future keyword used, rego.v1 imported",
			query:   `"b" in ["a", "b", "c"]`,
			imports: []string{"rego.v1"},
			exp:     "true\n",
		},
		{
			note:    "future keyword used, invalid rego.v2 imported",
			query:   `"b" in ["a", "b", "c"]`,
			imports: []string{"rego.v2"},
			expErrs: []string{
				"1:8: rego_parse_error: invalid import `rego.v2`, must be `rego.v1`",
			},
		},
		{
			note:  "future keyword used, no imports",
			query: `"b" in ["a", "b", "c"]`,
			expErrs: []string{
				"1:5: rego_unsafe_var_error: var in is unsafe (hint: `import future.keywords.in` to import a future keyword)",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			params := newEvalCommandParams()
			_ = params.outputFormat.Set(evalPrettyOutput)
			params.imports = newrepeatedStringFlag(tc.imports)

			var buf bytes.Buffer

			defined, err := eval([]string{tc.query}, params, &buf)

			if len(tc.expErrs) == 0 {
				if err != nil {
					t.Fatalf("Unexpected error: %v, buf: %s", err, buf.String())
				}

				if !defined {
					t.Fatal("expected result to be defined")
				}

				if buf.String() != tc.exp {
					t.Fatalf("expected:\n\n%s\n\ngot:\n\n%s", tc.exp, buf.String())
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got none")
				}

				actual := buf.String()
				for _, expErr := range tc.expErrs {
					if !strings.Contains(actual, expErr) {
						t.Fatalf("expected error:\n\n%v\n\ngot\n\n%v", expErr, actual)
					}
				}
			}
		})
	}
}
