package bootstrap

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/pigeon/ast"
)

type errList []error

func (e *errList) reset() {
	*e = (*e)[:0]
}

func (e *errList) add(p ast.Pos, err error) {
	*e = append(*e, fmt.Errorf("%s: %v", p, err))
}

func (e *errList) err() error {
	if len(*e) == 0 {
		return nil
	}
	return e
}

func (e *errList) Error() string {
	switch len(*e) {
	case 0:
		return ""
	case 1:
		return (*e)[0].Error()
	default:
		var buf bytes.Buffer

		for i, err := range *e {
			if i > 0 {
				buf.WriteRune('\n')
			}
			buf.WriteString(err.Error())
		}
		return buf.String()
	}
}

// Parser holds the state to parse the PEG grammar into
// an abstract syntax tree (AST).
type Parser struct {
	s   Scanner
	tok Token

	errs *errList
	dbg  bool
	pk   Token
}

func (p *Parser) in(s string) string {
	if p.dbg {
		fmt.Println("IN  "+s, p.tok.id, p.tok.lit)
	}
	return s
}

func (p *Parser) out(s string) {
	if p.dbg {
		fmt.Println("OUT "+s, p.tok.id, p.tok.lit)
	}
}

// NewParser creates a new Parser.
func NewParser() *Parser {
	return &Parser{errs: new(errList)}
}

// Parse parses the data from the reader r and generates the AST
// or returns an error if it fails. The filename is used as information
// in the error messages.
func (p *Parser) Parse(filename string, r io.Reader) (*ast.Grammar, error) {
	p.errs.reset()
	p.s.Init(filename, r, p.errs.add)

	g := p.grammar()
	return g, p.errs.err()
}

func (p *Parser) read() {
	if p.pk.pos.Line != 0 {
		p.tok = p.pk
		p.pk = Token{}
		return
	}
	tok, _ := p.s.Scan()
	p.tok = tok
}

func (p *Parser) peek() Token {
	if p.pk.pos.Line == 0 {
		p.pk, _ = p.s.Scan()
	}
	return p.pk
}

func (p *Parser) skip(ids ...tid) {
outer:
	for {
		for _, id := range ids {
			if p.tok.id == id {
				p.read()
				continue outer
			}
		}
		return
	}
}

func (p *Parser) grammar() *ast.Grammar {
	defer p.out(p.in("grammar"))

	// advance to the first token
	p.read()
	g := ast.NewGrammar(p.tok.pos)

	p.skip(eol, semicolon)
	if p.tok.id == code {
		g.Init = ast.NewCodeBlock(p.tok.pos, p.tok.lit)
		p.read()
		p.skip(eol, semicolon)
	}

	for {
		if p.tok.id == eof {
			return g
		}
		r := p.rule()
		if r != nil {
			g.Rules = append(g.Rules, r)
		}
		p.read()
		p.skip(eol, semicolon)
	}
}

func (p *Parser) expect(ids ...tid) bool {
	if len(ids) == 0 {
		return true
	}

	for _, id := range ids {
		if p.tok.id == id {
			return true
		}
	}
	if len(ids) == 1 {
		p.errs.add(p.tok.pos, fmt.Errorf("expected %s, got %s", ids[0], p.tok.id))
	} else {
		p.errs.add(p.tok.pos, fmt.Errorf("expected any of %v, got %s", ids, p.tok.id))
	}
	return false
}

func (p *Parser) rule() *ast.Rule {
	defer p.out(p.in("rule"))

	if !p.expect(ident) {
		return nil
	}
	r := ast.NewRule(p.tok.pos, ast.NewIdentifier(p.tok.pos, p.tok.lit))
	p.read()

	if p.tok.id == str || p.tok.id == rstr || p.tok.id == char {
		if strings.HasSuffix(p.tok.lit, "i") {
			p.errs.add(p.tok.pos, errors.New("invalid suffix 'i'"))
			return nil
		}
		s, err := strconv.Unquote(p.tok.lit)
		if err != nil {
			p.errs.add(p.tok.pos, err)
			return nil
		}
		r.DisplayName = ast.NewStringLit(p.tok.pos, s)
		p.read()
	}

	if !p.expect(ruledef) {
		return nil
	}
	p.read()
	p.skip(eol)

	expr := p.expression()
	if expr == nil {
		p.errs.add(p.tok.pos, errors.New("missing expression"))
		return nil
	}
	r.Expr = expr

	if !p.expect(eol, eof, semicolon) {
		p.errs.add(p.tok.pos, errors.New("rule not terminated"))
		return nil
	}
	return r
}

func (p *Parser) expression() ast.Expression {
	defer p.out(p.in("expression"))

	choice := ast.NewChoiceExpr(p.tok.pos)
	for {
		expr := p.actionExpr()
		if expr != nil {
			choice.Alternatives = append(choice.Alternatives, expr)
		}
		if p.tok.id != slash {
			switch len(choice.Alternatives) {
			case 0:
				p.errs.add(p.tok.pos, errors.New("no expression in choice"))
				return nil
			case 1:
				return choice.Alternatives[0]
			default:
				return choice
			}
		}
		// move after the slash
		p.read()
	}
}

func (p *Parser) actionExpr() ast.Expression {
	defer p.out(p.in("actionExpr"))

	act := ast.NewActionExpr(p.tok.pos)
	expr := p.seqExpr()
	if expr == nil {
		return nil
	}
	act.Expr = expr

	if p.tok.id == code {
		act.Code = ast.NewCodeBlock(p.tok.pos, p.tok.lit)
		p.read()
	}

	if act.Code == nil {
		return expr
	}
	return act
}

func (p *Parser) seqExpr() ast.Expression {
	defer p.out(p.in("seqExpr"))

	seq := ast.NewSeqExpr(p.tok.pos)
	for {
		expr := p.labeledExpr()
		if expr == nil {
			switch len(seq.Exprs) {
			case 0:
				p.errs.add(p.tok.pos, errors.New("no expression in sequence"))
				return nil
			case 1:
				return seq.Exprs[0]
			default:
				return seq
			}
		}
		seq.Exprs = append(seq.Exprs, expr)
	}
}

func (p *Parser) labeledExpr() ast.Expression {
	defer p.out(p.in("labeledExpr"))

	lab := ast.NewLabeledExpr(p.tok.pos)
	if p.tok.id == ident {
		peek := p.peek()
		if peek.id == colon {
			label := ast.NewIdentifier(p.tok.pos, p.tok.lit)
			lab.Label = label
			p.read()
			if !p.expect(colon) {
				return nil
			}
			p.read()
		}
	}

	expr := p.prefixedExpr()
	if expr == nil {
		if lab.Label != nil {
			p.errs.add(p.tok.pos, errors.New("label without expression"))
		}
		return nil
	}

	if lab.Label != nil {
		lab.Expr = expr
		return lab
	}
	return expr
}

func (p *Parser) prefixedExpr() ast.Expression {
	defer p.out(p.in("prefixedExpr"))

	var pref ast.Expression
	switch p.tok.id {
	case ampersand:
		pref = ast.NewAndExpr(p.tok.pos)
		p.read()
	case exclamation:
		pref = ast.NewNotExpr(p.tok.pos)
		p.read()
	}

	expr := p.suffixedExpr()
	if expr == nil {
		if pref != nil {
			p.errs.add(p.tok.pos, errors.New("prefix operator without expression"))
		}
		return nil
	}
	switch p := pref.(type) {
	case *ast.AndExpr:
		p.Expr = expr
		return p
	case *ast.NotExpr:
		p.Expr = expr
		return p
	default:
		return expr
	}
}

func (p *Parser) suffixedExpr() ast.Expression {
	defer p.out(p.in("suffixedExpr"))

	expr := p.primaryExpr()
	if expr == nil {
		if p.tok.id == question || p.tok.id == star || p.tok.id == plus {
			p.errs.add(p.tok.pos, errors.New("suffix operator without expression"))
		}
		return nil
	}

	switch p.tok.id {
	case question:
		q := ast.NewZeroOrOneExpr(expr.Pos())
		q.Expr = expr
		p.read()
		return q
	case star:
		s := ast.NewZeroOrMoreExpr(expr.Pos())
		s.Expr = expr
		p.read()
		return s
	case plus:
		l := ast.NewOneOrMoreExpr(expr.Pos())
		l.Expr = expr
		p.read()
		return l
	default:
		return expr
	}
}

func (p *Parser) primaryExpr() ast.Expression {
	defer p.out(p.in("primaryExpr"))

	switch p.tok.id {
	case str, rstr, char:
		// literal matcher
		ignore := strings.HasSuffix(p.tok.lit, "i")
		if ignore {
			p.tok.lit = p.tok.lit[:len(p.tok.lit)-1]
		}
		s, err := strconv.Unquote(p.tok.lit)
		if err != nil {
			p.errs.add(p.tok.pos, err)
		}
		lit := ast.NewLitMatcher(p.tok.pos, s)
		lit.IgnoreCase = ignore
		p.read()
		return lit

	case class:
		// character class matcher
		cl := ast.NewCharClassMatcher(p.tok.pos, p.tok.lit)
		p.read()
		return cl

	case dot:
		// any matcher
		any := ast.NewAnyMatcher(p.tok.pos, p.tok.lit)
		p.read()
		return any

	case ident:
		// rule reference expression
		return p.ruleRefExpr()

	case lparen:
		// expression in parenthesis
		p.read()
		expr := p.expression()
		if expr == nil {
			p.errs.add(p.tok.pos, errors.New("missing expression inside parenthesis"))
			return nil
		}
		if !p.expect(rparen) {
			return nil
		}
		p.read()
		return expr

	default:
		// if p.tok.id != eof && p.tok.id != eol && p.tok.id != semicolon {
		// 	p.errs.add(p.tok.pos, fmt.Errorf("invalid token %s (%q) for primary expression", p.tok.id, p.tok.lit))
		// }
		return nil
	}
}

func (p *Parser) ruleRefExpr() ast.Expression {
	defer p.out(p.in("ruleRefExpr"))

	if !p.expect(ident) {
		return nil
	}
	expr := ast.NewRuleRefExpr(p.tok.pos)
	expr.Name = ast.NewIdentifier(p.tok.pos, p.tok.lit)
	p.read()
	return expr
}
