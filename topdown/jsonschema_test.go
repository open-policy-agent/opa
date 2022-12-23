// Copyright 2019 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"testing"

	"github.com/open-policy-agent/opa/ast"
)

func TestAstValueToJSONSchemaLoader(t *testing.T) {
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
			schema: func() ast.Object {
				obj := ast.NewObject()
				obj.Insert(
					ast.StringTerm("properties"),
					ast.NewTerm(func() ast.Object {
						obj := ast.NewObject()
						obj.Insert(
							ast.StringTerm("id"),
							ast.NewTerm(func() ast.Object {
								obj := ast.NewObject()
								obj.Insert(
									ast.StringTerm("type"),
									ast.StringTerm("integer"),
								)
								return obj
							}()),
						)
						return obj
					}()),
				)
				return obj
			}(),
			valid: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
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

func TestBuiltinJSONSchemaIsValid(t *testing.T) {
	cases := []struct {
		note   string
		schema ast.Value
		result ast.Value
		err    bool
	}{
		{
			note:   "string empty schema",
			schema: ast.String(`{}`),
			result: ast.Boolean(true),
			err:    false,
		},
		{
			note:   "string broken JSON",
			schema: ast.String(`{ "a": "`),
			result: ast.Boolean(false),
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
			result: ast.Boolean(true),
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
			result: ast.Boolean(false),
			err:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			result := ast.NullTerm().Value
			err := builtinJSONSchemaIsValid(
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

func TestBuiltinJSONSchemaValidate(t *testing.T) {
	cases := []struct {
		note   string
		schema ast.Value
		result ast.Value
		err    bool
	}{
		{
			note:   "string empty schema",
			schema: ast.String(`{}`),
			result: ast.String(""),
			err:    false,
		},
		{
			note:   "string broken JSON",
			schema: ast.String(`{ "a": "`),
			result: ast.String("jsonschema: invalid JSON string"),
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
			result: ast.String(""),
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
			result: ast.String("jsonschema: has a primitive type that is NOT VALID -- given: /UNKNOWN/ Expected valid values are:[array boolean integer number null object string]"),
			err:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			result := ast.NullTerm().Value
			err := builtinJSONSchemaValidate(
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
			result:   ast.NewArray(),
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
			result: ast.NewArray(),
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
			result: ast.NewArray(ast.NewTerm(func() ast.Object {
				obj := ast.NewObject()
				obj.Insert(ast.StringTerm("error"), ast.StringTerm("id: Invalid type. Expected: integer, given: string"))
				obj.Insert(ast.StringTerm("type"), ast.StringTerm("invalid_type"))
				obj.Insert(ast.StringTerm("field"), ast.StringTerm("id"))
				obj.Insert(ast.StringTerm("desc"), ast.StringTerm("Invalid type. Expected: integer, given: string"))
				return obj
			}())),
			err: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
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

func TestBuiltinJSONIsMatchSchema(t *testing.T) {
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
			result:   ast.Boolean(true),
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
			result: ast.Boolean(true),
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
			result: ast.Boolean(false),
			err:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.note, func(t *testing.T) {
			result := ast.NullTerm().Value
			err := builtinJSONIsMatchSchema(
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
