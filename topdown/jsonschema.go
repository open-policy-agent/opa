package topdown

import (
	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/internal/gojsonschema"
	"github.com/open-policy-agent/opa/topdown/builtins"
)

// implements topdown.BuiltinFunc
func builtinJSONSchemaIsValid(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := validateSchema(operands)
	if err != nil {
		return err
	}

	return iter(ast.BooleanTerm(result.Valid()))
}

// implements topdown.BuiltinFunc
func builtinJSONSchemaValidate(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := validateSchema(operands)
	if err != nil {
		return err
	}

	var validationErrorTerms []*ast.Term
	for _, err := range result.Errors() {
		term := ast.ObjectTerm(
			[2]*ast.Term{ast.StringTerm("error"), ast.StringTerm(err.String())},
			[2]*ast.Term{ast.StringTerm("type"), ast.StringTerm(err.Type())},
			[2]*ast.Term{ast.StringTerm("field"), ast.StringTerm(err.Field())},
			[2]*ast.Term{ast.StringTerm("description"), ast.StringTerm(err.Description())},
		)
		validationErrorTerms = append(validationErrorTerms, term)
	}

	return iter(ast.SetTerm(validationErrorTerms...))
}

func validateSchema(operands []*ast.Term) (*gojsonschema.Result, error) {
	schema, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return nil, err
	}

	schemaLoader := gojsonschema.NewStringLoader(string(schema))

	document, err := ast.JSON(operands[1].Value)
	if err != nil {
		return nil, err
	}
	documentLoader := gojsonschema.NewGoLoader(document)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func init() {
	RegisterBuiltinFunc(ast.JSONSchemaIsValid.Name, builtinJSONSchemaIsValid)
	RegisterBuiltinFunc(ast.JSONSchemaValidate.Name, builtinJSONSchemaValidate)
}
