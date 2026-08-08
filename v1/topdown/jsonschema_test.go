// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
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
			// https://json-schema.org/draft/2020-12/json-schema-validation#section-6.1.2:
			// enum passes only if the instance deep-equals one of the listed values. An
			// empty list has nothing to deep-equal, so it rejects every instance. Treating
			// it as an absent keyword makes a schema meant to allow nothing allow
			// everything, which fails open for any schema generator whose allow-list
			// resolved to zero entries.
			note:     "empty enum rejects a string",
			document: ast.String(`{ "id": "anything" }`),
			schema:   ast.String(`{ "properties": { "id": { "enum": [] } } }`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("id: id must be one of the following: (none)")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("id")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("id must be one of the following: (none)")},
				)))),
			err: false,
		},
		{
			// null is the case most likely to slip through a length-based guard, since a
			// missing value and a null value look alike in a lot of validators.
			note:     "empty enum rejects null",
			document: ast.String(`{ "id": null }`),
			schema:   ast.String(`{ "properties": { "id": { "enum": [] } } }`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm("id: id must be one of the following: (none)")},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("id")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm("id must be one of the following: (none)")},
				)))),
			err: false,
		},
		{
			// The other half of the contract, and the one a presence flag could break: an
			// absent enum must still constrain nothing.
			note:     "absent enum constrains nothing",
			document: ast.String(`{ "id": "anything" }`),
			schema:   ast.String(`{ "properties": { "id": { "type": "string" } } }`),
			result:   ast.NewArray(ast.BooleanTerm(true), ast.ArrayTerm()),
			err:      false,
		},
		{
			// A non-empty enum keeps working, so the presence gate did not widen it.
			note:     "non-empty enum still rejects a value outside it",
			document: ast.String(`{ "id": "z" }`),
			schema:   ast.String(`{ "properties": { "id": { "enum": ["a", "b"] } } }`),
			result: ast.NewArray(ast.BooleanTerm(false),
				ast.ArrayTerm(ast.NewTerm(ast.NewObject(
					[...]*ast.Term{ast.StringTerm("error"), ast.StringTerm(`id: id must be one of the following: "a", "b"`)},
					[...]*ast.Term{ast.StringTerm("type"), ast.StringTerm("enum")},
					[...]*ast.Term{ast.StringTerm("field"), ast.StringTerm("id")},
					[...]*ast.Term{ast.StringTerm("desc"), ast.StringTerm(`id must be one of the following: "a", "b"`)},
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
