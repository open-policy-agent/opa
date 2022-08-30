package ast_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/parser"
)

func TestQueryDocMethods(t *testing.T) {
	doc, err := parser.ParseQuery(&Source{Input: `
		query Bob { foo { ...Frag } }
		fragment Frag on Foo {
			bar
		}
	`})

	require.Nil(t, err)
	t.Run("GetOperation", func(t *testing.T) {
		require.EqualValues(t, "Bob", doc.Operations.ForName("Bob").Name)
		require.Nil(t, doc.Operations.ForName("Alice"))
	})

	t.Run("GetFragment", func(t *testing.T) {
		require.EqualValues(t, "Frag", doc.Fragments.ForName("Frag").Name)
		require.Nil(t, doc.Fragments.ForName("Alice"))
	})
}

func TestNamedTypeCompatability(t *testing.T) {
	assert.True(t, NamedType("A", nil).IsCompatible(NamedType("A", nil)))
	assert.False(t, NamedType("A", nil).IsCompatible(NamedType("B", nil)))

	assert.True(t, ListType(NamedType("A", nil), nil).IsCompatible(ListType(NamedType("A", nil), nil)))
	assert.False(t, ListType(NamedType("A", nil), nil).IsCompatible(ListType(NamedType("B", nil), nil)))
	assert.False(t, ListType(NamedType("A", nil), nil).IsCompatible(ListType(NamedType("B", nil), nil)))

	assert.True(t, ListType(NamedType("A", nil), nil).IsCompatible(ListType(NamedType("A", nil), nil)))
	assert.False(t, ListType(NamedType("A", nil), nil).IsCompatible(ListType(NamedType("B", nil), nil)))
	assert.False(t, ListType(NamedType("A", nil), nil).IsCompatible(ListType(NamedType("B", nil), nil)))

	assert.True(t, NonNullNamedType("A", nil).IsCompatible(NamedType("A", nil)))
	assert.False(t, NamedType("A", nil).IsCompatible(NonNullNamedType("A", nil)))

	assert.True(t, NonNullListType(NamedType("String", nil), nil).IsCompatible(NonNullListType(NamedType("String", nil), nil)))
	assert.True(t, NonNullListType(NamedType("String", nil), nil).IsCompatible(ListType(NamedType("String", nil), nil)))
	assert.False(t, ListType(NamedType("String", nil), nil).IsCompatible(NonNullListType(NamedType("String", nil), nil)))

	assert.True(t, ListType(NonNullNamedType("String", nil), nil).IsCompatible(ListType(NamedType("String", nil), nil)))
	assert.False(t, ListType(NamedType("String", nil), nil).IsCompatible(ListType(NonNullNamedType("String", nil), nil)))
}
