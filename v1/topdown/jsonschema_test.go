// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/open-policy-agent/opa/v1/topdown/cache"

	"github.com/open-policy-agent/opa/v1/ast"
)

func TestAstValueToJSONSchemaLoader(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note   string
		schema ast.Value
		valid  bool
	}{
		{
			note:   "string empty json object",
			schema: ast.String(`{}`),
			valid:  true,
		},
		{
			note:   "string broken json",
			schema: ast.String(`{ "properties": { id: {} } }`),
			valid:  false,
		},
		{
			note: "string simple schema",
			schema: ast.String(`
			{
				"properties": {
					"id": {
						"type": "integer"
					}
				},
				"required": ["id"]
			}
			`),
			valid: true,
		},
		{
			note:   "object empty",
			schema: ast.NewObject(),
			valid:  true,
		},
		{
			note: "object simple schema",
			schema: ast.NewObject(
				[...]*ast.Term{
					ast.StringTerm("properties"),
					ast.NewTerm(ast.NewObject(
						[...]*ast.Term{
							ast.StringTerm("id"),
							ast.NewTerm(ast.NewObject(
								[...]*ast.Term{
									ast.StringTerm("type"),
									ast.StringTerm("integer"),
								},
							)),
						},
					)),
				},
			),
			valid: true,
		},
		{
			note: "array simple input",
			schema: ast.NewArray(
				ast.StringTerm("foo"),
				ast.StringTerm("bar"),
			),
			valid: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			_, err := astValueToJSONSchemaLoader(tc.schema)
			if tc.valid && err != nil {
				t.Errorf("Unexpected JSON Schema validation result, expected valid = true, got = false: %s", err)
				return
			}
			if !tc.valid && err == nil {
				t.Errorf("Unexpected JSON Schema validation result, expected valid = false, got = true")
				return
			}
		})
	}
}

func TestBuiltinJSONSchemaVerify(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note   string
		schema ast.Value
		result ast.Value
		err    bool
	}{
		{
			note:   "string empty schema",
			schema: ast.String(`{}`),
			result: ast.NewArray(ast.BooleanTerm(true), ast.NullTerm()),
			err:    false,
		},
		{
			note:   "string broken JSON",
			schema: ast.String(`{ "a": "`),
			result: ast.NewArray(ast.BooleanTerm(false), ast.StringTerm("jsonschema: invalid JSON string")),
			err:    false,
		},
		{
			note: "string simple schema",
			schema: ast.String(`
			{
				"properties": {
					"id": {
						"type": "integer"
					}
				},
				"required": ["id"]
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(true), ast.NullTerm()),
			err:    false,
		},
		{
			note: "string broken schema",
			schema: ast.String(`
			{
				"properties": {
					"id": {
						"type": "UNKNOWN"
					}
				},
				"required": ["id"]
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(false), ast.StringTerm("jsonschema: has a primitive type that is NOT VALID -- given: /UNKNOWN/ Expected valid values are:[array boolean integer number null object string]")),
			err:    false,
		},
		{
			note: "string schema with valid pattern",
			schema: ast.String(`
			{
				"properties": {
					"name": {
						"type": "string",
						"pattern": "^[a-z]+$"
					}
				}
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(true), ast.NullTerm()),
			err:    false,
		},
		{
			note: "string schema with Go-incompatible pattern (negative lookahead)",
			schema: ast.String(`
			{
				"properties": {
					"name": {
						"type": "string",
						"pattern": "^(?!testing:.*)[a-z]+$"
					}
				}
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(false), ast.StringTerm("jsonschema: pattern must be a valid regex")),
			err:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			result := ast.NullTerm().Value
			err := builtinJSONSchemaVerify(
				BuiltinContext{},
				[]*ast.Term{ast.NewTerm(tc.schema)},
				func(term *ast.Term) error {
					result = term.Value
					return nil
				},
			)

			if tc.err && err == nil {
				t.Errorf("Unexpected schema validation, expected error, got nil")
				return
			}
			if !tc.err && err != nil {
				t.Errorf("Unexpected schema validation, expected nil, got error: %s", err)
				return
			}
			if tc.result.Compare(result) != 0 {
				t.Errorf("Unexpected schema validation, expected result %s, got result %s", tc.result.String(), result.String())
				return
			}
		})
	}
}

func TestBuiltinJSONMatchSchema(t *testing.T) {
	t.Parallel()

	cases := []struct {
		note     string
		document ast.Value
		schema   ast.Value
		result   ast.Value
		err      bool
	}{
		{
			note:     "string empty document, empty schema",
			document: ast.String(`{}`),
			schema:   ast.String(`{}`),
			result:   ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm()),
			err:      false,
		},
		{
			note:     "string empty document, broken schema",
			document: ast.String(`{}`),
			schema:   ast.String(`{ "a": "`),
			result:   ast.NullTerm().Value,
			err:      true,
		},
		{
			note:     "string broken document, empty schema",
			document: ast.String(`{ "a": "`),
			schema:   ast.String(`{}`),
			result:   ast.NullTerm().Value,
			err:      true,
		},
		{
			note:     "string correct document, simple schema",
			document: ast.String(`{ "id": 5 }`),
			schema: ast.String(`
			{
				"properties": {
					"id": {
						"type": "integer"
					}
				},
				"required": ["id"]
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm()),
			err:    false,
		},
		{
			note:     "string correct document, invalid schema",
			document: ast.String(`{ "id": 5 }`),
			schema: ast.String(`
			{
				"properties": {
					"id": {
						"type": "UNKNOWN"
					}
				},
				"required": ["id"]
			}
			`),
			result: ast.NullTerm().Value,
			err:    true,
		},
		{
			note:     "string invalid document, correct schema",
			document: ast.String(`{ "id": "test" }`),
			schema: ast.String(`
			{
				"properties": {
					"id": {
						"type": "integer"
					}
				},
				"required": ["id"]
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("id: Invalid type. Expected: integer, given: string")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("invalid_type")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("id")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("Invalid type. Expected: integer, given: string")},
				)))),
			err: false,
		},
		{
			note:     "string document with matching pattern",
			document: ast.String(`{ "name": "alice" }`),
			schema: ast.String(`
			{
				"properties": {
					"name": {
						"type": "string",
						"pattern": "^[a-z]+$"
					}
				}
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm()),
			err:    false,
		},
		{
			note:     "string document violating pattern",
			document: ast.String(`{ "name": "Alice1" }`),
			schema: ast.String(`
			{
				"properties": {
					"name": {
						"type": "string",
						"pattern": "^[a-z]+$"
					}
				}
			}
			`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("name: Does not match pattern '^[a-z]+$'")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("pattern")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("name")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("Does not match pattern '^[a-z]+$'")},
				)))),
			err: false,
		},
		{
			note:     "schema with Go-incompatible pattern",
			document: ast.String(`{ "name": "alice" }`),
			schema: ast.String(`
			{
				"properties": {
					"name": {
						"type": "string",
						"pattern": "^(?!testing:.*)[a-z]+$"
					}
				}
			}
			`),
			result: ast.NullTerm().Value,
			err:    true,
		},
		// Empty enum is unsatisfiable per JSON Schema: every instance must fail.
		// See https://github.com/open-policy-agent/opa/issues/8910
		{
			note:     "empty enum rejects string",
			document: ast.String(`"a"`),
			schema:   ast.String(`{"enum": []}`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("(Root): (Root) must be one of the following: ")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("(Root)")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("(Root) must be one of the following: ")},
				)))),
			err: false,
		},
		{
			note:     "empty enum rejects number",
			document: ast.String(`1`),
			schema:   ast.String(`{"enum": []}`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("(Root): (Root) must be one of the following: ")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("(Root)")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("(Root) must be one of the following: ")},
				)))),
			err: false,
		},
		{
			note:     "empty enum rejects null",
			document: ast.String(`null`),
			schema:   ast.String(`{"enum": []}`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("(Root): (Root) must be one of the following: ")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("(Root)")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("(Root) must be one of the following: ")},
				)))),
			err: false,
		},
		{
			note:     "empty enum rejects object",
			document: ast.String(`{}`),
			schema:   ast.String(`{"enum": []}`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("(Root): (Root) must be one of the following: ")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("(Root)")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("(Root) must be one of the following: ")},
				)))),
			err: false,
		},
		{
			note:     "non-empty enum accepts listed value",
			document: ast.String(`"a"`),
			schema:   ast.String(`{"enum": ["a", "b", "c"]}`),
			result:   ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm()),
			err:      false,
		},
		{
			note:     "non-empty enum rejects unlisted value",
			document: ast.String(`"z"`),
			schema:   ast.String(`{"enum": ["a", "b", "c"]}`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("(Root): (Root) must be one of the following: \"a\", \"b\", \"c\"")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("(Root)")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("(Root) must be one of the following: \"a\", \"b\", \"c\"")},
				)))),
			err: false,
		},
		// Missing enum keyword is unrestricted (contrast empty enum above).
		{
			note:     "missing enum accepts arbitrary value",
			document: ast.String(`"z"`),
			schema:   ast.String(`{}`),
			result:   ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm()),
			err:      false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			t.Parallel()

			result := ast.NullTerm().Value
			err := builtinJSONMatchSchema(
				BuiltinContext{},
				[]*ast.Term{ast.NewTerm(tc.document), ast.NewTerm(tc.schema)},
				func(term *ast.Term) error {
					result = term.Value
					return nil
				},
			)

			if tc.err && err == nil {
				t.Errorf("Unexpected schema validation, expected error, got nil")
				return
			}
			if !tc.err && err != nil {
				t.Errorf("Unexpected schema validation, expected nil, got error: %s", err)
				return
			}
			if tc.result.Compare(result) != 0 {
				t.Errorf("Unexpected schema validation, expected result %s, got result %s", tc.result.String(), result.String())
				return
			}
		})
	}
}

func TestBuiltinJSONMatchSchemaCache(t *testing.T) {
	t.Parallel()

	schema := ast.String(`
{
  "properties": {
    "id": {
      "type": "integer"
    }
  },
  "required": ["id"]
}
`)

	valueCache := cache.NewInterQueryValueCache(t.Context(), nil)
	document := ast.String(`{ "id": 5 }`)

	var result ast.Value
	err := builtinJSONMatchSchema(
		BuiltinContext{
			InterQueryBuiltinValueCache: valueCache,
		},
		[]*ast.Term{ast.NewTerm(document), ast.NewTerm(schema)},
		func(term *ast.Term) error {
			result = term.Value
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Unexpected schema validation error: %s", err)
	}

	arr, ok := result.(*ast.Array)
	if !ok {
		t.Fatalf("Unexpected result type, expected array, got %T", result)
	}

	expected := ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm())

	if arr.Compare(expected) != 0 {
		t.Fatalf("Unexpected result, expected %s, got %s", expected, arr)
	}

	if _, found := valueCache.Get(schema); !found {
		t.Fatalf("Expected document to be cached")
	}
}

func TestBuiltinJSONSchemaAllowNet(t *testing.T) {
	t.Parallel()

	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprint(w, `{"required": ["pwned"]}`)
	}))
	defer srv.Close()

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	remoteSchema := ast.String(fmt.Sprintf(`{"$ref": %q}`, srv.URL+"/schema.json"))

	cases := []struct {
		note         string
		capabilities *ast.Capabilities
		wantDenied   bool
	}{
		{
			note:         "no capabilities permits any host",
			capabilities: nil,
		},
		{
			note:         "unset allow_net permits any host",
			capabilities: &ast.Capabilities{},
		},
		{
			note:         "empty allow_net permits no host",
			capabilities: &ast.Capabilities{AllowNet: []string{}},
			wantDenied:   true,
		},
		{
			note:         "listed host is permitted",
			capabilities: &ast.Capabilities{AllowNet: []string{srvURL.Hostname()}},
		},
		{
			note:         "unlisted host is denied",
			capabilities: &ast.Capabilities{AllowNet: []string{"example.com"}},
			wantDenied:   true,
		},
	}

	for _, tc := range cases {
		t.Run("json.match_schema/"+tc.note, func(t *testing.T) {
			before := requests.Load()
			err := builtinJSONMatchSchema(
				BuiltinContext{Capabilities: tc.capabilities},
				[]*ast.Term{ast.NewTerm(ast.String(`{"id": 5}`)), ast.NewTerm(remoteSchema)},
				func(*ast.Term) error { return nil },
			)

			fetched := requests.Load() > before
			if tc.wantDenied {
				if err == nil {
					t.Fatal("expected remote reference to be denied, got no error")
				}
				if !strings.Contains(err.Error(), "remote reference loading disabled") {
					t.Fatalf("expected remote reference loading to be disabled, got %v", err)
				}
				if fetched {
					t.Fatal("expected no request to reach the server")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected remote reference to be permitted, got %v", err)
			}
			if !fetched {
				t.Fatal("expected a request to reach the server")
			}
		})

		t.Run("json.verify_schema/"+tc.note, func(t *testing.T) {
			before := requests.Load()
			var result ast.Value
			err := builtinJSONSchemaVerify(
				BuiltinContext{Capabilities: tc.capabilities},
				[]*ast.Term{ast.NewTerm(remoteSchema)},
				func(term *ast.Term) error {
					result = term.Value
					return nil
				},
			)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			arr, ok := result.(*ast.Array)
			if !ok {
				t.Fatalf("Unexpected result type, expected array, got %T", result)
			}
			valid := arr.Elem(0).Value.Compare(ast.Boolean(true)) == 0

			fetched := requests.Load() > before
			if tc.wantDenied {
				if valid {
					t.Fatalf("expected schema verification to fail, got %s", arr)
				}
				if !strings.Contains(arr.Elem(1).String(), "remote reference loading disabled") {
					t.Fatalf("expected remote reference loading to be disabled, got %s", arr)
				}
				if fetched {
					t.Fatal("expected no request to reach the server")
				}
				return
			}
			if !valid {
				t.Fatalf("expected schema verification to succeed, got %s", arr)
			}
			if !fetched {
				t.Fatal("expected a request to reach the server")
			}
		})
	}
}
