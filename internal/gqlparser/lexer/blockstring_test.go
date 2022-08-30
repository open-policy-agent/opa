package lexer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBlockStringValue(t *testing.T) {
	t.Run("removes uniform indentation from a string", func(t *testing.T) {
		result := blockStringValue(`
    Hello,
      World!

    Yours,
      GraphQL.`)

		require.Equal(t, "Hello,\n  World!\n\nYours,\n  GraphQL.", result)
	})

	t.Run("removes empty leading and trailing lines", func(t *testing.T) {
		result := blockStringValue(`


    Hello,
      World!

    Yours,
      GraphQL.

`)

		require.Equal(t, "Hello,\n  World!\n\nYours,\n  GraphQL.", result)
	})

	t.Run("removes blank and trailing newlines", func(t *testing.T) {
		result := blockStringValue(`

       
    Hello,
      World!

    Yours,
      GraphQL.
         

`)

		require.Equal(t, "Hello,\n  World!\n\nYours,\n  GraphQL.", result)
	})

	t.Run("does not alter trailing spaces", func(t *testing.T) {
		result := blockStringValue(`

                
    Hello,      
      World!    
                
    Yours,      
      GraphQL.  
                

`)

		require.Equal(t, "Hello,      \n  World!    \n            \nYours,      \n  GraphQL.  ", result)
	})
}
