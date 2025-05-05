package parser

import (
	"fmt"
	"strconv"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/vektah/gqlparser/v2/lexer"
)

type parser struct {
	lexer lexer.Lexer
	err   error

	peeked    bool
	peekToken lexer.Token
	peekError error

	prev lexer.Token

	comment          *ast.CommentGroup
	commentConsuming bool

	tokenCount    int
	maxTokenLimit int
}

func (p *parser) SetMaxTokenLimit(maxToken int) {
	p.maxTokenLimit = maxToken
}

func (p *parser) consumeComment() (*ast.Comment, bool) {
	if p.err != nil {
		return nil, false
	}
	tok := p.peek()
	if tok.Kind != lexer.Comment {
		return nil, false
	}
	p.next()
	return &ast.Comment{
		Value:    tok.Value,
		Position: &tok.Pos,
	}, true
}

func (p *parser) consumeCommentGroup() {
	if p.err != nil {
		return
	}
	if p.commentConsuming {
		return
	}
	p.commentConsuming = true

	var comments []*ast.Comment
	for {
		comment, ok := p.consumeComment()
		if !ok {
			break
		}
		comments = append(comments, comment)
	}

	p.comment = &ast.CommentGroup{List: comments}
	p.commentConsuming = false
}

func (p *parser) peekPos() *ast.Position {
	if p.err != nil {
		return nil
	}

	peek := p.peek()
	return &peek.Pos
}

func (p *parser) peek() lexer.Token {
	if p.err != nil {
		return p.prev
	}

	if !p.peeked {
		p.peekToken, p.peekError = p.lexer.ReadToken()
		p.peeked = true
		if p.peekToken.Kind == lexer.Comment {
			p.consumeCommentGroup()
		}
	}

	return p.peekToken
}

func (p *parser) error(tok lexer.Token, format string, args ...interface{}) {
	if p.err != nil {
		return
	}
	p.err = gqlerror.ErrorLocf(tok.Pos.Src.Name, tok.Pos.Line, tok.Pos.Column, format, args...)
}

func (p *parser) next() lexer.Token {
	if p.err != nil {
		return p.prev
	}
	// Increment the token count before reading the next token
	p.tokenCount++
	if p.maxTokenLimit != 0 && p.tokenCount > p.maxTokenLimit {
		p.err = fmt.Errorf("exceeded token limit of %d", p.maxTokenLimit)
		return p.prev
	}
	if p.peeked {
		p.peeked = false
		p.comment = nil
		p.prev, p.err = p.peekToken, p.peekError
	} else {
		p.prev, p.err = p.lexer.ReadToken()
		if p.prev.Kind == lexer.Comment {
			p.consumeCommentGroup()
		}
	}
	return p.prev
}

func (p *parser) expectKeyword(value string) (lexer.Token, *ast.CommentGroup) {
	tok := p.peek()
	comment := p.comment
	if tok.Kind == lexer.Name && tok.Value == value {
		return p.next(), comment
	}

	p.error(tok, "Expected %s, found %s", strconv.Quote(value), tok.String())
	return tok, comment
}

func (p *parser) expect(kind lexer.Type) (lexer.Token, *ast.CommentGroup) {
	tok := p.peek()
	comment := p.comment
	if tok.Kind == kind {
		return p.next(), comment
	}

	p.error(tok, "Expected %s, found %s", kind, tok.Kind.String())
	return tok, comment
}

func (p *parser) skip(kind lexer.Type) bool {
	if p.err != nil {
		return false
	}

	tok := p.peek()

	if tok.Kind != kind {
		return false
	}
	p.next()
	return true
}

func (p *parser) unexpectedError() {
	p.unexpectedToken(p.peek())
}

func (p *parser) unexpectedToken(tok lexer.Token) {
	p.error(tok, "Unexpected %s", tok.String())
}

func (p *parser) many(start lexer.Type, end lexer.Type, cb func()) {
	hasDef := p.skip(start)
	if !hasDef {
		return
	}

	for p.peek().Kind != end && p.err == nil {
		cb()
	}
	p.next()
}

func (p *parser) some(start lexer.Type, end lexer.Type, cb func()) *ast.CommentGroup {
	hasDef := p.skip(start)
	if !hasDef {
		return nil
	}

	called := false
	for p.peek().Kind != end && p.err == nil {
		called = true
		cb()
	}

	if !called {
		p.error(p.peek(), "expected at least one definition, found %s", p.peek().Kind.String())
		return nil
	}

	comment := p.comment
	p.next()
	return comment
}
