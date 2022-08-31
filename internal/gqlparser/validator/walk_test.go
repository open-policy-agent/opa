package validator

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/parser"
	"github.com/stretchr/testify/require"
)

func TestWalker(t *testing.T) {
	schema, err := LoadSchema(Prelude, &ast.Source{Input: "type Query { name: String }\n schema { query: Query }"})
	require.Nil(t, err)
	query, err := parser.ParseQuery(&ast.Source{Input: "{ as: name }"})
	require.Nil(t, err)

	called := false
	observers := &Events{}
	observers.OnField(func(walker *Walker, field *ast.Field) {
		called = true

		require.Equal(t, "name", field.Name)
		require.Equal(t, "as", field.Alias)
		require.Equal(t, "name", field.Definition.Name)
		require.Equal(t, "Query", field.ObjectDefinition.Name)
	})

	Walk(schema, query, observers)

	require.True(t, called)
}

func TestWalkInlineFragment(t *testing.T) {
	schema, err := LoadSchema(Prelude, &ast.Source{Input: "type Query { name: String }\n schema { query: Query }"})
	require.Nil(t, err)
	query, err := parser.ParseQuery(&ast.Source{Input: "{ ... { name } }"})
	require.Nil(t, err)

	called := false
	observers := &Events{}
	observers.OnField(func(walker *Walker, field *ast.Field) {
		called = true

		require.Equal(t, "name", field.Name)
		require.Equal(t, "name", field.Definition.Name)
		require.Equal(t, "Query", field.ObjectDefinition.Name)
	})

	Walk(schema, query, observers)

	require.True(t, called)
}
