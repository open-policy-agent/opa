package gqlparser

import (
	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/gqlerror"
	"github.com/open-policy-agent/opa/internal/gqlparser/parser"
	"github.com/open-policy-agent/opa/internal/gqlparser/validator"

	// Blank import is used to load up the validator rules.
	_ "github.com/open-policy-agent/opa/internal/gqlparser/validator/rules"
)

func LoadSchema(str ...*ast.Source) (*ast.Schema, error) {
	return validator.LoadSchema(append([]*ast.Source{validator.Prelude}, str...)...)
}

func MustLoadSchema(str ...*ast.Source) *ast.Schema {
	s, err := validator.LoadSchema(append([]*ast.Source{validator.Prelude}, str...)...)
	if err != nil {
		panic(err)
	}
	return s
}

func LoadQuery(schema *ast.Schema, str string) (*ast.QueryDocument, gqlerror.List) {
	query, err := parser.ParseQuery(&ast.Source{Input: str})
	if err != nil {
		gqlErr := err.(*gqlerror.Error)
		return nil, gqlerror.List{gqlErr}
	}
	errs := validator.Validate(schema, query)
	if errs != nil {
		return nil, errs
	}

	return query, nil
}

func MustLoadQuery(schema *ast.Schema, str string) *ast.QueryDocument {
	q, err := LoadQuery(schema, str)
	if err != nil {
		panic(err)
	}
	return q
}
