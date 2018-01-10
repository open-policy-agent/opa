// Package builder generates the parser code for a given grammar. It makes
// no attempt to verify the correctness of the grammar.
package builder

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"regexp"

	"github.com/mna/pigeon/ast"
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
	onStateFuncTemplate = `func (%s *current) %s(%s) (error) {
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
	callStateFuncTemplate = `func (p *parser) call%s() error {
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

// Optimize returns an option that specifies the optimize option
// If optimize is true, the Debug and Memoize code is completely
// removed from the resulting parser
func Optimize(optimize bool) Option {
	return func(b *builder) Option {
		prev := b.optimize
		b.optimize = optimize
		return Optimize(prev)
	}
}

// BasicLatinLookupTable returns an option that specifies the basicLatinLookup option
// If basicLatinLookup is true, a lookup slice for the first 128 chars of
// the Unicode table (Basic Latin) is generated for each CharClassMatcher
// to increase the character matching.
func BasicLatinLookupTable(basicLatinLookupTable bool) Option {
	return func(b *builder) Option {
		prev := b.basicLatinLookupTable
		b.basicLatinLookupTable = basicLatinLookupTable
		return BasicLatinLookupTable(prev)
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
	recvName              string
	optimize              bool
	basicLatinLookupTable bool
	globalState           bool

	ruleName  string
	exprIndex int
	argsStack [][]string

	rangeTable bool
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
	case *ast.RecoveryExpr:
		b.writeRecoveryExpr(expr)
	case *ast.RuleRefExpr:
		b.writeRuleRefExpr(expr)
	case *ast.SeqExpr:
		b.writeSeqExpr(expr)
	case *ast.StateCodeExpr:
		b.writeStateCodeExpr(expr)
	case *ast.ThrowExpr:
		b.writeThrowExpr(expr)
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
		b.rangeTable = true
		b.writef("\tclasses: []*unicode.RangeTable{")
		for _, cl := range ch.UnicodeClasses {
			b.writef("rangeTable(%q),", cl)
		}
		b.writelnf("},")
	}
	if b.basicLatinLookupTable {
		b.writelnf("\tbasicLatinChars: %#v,", BasicLatinLookup(ch.Chars, ch.Ranges, ch.UnicodeClasses, ch.IgnoreCase))
	}
	b.writelnf("\tignoreCase: %t,", ch.IgnoreCase)
	b.writelnf("\tinverted: %t,", ch.Inverted)
	b.writelnf("},")
}

// BasicLatinLookup calculates the decision results for the first 256 characters of the UTF-8 character
// set for a given set of chars, ranges and unicodeClasses to speedup the CharClassMatcher.
func BasicLatinLookup(chars, ranges []rune, unicodeClasses []string, ignoreCase bool) (basicLatinChars [128]bool) {
	for _, rn := range chars {
		if rn < 128 {
			basicLatinChars[rn] = true
			if ignoreCase {
				if unicode.IsLower(rn) {
					basicLatinChars[unicode.ToUpper(rn)] = true
				} else {
					basicLatinChars[unicode.ToLower(rn)] = true
				}
			}
		}
	}
	for i := 0; i < len(ranges); i += 2 {
		if ranges[i] < 128 {
			for j := ranges[i]; j < 128 && j <= ranges[i+1]; j++ {
				basicLatinChars[j] = true
				if ignoreCase {
					if unicode.IsLower(j) {
						basicLatinChars[unicode.ToUpper(j)] = true
					} else {
						basicLatinChars[unicode.ToLower(j)] = true
					}
				}
			}
		}
	}
	for _, cl := range unicodeClasses {
		rt := rangeTable(cl)
		for r := rune(0); r < 128; r++ {
			if unicode.Is(rt, r) {
				basicLatinChars[r] = true
			}
		}
	}
	return
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

func (b *builder) writeRecoveryExpr(recover *ast.RecoveryExpr) {
	if recover == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&recoveryExpr{")
	pos := recover.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)

	b.writef("\texpr: ")
	b.writeExpr(recover.Expr)
	b.writef("\trecoverExpr: ")
	b.writeExpr(recover.RecoverExpr)
	b.writelnf("\tfailureLabel: []string{")
	for _, label := range recover.Labels {
		b.writelnf("%q,", label)
	}
	b.writelnf("\t},")
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

func (b *builder) writeStateCodeExpr(state *ast.StateCodeExpr) {
	if state == nil {
		b.writelnf("nil,")
		return
	}
	b.globalState = true
	b.writelnf("&stateCodeExpr{")
	pos := state.Pos()
	state.FuncIx = b.exprIndex
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writelnf("\trun: (*parser).call%s,", b.funcName(state.FuncIx))
	b.writelnf("},")
}

func (b *builder) writeThrowExpr(throw *ast.ThrowExpr) {
	if throw == nil {
		b.writelnf("nil,")
		return
	}
	b.writelnf("&throwExpr{")
	pos := throw.Pos()
	b.writelnf("\tpos: position{line: %d, col: %d, offset: %d},", pos.Line, pos.Col, pos.Off)
	b.writelnf("\tlabel: %q,", throw.Label)
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

	case *ast.RecoveryExpr:
		b.pushArgsSet()
		b.writeExprCode(expr.Expr)
		b.writeExprCode(expr.RecoverExpr)
		b.popArgsSet()

	case *ast.SeqExpr:
		for _, sub := range expr.Exprs {
			b.writeExprCode(sub)
		}

	case *ast.StateCodeExpr:
		b.writeStateCodeExprCode(expr)

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

func (b *builder) writeStateCodeExprCode(state *ast.StateCodeExpr) {
	if state == nil {
		return
	}
	b.writeFunc(state.FuncIx, state.Code, callStateFuncTemplate, onStateFuncTemplate)
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
	buffer := bytes.NewBufferString("")
	params := struct {
		Optimize              bool
		BasicLatinLookupTable bool
		GlobalState           bool
	}{
		Optimize:              b.optimize,
		BasicLatinLookupTable: b.basicLatinLookupTable,
		GlobalState:           b.globalState,
	}
	t := template.Must(template.New("static_code").Parse(staticCode))

	err := t.Execute(buffer, params)
	if err != nil {
		// This is very unlikely to ever happen
		panic("executing template: " + err.Error())
	}

	// Clean the ==template== comments from the generated parser
	lines := strings.Split(buffer.String(), "\n")
	buffer.Reset()
	re := regexp.MustCompile(`^\s*//\s*(==template==\s*)+$`)
	for _, line := range lines {
		if !re.MatchString(line) {
			_, err := buffer.WriteString(line + "\n")
			if err != nil {
				// This is very unlikely to ever happen
				panic("unable to write to byte buffer: " + err.Error())
			}
		}
	}

	b.writeln(buffer.String())
	if b.rangeTable {
		b.writeln(rangeTable0)
	}
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

func (b *builder) writeln(f string) {
	if b.err == nil {
		_, b.err = fmt.Fprint(b.w, f+"\n")
	}
}
