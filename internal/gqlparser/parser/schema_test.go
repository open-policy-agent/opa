package parser

import (
	"github.com/stretchr/testify/assert"
	"github.com/open-policy-agent/opa/internal/gqlparser/gqlerror"
	"testing"

	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/parser/testrunner"
)

func TestSchemaDocument(t *testing.T) {
	testrunner.Test(t, "schema_test.yml", func(t *testing.T, input string) testrunner.Spec {
		doc, err := ParseSchema(&ast.Source{Input: input, Name: "spec"})
		if err != nil {
			return testrunner.Spec{
				Error: err.(*gqlerror.Error),
				AST:   ast.Dump(doc),
			}
		}
		return testrunner.Spec{
			AST: ast.Dump(doc),
		}
	})
}

func TestTypePosition(t *testing.T) {
	t.Run("type line number with no bang", func(t *testing.T) {
		schema, parseErr := ParseSchema(&ast.Source{
			Input: `type query {
						me: User
					}
			`,
		})
		assert.Nil(t, parseErr)
		assert.Equal(t, 2, schema.Definitions.ForName("query").Fields.ForName("me").Type.Position.Line)
	})
	t.Run("type line number with bang", func(t *testing.T) {
		schema, parseErr := ParseSchema(&ast.Source{
			Input: `type query {
						me: User!
					}
			`,
		})
		assert.Nil(t, parseErr)
		assert.Equal(t, 2, schema.Definitions.ForName("query").Fields.ForName("me").Type.Position.Line)
	})
}
