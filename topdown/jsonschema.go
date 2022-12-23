// Copyright 2022 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"encoding/json"
	"errors"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/gojsonschema"
)

// astValueToJSONSchemaLoader converts a value to JSON Loader.
// Value can be ast.String or ast.Object.
func astValueToJSONSchemaLoader(value ast.Value) (gojsonschema.JSONLoader, error) {
	var loader gojsonschema.JSONLoader
	var err error

	// ast.Value type selector.
	switch x := value.(type) {
	case ast.String:
		// In case of string pass it as is as a raw JSON string.
		// Make pre-check that it's a valid JSON at all because gojsonschema won't do that.
		if !json.Valid([]byte(x)) {
			err = errors.New("invalid JSON string")
			break
		}
		loader = gojsonschema.NewStringLoader(string(x))
	case ast.Object:
		// In case of object serialize it to JSON string and acts like case above.
		var data []byte
		var asJSON interface{}
		asJSON, err = ast.JSON(value)
		if err != nil {
			break
		}
		data, err = json.Marshal(asJSON)
		loader = gojsonschema.NewStringLoader(string(data))
	default:
		// Any other cases will produce an error.
		return nil, errors.New("wrong type, expected string or object")
	}

	// Catch JSON marshaling errors.
	if err != nil {
		return nil, err
	}

	return loader, nil
}

// builtinJSONSchemaIsValid accepts 1 argument which can be string or object and checks if it is valid JSON schema.
func builtinJSONSchemaIsValid(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Take first argument and make JSON Loader from it.
	loader, err := astValueToJSONSchemaLoader(operands[0].Value)
	if err != nil {
		return iter(ast.BooleanTerm(false))
	}

	// Check that schema is correct and parses without errors.
	if _, err = gojsonschema.NewSchema(loader); err != nil {
		return iter(ast.BooleanTerm(false))
	}

	return iter(ast.BooleanTerm(true))
}

// builtinJSONSchemaValidate accepts 1 argument which can be string or object and checks if it is valid JSON schema.
// Returns string in case of error or empty string otherwise.
func builtinJSONSchemaValidate(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Take first argument and make JSON Loader from it.
	loader, err := astValueToJSONSchemaLoader(operands[0].Value)
	if err != nil {
		return iter(ast.StringTerm("jsonschema: " + err.Error()))
	}

	// Check that schema is correct and parses without errors.
	if _, err = gojsonschema.NewSchema(loader); err != nil {
		return iter(ast.StringTerm("jsonschema: " + err.Error()))
	}

	return iter(ast.StringTerm(""))
}

// builtinJSONMatchSchema accepts 2 arguments both can be string or object and verifies if the document matches the JSON schema.
// Returns an array of errors or empty array.
// Returns an empty array if no errors are found.
// In case of internal error returns empty array.
func builtinJSONMatchSchema(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Take first argument and make JSON Loader from it.
	// This is a JSON document made from Rego JSON string or object.
	documentLoader, err := astValueToJSONSchemaLoader(operands[0].Value)
	if err != nil {
		return err
	}

	// Take second argument and make JSON Loader from it.
	// This is a JSON schema made from Rego JSON string or object.
	schemaLoader, err := astValueToJSONSchemaLoader(operands[1].Value)
	if err != nil {
		return err
	}

	// Make new schema instance to provide validations.
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return err
	}

	// Use the schema instance to validate the document.
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return err
	}

	// In case of validation errors produce Rego array of objects to describe the errors.
	arr := ast.NewArray()
	for _, re := range result.Errors() {
		o := ast.NewObject()
		o.Insert(ast.StringTerm("error"), ast.StringTerm(re.String()))
		o.Insert(ast.StringTerm("type"), ast.StringTerm(re.Type()))
		o.Insert(ast.StringTerm("field"), ast.StringTerm(re.Field()))
		o.Insert(ast.StringTerm("desc"), ast.StringTerm(re.Description()))
		arr = arr.Append(ast.NewTerm(o))
	}

	return iter(ast.NewTerm(arr))
}

// builtinJSONIsMatchSchema accepts 2 arguments both can be string or object and verifies if the document matches the JSON schema.
// Returns true if the document matches the schema.
func builtinJSONIsMatchSchema(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// Take first argument and make JSON Loader from it.
	// This is a JSON document made from Rego JSON string or object.
	documentLoader, err := astValueToJSONSchemaLoader(operands[0].Value)
	if err != nil {
		return err
	}

	// Take second argument and make JSON Loader from it.
	// This is a JSON schema made from Rego JSON string or object.
	schemaLoader, err := astValueToJSONSchemaLoader(operands[1].Value)
	if err != nil {
		return err
	}

	// Make new schema instance to provide validations.
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return err
	}

	// Use the schema instance to validate the document.
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return err
	}

	// Return true/false only without errors explanation.
	return iter(ast.BooleanTerm(result.Valid()))
}

func init() {
	RegisterBuiltinFunc(ast.JSONSchemaIsValid.Name, builtinJSONSchemaIsValid)
	RegisterBuiltinFunc(ast.JSONSchemaValidate.Name, builtinJSONSchemaValidate)
	RegisterBuiltinFunc(ast.JSONIsMatchSchema.Name, builtinJSONIsMatchSchema)
	RegisterBuiltinFunc(ast.JSONMatchSchema.Name, builtinJSONMatchSchema)
}
