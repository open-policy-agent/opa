// Package builder generates the parser code for a given grammar. It makes
// no attempt to verify the correctness of the grammar.
package builder

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/pigeon/ast"
)

// generated function templates
var (
	onFuncTemplate = `func (%s *current) %s(%s) (interface{}, error) {
%s
}
`
	onPredFuncTemplate = `func (%s *current) %s(%s) (bool, error) {
%s
}
`
	callFuncTemplate = `func (p *parser) call%s() (interface{}, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.%[1]s(%s)
}
`
	callPredFuncTemplate = `func (p *parser) call%s() (bool, error) {
	stack := p.vstack[len(p.vstack)-1]
	_ = stack
	return p.cur.%[1]s(%s)
}
`
)

// Option is a function that can set an option on the builder. It returns
// the previous setting as an Option.
type Option func(*builder) Option

// ReceiverName returns an option that specifies the receiver name to
// use for the current struct (which is the struct on which all code blocks
// except the initializer are generated).
func ReceiverName(nm string) Option {
	return func(b *builder) Option {
		prev := b.recvName
		b.recvName = nm
		return ReceiverName(prev)
	}
}

// BuildParser builds the PEG parser using the provider grammar. The code is
// written to the specified w.
func BuildParser(w io.Writer, g *ast.Grammar, opts ...Option) error {
	b := &builder{w: w, recvName: "c"}
	b.setOptions(opts)
	return b.buildParser(g)
}

type builder struct {
	w   io.Writer
	err error

	// options
	recvName string

	ruleName  string
	exprIndex int
	argsStack [][]string
}

func (b *builder) setOptions(opts []Option) {
	for _, opt := range opts {
		opt(b)
	}
}

func (b *builder) buildParser(g *ast.Grammar) error {
	b.writeInit(g.Init)
	b.writeGrammar(g)

	for _, rule := range g.Rules {
		b.writeRuleCode(rule)
	}
	b.writeStaticCode()

	return b.err
}

func (b *builder) writeInit(init *ast.CodeBlock) {
	if init == nil {
		return
	}

	// remove opening and closing braces
	val := init.Val[1 : len(init.Val)-1]
	b.writelnf("%s", val)
}

func (b *builder) writeGrammar(g *ast.Grammar) {
	// transform the ast grammar to the self-contained, no dependency version
	// of the parser-generator grammar.
	b.writelnf("var g = &grammar {")
	b.writelnf("\trules: []*rule{")
	for _, r := range g.Rules {
		b.writeRule(r)
	}
	b.writelnf("\t},")
	b.writelnf("}")
}

func (b *builder) writeRule(r *ast.Rule) {
	if r == nil || r.Name == nil {
		return
	}

	b.exprIndex = 0
	b.ruleName = r.Name.Val

	b.writelnf("{")
	b.writelnf("\tname: %q,", r.Name.Val)
	if r.DisplayName != nil && r.DisplayName.Val != "" {
		b.writelnf("\tdisplayName: %q,", r.DisplayName.Val)
	}
	pos := r.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writef("\texpr: ")
	b.writeExpr(r.Expr)
	b.writelnf("},")
}

func (b *builder) writeExpr(expr ast.Expression) {
	b.exprIndex++
	switch expr := expr.(type) {
	case *ast.ActionExpr:
		b.writeActionExpr(expr)
	case *ast.AndCodeExpr:
		b.writeAndCodeExpr(expr)
	case *ast.AndExpr:
		b.writeAndExpr(expr)
	case *ast.AnyMatcher:
		b.writeAnyMatcher(expr)
	case *ast.CharClassMatcher:
		b.writeCharClassMatcher(expr)
	case *ast.ChoiceExpr:
		b.writeChoiceExpr(expr)
	case *ast.LabeledExpr:
		b.writeLabeledExpr(expr)
	case *ast.LitMatcher:
		b.writeLitMatcher(expr)
	case *ast.NotCodeExpr:
		b.writeNotCodeExpr(expr)
	case *ast.NotExpr:
		b.writeNotExpr(expr)
	case *ast.OneOrMoreExpr:
		b.writeOneOrMoreExpr(expr)
	case *ast.RuleRefExpr:
		b.writeRuleRefExpr(expr)
	case *ast.SeqExpr:
		b.writeSeqExpr(expr)
	case *ast.ZeroOrMoreExpr:
		b.writeZeroOrMoreExpr(expr)
	case *ast.ZeroOrOneExpr:
		b.writeZeroOrOneExpr(expr)
	default:
		b.err = fmt.Errorf("builder: unknown expression type %T", expr)
	}
}

func (b *builder) writeActionExpr(act *ast.ActionExpr) {
	if act == nil {
		b.writelnf("nil,")
		return
	}
	act.FuncIx = b.exprIndex
	b.writelnf("&actionExpr{")
	pos := act.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writelnf("\trun: (*parser).call%s,", b.funcName(act.FuncIx))
	b.writef("\texpr: ")
	b.writeExpr(act.Expr)
	b.writelnf("},")
}

func (b *builder) writeAndCodeExpr(and *ast.AndCodeExpr) {
	if and == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&andCodeExpr{")
	pos := and.Pos()
	and.FuncIx = b.exprIndex
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writelnf("\trun: (*parser).call%s,", b.funcName(and.FuncIx))
	b.writelnf("},")
}

func (b *builder) writeAndExpr(and *ast.AndExpr) {
	if and == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&andExpr{")
	pos := and.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writef("\texpr: ")
	b.writeExpr(and.Expr)
	b.writelnf("},")
}

func (b *builder) writeAnyMatcher(any *ast.AnyMatcher) {
	if any == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&anyMatcher{")
	pos := any.Pos()
	b.writelnf("\tline: %d, col: %d, offset: %d,", pos.Line, pos.Col, pos.Off)
	b.writelnf("},")
}

func (b *builder) writeCharClassMatcher(ch *ast.CharClassMatcher) {
	if ch == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&charClassMatcher{")
	pos := ch.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writelnf("\tval: %q,", ch.Val)
	if len(ch.Chars) > 0 {
		b.writef("\tchars: []rune{")
		for _, rn := range ch.Chars {
			if ch.IgnoreCase {
				b.writef("%q,", unicode.ToLower(rn))
			} else {
				b.writef("%q,", rn)
			}
		}
		b.writelnf("},")
	}
	if len(ch.Ranges) > 0 {
		b.writef("\tranges: []rune{")
		for _, rn := range ch.Ranges {
			if ch.IgnoreCase {
				b.writef("%q,", unicode.ToLower(rn))
			} else {
				b.writef("%q,", rn)
			}
		}
		b.writelnf("},")
	}
	if len(ch.UnicodeClasses) > 0 {
		b.writef("\tclasses: []*unicode.RangeTable{")
		for _, cl := range ch.UnicodeClasses {
			b.writef("rangeTable(%q),", cl)
		}
		b.writelnf("},")
	}
	b.writelnf("\tignoreCase: %t,", ch.IgnoreCase)
	b.writelnf("\tinverted: %t,", ch.Inverted)
	b.writelnf("},")
}

func (b *builder) writeChoiceExpr(ch *ast.ChoiceExpr) {
	if ch == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&choiceExpr{")
	pos := ch.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	if len(ch.Alternatives) > 0 {
		b.writelnf("\talternatives: []interface{}{")
		for _, alt := range ch.Alternatives {
			b.writeExpr(alt)
		}
		b.writelnf("\t},")
	}
	b.writelnf("},")
}

func (b *builder) writeLabeledExpr(lab *ast.LabeledExpr) {
	if lab == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&labeledExpr{")
	pos := lab.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	if lab.Label != nil && lab.Label.Val != "" {
		b.writelnf("\tlabel: %q,", lab.Label.Val)
	}
	b.writef("\texpr: ")
	b.writeExpr(lab.Expr)
	b.writelnf("},")
}

func (b *builder) writeLitMatcher(lit *ast.LitMatcher) {
	if lit == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&litMatcher{")
	pos := lit.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	if lit.IgnoreCase {
		b.writelnf("\tval: %q,", strings.ToLower(lit.Val))
	} else {
		b.writelnf("\tval: %q,", lit.Val)
	}
	b.writelnf("\tignoreCase: %t,", lit.IgnoreCase)
	b.writelnf("},")
}

func (b *builder) writeNotCodeExpr(not *ast.NotCodeExpr) {
	if not == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&notCodeExpr{")
	pos := not.Pos()
	not.FuncIx = b.exprIndex
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writelnf("\trun: (*parser).call%s,", b.funcName(not.FuncIx))
	b.writelnf("},")
}

func (b *builder) writeNotExpr(not *ast.NotExpr) {
	if not == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&notExpr{")
	pos := not.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writef("\texpr: ")
	b.writeExpr(not.Expr)
	b.writelnf("},")
}

func (b *builder) writeOneOrMoreExpr(one *ast.OneOrMoreExpr) {
	if one == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&oneOrMoreExpr{")
	pos := one.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writef("\texpr: ")
	b.writeExpr(one.Expr)
	b.writelnf("},")
}

func (b *builder) writeRuleRefExpr(ref *ast.RuleRefExpr) {
	if ref == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&ruleRefExpr{")
	pos := ref.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	if ref.Name != nil && ref.Name.Val != "" {
		b.writelnf("\tname: %q,", ref.Name.Val)
	}
	b.writelnf("},")
}

func (b *builder) writeSeqExpr(seq *ast.SeqExpr) {
	if seq == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&seqExpr{")
	pos := seq.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	if len(seq.Exprs) > 0 {
		b.writelnf("\texprs: []interface{}{")
		for _, e := range seq.Exprs {
			b.writeExpr(e)
		}
		b.writelnf("\t},")
	}
	b.writelnf("},")
}

func (b *builder) writeZeroOrMoreExpr(zero *ast.ZeroOrMoreExpr) {
	if zero == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&zeroOrMoreExpr{")
	pos := zero.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writef("\texpr: ")
	b.writeExpr(zero.Expr)
	b.writelnf("},")
}

func (b *builder) writeZeroOrOneExpr(zero *ast.ZeroOrOneExpr) {
	if zero == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&zeroOrOneExpr{")
	pos := zero.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writef("\texpr: ")
	b.writeExpr(zero.Expr)
	b.writelnf("},")
}

func (b *builder) writeRuleCode(rule *ast.Rule) {
	if rule == nil || rule.Name == nil {
		return
	}

	// keep trace of the current rule, as the code blocks are created
	// in functions named "on<RuleName><#ExprIndex>".
	b.ruleName = rule.Name.Val
	b.pushArgsSet()
	b.writeExprCode(rule.Expr)
	b.popArgsSet()
}

func (b *builder) pushArgsSet() {
	b.argsStack = append(b.argsStack, nil)
}

func (b *builder) popArgsSet() {
	b.argsStack = b.argsStack[:len(b.argsStack)-1]
}

func (b *builder) addArg(arg *ast.Identifier) {
	if arg == nil {
		return
	}
	ix := len(b.argsStack) - 1
	b.argsStack[ix] = append(b.argsStack[ix], arg.Val)
}

func (b *builder) writeExprCode(expr ast.Expression) {
	switch expr := expr.(type) {
	case *ast.ActionExpr:
		b.writeExprCode(expr.Expr)
		b.writeActionExprCode(expr)

	case *ast.AndCodeExpr:
		b.writeAndCodeExprCode(expr)

	case *ast.LabeledExpr:
		b.addArg(expr.Label)
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.popArgsSet()

	case *ast.NotCodeExpr:
		b.writeNotCodeExprCode(expr)

	case *ast.AndExpr:
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.popArgsSet()
	case *ast.ChoiceExpr:
		for _, alt := range expr.Alternatives {
			b.pushArgsSet()
			b.writeExprCode(alt)
			b.popArgsSet()
		}
	case *ast.NotExpr:
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.popArgsSet()
	case *ast.OneOrMoreExpr:
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.popArgsSet()
	case *ast.SeqExpr:
		for _, sub := range expr.Exprs {
			b.writeExprCode(sub)
		}
	case *ast.ZeroOrMoreExpr:
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.popArgsSet()
	case *ast.ZeroOrOneExpr:
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.popArgsSet()
	}
}

func (b *builder) writeActionExprCode(act *ast.ActionExpr) {
	if act == nil {
		return
	}
	b.writeFunc(act.FuncIx, act.Code, callFuncTemplate, onFuncTemplate)
}

func (b *builder) writeAndCodeExprCode(and *ast.AndCodeExpr) {
	if and == nil {
		return
	}
	b.writeFunc(and.FuncIx, and.Code, callPredFuncTemplate, onPredFuncTemplate)
}

func (b *builder) writeNotCodeExprCode(not *ast.NotCodeExpr) {
	if not == nil {
		return
	}
	b.writeFunc(not.FuncIx, not.Code, callPredFuncTemplate, onPredFuncTemplate)
}

func (b *builder) writeFunc(funcIx int, code *ast.CodeBlock, callTpl, funcTpl string) {
	if code == nil {
		return
	}
	val := strings.TrimSpace(code.Val)[1 : len(code.Val)-1]
	if len(val) > 0 && val[0] == '\n' {
		val = val[1:]
	}
	if len(val) > 0 && val[len(val)-1] == '\n' {
		val = val[:len(val)-1]
	}
	var args bytes.Buffer
	ix := len(b.argsStack) - 1
	if ix >= 0 {
		for i, arg := range b.argsStack[ix] {
			if i > 0 {
				args.WriteString(", ")
			}
			args.WriteString(arg)
		}
	}
	if args.Len() > 0 {
		args.WriteString(" interface{}")
	}

	fnNm := b.funcName(funcIx)
	b.writelnf(funcTpl, b.recvName, fnNm, args.String(), val)

	args.Reset()
	if ix >= 0 {
		for i, arg := range b.argsStack[ix] {
			if i > 0 {
				args.WriteString(", ")
			}
			args.WriteString(fmt.Sprintf(`stack[%q]`, arg))
		}
	}
	b.writelnf(callTpl, fnNm, args.String())
}

func (b *builder) writeStaticCode() {
	b.writelnf(staticCode)
}

func (b *builder) funcName(ix int) string {
	return "on" + b.ruleName + strconv.Itoa(ix)
}

func (b *builder) writef(f string, args ...interface{}) {
	if b.err == nil {
		_, b.err = fmt.Fprintf(b.w, f, args...)
	}
}

func (b *builder) writelnf(f string, args ...interface{}) {
	b.writef(f+"\n", args...)
}
