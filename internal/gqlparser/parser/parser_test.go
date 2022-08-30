package parser

import (
	"testing"

	"github.com/open-policy-agent/opa/internal/gqlparser/ast"
	"github.com/open-policy-agent/opa/internal/gqlparser/lexer"
	"github.com/stretchr/testify/require"
)

func TestParserUtils(t *testing.T) {
	t.Run("test lookaround", func(t *testing.T) {
		p := newParser("asdf 1.0 turtles")
		require.Equal(t, "asdf", p.peek().Value)
		require.Equal(t, "asdf", p.expectKeyword("asdf").Value)
		require.Equal(t, "asdf", p.prev.Value)
		require.Nil(t, p.err)

		require.Equal(t, "1.0", p.peek().Value)
		require.Equal(t, "1.0", p.peek().Value)
		require.Equal(t, "1.0", p.expect(lexer.Float).Value)
		require.Equal(t, "1.0", p.prev.Value)
		require.Nil(t, p.err)

		require.True(t, p.skip(lexer.Name))
		require.Nil(t, p.err)

		require.Equal(t, lexer.EOF, p.peek().Kind)
		require.Nil(t, p.err)
	})

	t.Run("test many", func(t *testing.T) {
		t.Run("can read array", func(t *testing.T) {
			p := newParser("[a b c d]")

			var arr []string
			p.many(lexer.BracketL, lexer.BracketR, func() {
				arr = append(arr, p.next().Value)
			})
			require.Nil(t, p.err)
			require.Equal(t, []string{"a", "b", "c", "d"}, arr)

			require.Equal(t, lexer.EOF, p.peek().Kind)
			require.Nil(t, p.err)
		})

		t.Run("return if open is not found", func(t *testing.T) {
			p := newParser("turtles are happy")

			p.many(lexer.BracketL, lexer.BracketR, func() {
				t.Error("cb should not be called")
			})
			require.Nil(t, p.err)
			require.Equal(t, "turtles", p.next().Value)
		})

		t.Run("will stop on error", func(t *testing.T) {
			p := newParser("[a b c d]")

			var arr []string
			p.many(lexer.BracketL, lexer.BracketR, func() {
				arr = append(arr, p.next().Value)
				if len(arr) == 2 {
					p.error(p.peek(), "boom")
				}
			})
			require.EqualError(t, p.err, "input.graphql:1: boom")
			require.Equal(t, []string{"a", "b"}, arr)
		})
	})

	t.Run("test some", func(t *testing.T) {
		t.Run("can read array", func(t *testing.T) {
			p := newParser("[a b c d]")

			var arr []string
			p.some(lexer.BracketL, lexer.BracketR, func() {
				arr = append(arr, p.next().Value)
			})
			require.Nil(t, p.err)
			require.Equal(t, []string{"a", "b", "c", "d"}, arr)

			require.Equal(t, lexer.EOF, p.peek().Kind)
			require.Nil(t, p.err)
		})

		t.Run("can't read empty array", func(t *testing.T) {
			p := newParser("[]")

			var arr []string
			p.some(lexer.BracketL, lexer.BracketR, func() {
				arr = append(arr, p.next().Value)
			})
			require.EqualError(t, p.err, "input.graphql:1: expected at least one definition, found ]")
			require.Equal(t, []string(nil), arr)
			require.NotEqual(t, lexer.EOF, p.peek().Kind)
		})

		t.Run("return if open is not found", func(t *testing.T) {
			p := newParser("turtles are happy")

			p.some(lexer.BracketL, lexer.BracketR, func() {
				t.Error("cb should not be called")
			})
			require.Nil(t, p.err)
			require.Equal(t, "turtles", p.next().Value)
		})

		t.Run("will stop on error", func(t *testing.T) {
			p := newParser("[a b c d]")

			var arr []string
			p.some(lexer.BracketL, lexer.BracketR, func() {
				arr = append(arr, p.next().Value)
				if len(arr) == 2 {
					p.error(p.peek(), "boom")
				}
			})
			require.EqualError(t, p.err, "input.graphql:1: boom")
			require.Equal(t, []string{"a", "b"}, arr)
		})
	})

	t.Run("test errors", func(t *testing.T) {
		p := newParser("foo bar")

		p.next()
		p.error(p.peek(), "test error")
		p.error(p.peek(), "secondary error")

		require.EqualError(t, p.err, "input.graphql:1: test error")

		require.Equal(t, "foo", p.peek().Value)
		require.Equal(t, "foo", p.next().Value)
		require.Equal(t, "foo", p.peek().Value)
	})

	t.Run("unexpected error", func(t *testing.T) {
		p := newParser("1 3")
		p.unexpectedError()
		require.EqualError(t, p.err, "input.graphql:1: Unexpected Int \"1\"")
	})

	t.Run("unexpected error", func(t *testing.T) {
		p := newParser("1 3")
		p.unexpectedToken(p.next())
		require.EqualError(t, p.err, "input.graphql:1: Unexpected Int \"1\"")
	})

	t.Run("expect error", func(t *testing.T) {
		p := newParser("foo bar")
		p.expect(lexer.Float)

		require.EqualError(t, p.err, "input.graphql:1: Expected Float, found Name")
	})

	t.Run("expectKeyword error", func(t *testing.T) {
		p := newParser("foo bar")
		p.expectKeyword("baz")

		require.EqualError(t, p.err, "input.graphql:1: Expected \"baz\", found Name \"foo\"")
	})
}

func newParser(input string) parser {
	return parser{lexer: lexer.New(&ast.Source{Input: input, Name: "input.graphql"})}
}
