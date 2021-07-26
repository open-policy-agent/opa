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
	"path/filepath"
	"reflect"
	"testing"

	"github.com/open-policy-agent/opa/ast"
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

func TestEvalWithProfiler(t *testing.T) {
	files := map[string]string{
		"x.rego": `package x

p = 1`,
	}

	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.profile = true
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
		if len(rs) != 1 {
			t.Fatalf("Expected exactly 1 result, actual: %s", rs)
		}

		r := rs[0].Expressions
		if len(r) != 1 {
			t.Fatalf("Expected exactly 1 expression in the result, actual: %s", r)
		}

		if string(util.MustMarshalJSON(r[0].Value)) != "true" {
			t.Fatalf("Expected result value to be true")
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

func testEvalWithSchemaFile(t *testing.T, input string, query string, schema string) error {
	files := map[string]string{
		"input.json":  input,
		"schema.json": schema,
	}

	var err error
	test.WithTempFS(files, func(path string) {

		params := newEvalCommandParams()
		params.inputPath = filepath.Join(path, "input.json")
		params.schemaPath = filepath.Join(path, "schema.json")

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
		if len(rs) != 1 {
			t.Fatalf("Expected exactly 1 result, actual: %s", rs)
		}

		r := rs[0].Expressions
		if len(r) != 1 {
			t.Fatalf("Expected exactly 1 expression in the result, actual: %s", r)
		}

		if string(util.MustMarshalJSON(r[0].Value)) != "true" {
			t.Fatalf("Expected result value to be true")
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
		params.schemaPath = filepath.Join(path, "schemaBad.json")

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
		params.schemaPath = filepath.Join(path, "schemas")

		// Don't assign over "err" or "err =" does nothing.
		schemaSet, errSchema := loader.Schemas(params.schemaPath)
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
	err := testEvalWithSchemaFile(t, input, query, schema)
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
	err := testEvalWithSchemaFile(t, input, query, schema)
	if err == nil {
		t.Fatalf("expected error but err == nil")
	}

	err = testEvalWithInvalidSchemaFile(input, query, schema)
	if err == nil {
		t.Fatalf("expected error but err == nil")
	}
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
		params.bundlePaths = repeatedStringFlag{
			v:     []string{path},
			isSet: true,
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
		params.bundlePaths = repeatedStringFlag{
			v: []string{
				filepath.Join(path, "a"),
				filepath.Join(path, "b"),
			},
			isSet: true,
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

	p {
		a := input.z
		a == 1
	}

	p {
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
