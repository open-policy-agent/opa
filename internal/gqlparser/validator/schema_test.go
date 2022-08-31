package validator

import (
	"os"
	"testing"

	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/parser/testrunner"
	"github.com/stretchr/testify/require"
)

func TestLoadSchema(t *testing.T) {
	t.Run("prelude", func(t *testing.T) {
		s, err := LoadSchema(Prelude)
		require.Nil(t, err)

		boolDef := s.Types["Boolean"]
		require.Equal(t, "Boolean", boolDef.Name)
		require.Equal(t, ast.Scalar, boolDef.Kind)
		require.Equal(t, "The `Boolean` scalar type represents `true` or `false`.", boolDef.Description)
	})
	t.Run("swapi", func(t *testing.T) {
		file, err := os.ReadFile("testdata/swapi.graphql")
		require.Nil(t, err)
		s, err := LoadSchema(Prelude, &ast.Source{Input: string(file), Name: "TestLoadSchema"})
		require.Nil(t, err)

		require.Equal(t, "Query", s.Query.Name)
		require.Equal(t, "hero", s.Query.Fields[0].Name)

		require.Equal(t, "Human", s.Types["Human"].Name)

		require.Equal(t, "Subscription", s.Subscription.Name)
		require.Equal(t, "reviewAdded", s.Subscription.Fields[0].Name)

		possibleCharacters := s.GetPossibleTypes(s.Types["Character"])
		require.Len(t, possibleCharacters, 2)
		require.Equal(t, "Human", possibleCharacters[0].Name)
		require.Equal(t, "Droid", possibleCharacters[1].Name)

		implements := s.GetImplements(s.Types["Droid"])
		require.Len(t, implements, 2)
		require.Equal(t, "Character", implements[0].Name)    // interface
		require.Equal(t, "SearchResult", implements[1].Name) // union
	})

	t.Run("default root operation type names", func(t *testing.T) {
		file, err := os.ReadFile("testdata/default_root_operation_type_names.graphql")
		require.Nil(t, err)
		s, err := LoadSchema(Prelude, &ast.Source{Input: string(file), Name: "TestLoadSchema"})
		require.Nil(t, err)

		require.Nil(t, s.Mutation)
		require.Nil(t, s.Subscription)

		require.Equal(t, "Mutation", s.Types["Mutation"].Name)
		require.Equal(t, "Subscription", s.Types["Subscription"].Name)
	})

	t.Run("type extensions", func(t *testing.T) {
		file, err := os.ReadFile("testdata/extensions.graphql")
		require.Nil(t, err)
		s, err := LoadSchema(Prelude, &ast.Source{Input: string(file), Name: "TestLoadSchema"})
		require.Nil(t, err)

		require.Equal(t, "Subscription", s.Subscription.Name)
		require.Equal(t, "dogEvents", s.Subscription.Fields[0].Name)

		require.Equal(t, "owner", s.Types["Dog"].Fields[1].Name)

		directives := s.Types["Person"].Directives
		require.Len(t, directives, 2)
		wantArgs := []string{"sushi", "tempura"}
		for i, directive := range directives {
			require.Equal(t, "favorite", directive.Name)
			require.True(t, directive.Definition.IsRepeatable)
			for _, arg := range directive.Arguments {
				require.Equal(t, wantArgs[i], arg.Value.Raw)
			}
		}
	})

	t.Run("interfaces", func(t *testing.T) {
		file, err := os.ReadFile("testdata/interfaces.graphql")
		require.Nil(t, err)
		s, err := LoadSchema(Prelude, &ast.Source{Input: string(file), Name: "interfaces"})
		require.Nil(t, err)

		implements := s.GetImplements(s.Types["Canine"])
		require.Len(t, implements, 1)
		require.Equal(t, "Mammal", implements[0].Name)

		possibleTypes := s.GetPossibleTypes(s.Types["Mammal"])
		require.Len(t, possibleTypes, 1)
		require.Equal(t, "Canine", possibleTypes[0].Name)
	})

	testrunner.Test(t, "./schema_test.yml", func(t *testing.T, input string) testrunner.Spec {
		_, err := LoadSchema(Prelude, &ast.Source{Input: input})
		return testrunner.Spec{
			Error: err,
		}
	})
}

func TestSchemaDescription(t *testing.T) {
	s, err := LoadSchema(Prelude, &ast.Source{Name: "graph/schema.graphqls", Input: `
	"""
	A simple GraphQL schema which is well described.
	"""
	schema {
		query: Query
	}

	type Query {
		entity: String
	}
	`, BuiltIn: false})
	require.Nil(t, err)
	want := "A simple GraphQL schema which is well described."
	require.Equal(t, want, s.Description)
}
