// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package format implements formatting of Rego source files.
package format

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/open-policy-agent/opa/internal/future"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/types"
	"github.com/open-policy-agent/opa/v1/util"
)

// defaultLocationFile is the file name used in `Ast()` for terms
// without a location, as could happen when pretty-printing the
// results of partial eval.
const defaultLocationFile = "__format_default__"

var (
	expandedConst     = ast.NewBody(ast.NewExpr(ast.InternedTerm(true)))
	commentsSlicePool = util.NewSlicePool[*ast.Comment](50)
)

// Opts lets you control the code formatting via `AstWithOpts()`.
type Opts struct {
	// IgnoreLocations instructs the formatter not to use the AST nodes' locations
	// into account when laying out the code: notably, when the input is the result
	// of partial evaluation, arguments maybe have been shuffled around, but still
	// carry along their original source locations.
	IgnoreLocations bool

	// RegoVersion is the version of Rego to format code for.
	RegoVersion ast.RegoVersion

	// ParserOptions is the parser options used when parsing the module to be formatted.
	ParserOptions *ast.ParserOptions

	// DropV0Imports instructs the formatter to drop all v0 imports from the module; i.e. 'rego.v1' and 'future.keywords' imports.
	// Imports are only removed if [Opts.RegoVersion] makes them redundant.
	DropV0Imports bool

	// SkipDefensiveCopying, if true, will avoid deep-copying the AST before formatting it.
	// This is true by default for all Source* functions, but false by default for Ast* functions,
	// as some formatting operations may otherwise mutate the AST.
	SkipDefensiveCopying bool

	Capabilities *ast.Capabilities
}

func (o Opts) effectiveRegoVersion() ast.RegoVersion {
	if o.RegoVersion == ast.RegoUndefined {
		return ast.DefaultRegoVersion
	}
	return o.RegoVersion
}

// Source formats a Rego source file. The bytes provided must describe a complete
// Rego module. If they don't, Source will return an error resulting from the attempt
// to parse the bytes.
func Source(filename string, src []byte) ([]byte, error) {
	return SourceWithOpts(filename, src, Opts{SkipDefensiveCopying: true})
}

func SourceWithOpts(filename string, src []byte, opts Opts) ([]byte, error) {
	regoVersion := opts.effectiveRegoVersion()

	var parserOpts ast.ParserOptions
	if opts.ParserOptions != nil {
		parserOpts = *opts.ParserOptions
	} else if regoVersion == ast.RegoV1 {
		// If the rego version is V1, we need to parse it as such, to allow for future keywords not being imported.
		// Otherwise, we'll default to the default rego-version.
		parserOpts.RegoVersion = ast.RegoV1
	}

	// Copying the node does not make sense when both input and output are bytes.
	opts.SkipDefensiveCopying = true

	if parserOpts.RegoVersion == ast.RegoUndefined {
		parserOpts.RegoVersion = ast.DefaultRegoVersion
	}

	module, err := ast.ParseModuleWithOpts(filename, string(src), parserOpts)
	if err != nil {
		return nil, err
	}

	if regoVersion == ast.RegoV0CompatV1 || regoVersion == ast.RegoV1 {
		checkOpts := ast.NewRegoCheckOptions()
		// The module is parsed as v0, so we need to disable checks that will be automatically amended by the AstWithOpts call anyways.
		checkOpts.RequireIfKeyword = false
		checkOpts.RequireContainsKeyword = false
		checkOpts.RequireRuleBodyOrValue = false
		errs := ast.CheckRegoV1WithOptions(module, checkOpts)
		if len(errs) > 0 {
			return nil, errs
		}
	}

	formatted, err := AstWithOpts(module, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", filename, err)
	}

	return formatted, nil
}

// MustAst is a helper function to format a Rego AST element. If any errors
// occur this function will panic. This is mostly used for test
func MustAst(x any) []byte {
	bs, err := Ast(x)
	if err != nil {
		panic(err)
	}
	return bs
}

// MustAstWithOpts is a helper function to format a Rego AST element. If any errors
// occur this function will panic. This is mostly used for test
func MustAstWithOpts(x any, opts Opts) []byte {
	bs, err := AstWithOpts(x, opts)
	if err != nil {
		panic(err)
	}
	return bs
}

// Ast formats a Rego AST element. If the passed value is not a valid AST
// element, Ast returns nil and an error. If AST nodes are missing locations
// an arbitrary location will be used.
func Ast(x any) ([]byte, error) {
	return AstWithOpts(x, Opts{})
}

type fmtOpts struct {
	// When the future keyword "contains" is imported, all the pretty-printed
	// modules will use that format for partial sets.
	// NOTE(sr): For ref-head rules, this will be the default behaviour, since
	// we need "contains" to disambiguate complete rules from partial sets.
	contains bool

	// Same logic applies as for "contains": if `future.keywords.if` (or all
	// future keywords) is imported, we'll render rules that can use `if` with
	// `if`.
	ifs bool

	// We check all rule ref heads to see if any of them _requires_ support
	// for ref heads -- if they do, we'll print all of them in a different way
	// than if they don't.
	refHeads bool

	regoV1         bool
	regoV1Imported bool
	futureKeywords []string

	// If true, the formatter will retain keywords in refs, e.g. `p.not ` instead of `p["not"]`.
	// The format of the original ref is preserved, so `p["not"]` will still be formatted as `p["not"]`.
	allowKeywordsInRefs bool
}

func (o fmtOpts) keywords() []string {
	if o.regoV1 {
		return append(ast.KeywordsV1[:], o.futureKeywords...)
	}
	kws := ast.KeywordsV0[:]
	return append(kws, o.futureKeywords...)
}

func AstWithOpts(x any, opts Opts) ([]byte, error) {
	// The node has to be deep copied because it may be mutated below. Alternatively,
	// we could avoid the copy by checking if mutation will occur first. For now,
	// since format is not latency sensitive, just deep copy in all cases.
	if !opts.SkipDefensiveCopying {
		x = ast.Copy(x)
	}

	wildcards := map[ast.Var]*ast.Term{}

	// NOTE(sr): When the formatter encounters a call to internal.member_2
	// or internal.member_3, it will sugarize them into usage of the `in`
	// operator. It has to ensure that the proper future keyword import is
	// present.
	extraFutureKeywordImports := map[string]struct{}{}

	o := fmtOpts{}

	regoVersion := opts.effectiveRegoVersion()
	if regoVersion == ast.RegoV0CompatV1 || regoVersion == ast.RegoV1 {
		o.regoV1 = true
		o.ifs = true
		o.contains = true
	}

	capabilities := opts.Capabilities
	if capabilities == nil {
		capabilities = ast.CapabilitiesForThisVersion(ast.CapabilitiesRegoVersion(opts.effectiveRegoVersion()))
	}
	o.allowKeywordsInRefs = capabilities.ContainsFeature(ast.FeatureKeywordsInRefs)

	// Preprocess the AST. Set any required defaults and calculate
	// values required for printing the formatted output.
	ast.WalkNodes(x, func(x ast.Node) bool {
		switch n := x.(type) {
		case ast.Body:
			if len(n) == 0 {
				return false
			}
		case *ast.Term:
			unmangleWildcardVar(wildcards, n)

		case *ast.Expr:
			switch {
			case n.IsCall() && ast.Interned.Refs.Member.Equal(n.Operator()) ||
				ast.Interned.Refs.MemberWithKey.Equal(n.Operator()):
				extraFutureKeywordImports["in"] = struct{}{}
			case n.IsEvery():
				extraFutureKeywordImports["every"] = struct{}{}
			case n.IsNot():
				extraFutureKeywordImports["not"] = struct{}{}
			case n.IsAnd():
				extraFutureKeywordImports["and"] = struct{}{}
			case n.IsOr():
				extraFutureKeywordImports["or"] = struct{}{}
			}

			if n.Negated && isLogicalExpr(n) {
				// A negated logical expression is written parenthesized
				// (`not (a or b)`), which requires the `not` keyword.
				extraFutureKeywordImports["not"] = struct{}{}
			}

		case *ast.Import:
			if kw, ok := future.WhichFutureKeyword(n); ok {
				o.futureKeywords = append(o.futureKeywords, kw)
			}

			switch {
			case isRegoV1Compatible(n):
				o.regoV1Imported = true
				o.contains = true
				o.ifs = true
			case future.IsAllFutureKeywords(n):
				o.contains = true
				o.ifs = true
			case future.IsFutureKeyword(n, "contains"):
				o.contains = true
			case future.IsFutureKeyword(n, "if"):
				o.ifs = true
			}

		case *ast.Rule:
			headLen := len(n.Head.Ref())
			if headLen > 2 {
				o.refHeads = true
			}
			if headLen == 2 && n.Head.Key != nil && n.Head.Value == nil { // p.q contains "x"
				o.refHeads = true
			}
		}

		if opts.IgnoreLocations || x.Loc() == nil {
			x.SetLoc(defaultLocation(x))
		}
		return false
	})

	w := &writer{
		indent:  "\t",
		errs:    make([]*ast.Error, 0),
		fmtOpts: o,
	}

	switch x := x.(type) {
	case *ast.Module:
		if regoVersion == ast.RegoV1 && opts.DropV0Imports {
			x.Imports = filterRegoV1Import(x.Imports)
		} else if regoVersion == ast.RegoV0CompatV1 {
			x.Imports = ensureRegoV1Import(x.Imports)
		}

		regoV1Imported := slices.ContainsFunc(x.Imports, isRegoV1Compatible)

		if regoVersion == ast.RegoV0CompatV1 || regoVersion == ast.RegoV1 || regoV1Imported {
			if !opts.DropV0Imports && !regoV1Imported {
				for _, kw := range o.futureKeywords {
					x.Imports = ensureFutureKeywordImport(x.Imports, kw)
				}
			} else {
				x.Imports = future.FilterFutureImports(x.Imports)
			}

			for kw := range extraFutureKeywordImports {
				if ast.IsFutureKeywordForRegoVersion(kw, ast.RegoV1) {
					x.Imports = ensureFutureKeywordImport(x.Imports, kw)
				}
			}
		} else {
			for kw := range extraFutureKeywordImports {
				x.Imports = ensureFutureKeywordImport(x.Imports, kw)
			}
		}

		err := w.writeModule(x)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.Package:
		_, err := w.writePackage(x, nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.Import:
		_, err := w.writeImports([]*ast.Import{x}, nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.Rule:
		_, err := w.writeRule(x, false /* isElse */, nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.Head:
		_, err := w.writeHead(x,
			false, // isDefault
			false, // isExpandedConst
			nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case ast.Body:
		_, err := w.writeBody(x, nil)
		if err != nil {
			return nil, err
		}
	case *ast.Expr:
		_, err := w.writeExpr(x, nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.With:
		_, err := w.writeWith(x, nil, false)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.Term:
		_, err := w.writeTerm(x, nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case ast.Value:
		_, err := w.writeTerm(&ast.Term{Value: x, Location: &ast.Location{}}, nil)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	case *ast.Comment:
		err := w.writeComments([]*ast.Comment{x})
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
	default:
		return nil, fmt.Errorf("not an ast element: %v", x)
	}

	if len(w.errs) > 0 {
		return nil, w.errs
	}

	return squashTrailingNewlines(w.buf.Bytes()), nil
}

func unmangleWildcardVar(wildcards map[ast.Var]*ast.Term, n *ast.Term) {

	v, ok := n.Value.(ast.Var)
	if !ok || !v.IsWildcard() {
		return
	}

	first, ok := wildcards[v]
	if !ok {
		wildcards[v] = n
		return
	}

	w := v[len(ast.WildcardPrefix):]

	// Prepend an underscore to ensure the variable will parse.
	if len(w) == 0 || w[0] != '_' {
		w = "_" + w
	}

	if first != nil {
		first.Value = w
		wildcards[v] = nil
	}

	n.Value = w
}

func squashTrailingNewlines(bs []byte) []byte {
	if bytes.HasSuffix(bs, []byte("\n")) {
		return append(bytes.TrimRight(bs, "\n"), '\n')
	}
	return bs
}

func defaultLocation(x ast.Node) *ast.Location {
	return ast.NewLocation([]byte(x.String()), defaultLocationFile, 1, 1)
}

type writer struct {
	// parenExpr, when set, is an expression whose terms must be wrapped in parens
	// when written; consumed by the first writeExpr that sees it. Any `with`
	// clauses stay outside the parens, as `(x | y with p as 1)` doesn't parse.
	parenExpr *ast.Expr

	// parenTerm, when set, is a term that must be wrapped in parens when written;
	// consumed by the first writeTermParens that sees it.
	parenTerm *ast.Term

	buf bytes.Buffer

	indent                  string
	level                   int
	inline                  bool
	beforeEnd               *ast.Comment
	delay                   bool
	errs                    ast.Errors
	fmtOpts                 fmtOpts
	writeCommentOnFinalLine bool
}

func (w *writer) writeModule(module *ast.Module) error {
	var pkg *ast.Package
	var others []any
	var comments []*ast.Comment
	visitor := ast.NewGenericVisitor(func(x any) bool {
		switch x := x.(type) {
		case *ast.Comment:
			comments = append(comments, x)
			return true
		case *ast.Import, *ast.Rule:
			others = append(others, x)
			return true
		case *ast.Package:
			pkg = x
			return true
		default:
			return false
		}
	})
	visitor.Walk(module)

	slices.SortFunc(comments, func(a, b *ast.Comment) int {
		al, bl, err := getLocs(a, b)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
		return locCmp(al, bl)
	})

	slices.SortFunc(others, func(a, b any) int {
		al, bl, err := getLocs(a, b)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
		return locCmp(al, bl)
	})

	comments = trimTrailingWhitespaceInComments(comments)

	// Imports added by the formatter get an assigned line number, which can sort
	// after a rule's. An import written after a rule has no effect.
	var added []*ast.Import
	if addedImportFollowsRule(others) {
		others = slices.DeleteFunc(others, func(x any) bool {
			imp, ok := x.(*ast.Import)
			if !ok || !isAddedImport(imp) {
				return false
			}
			added = append(added, imp)
			return true
		})
	}

	var err error
	comments, err = w.writePackage(pkg, comments)
	if err != nil {
		return err
	}
	comments, err = w.writeImports(added, comments)
	if err != nil {
		return err
	}
	var imports []*ast.Import
	var rules []*ast.Rule
	for len(others) > 0 {
		imports, others = gatherImports(others)
		comments, err = w.writeImports(imports, comments)
		if err != nil {
			return err
		}
		rules, others = gatherRules(others)
		comments, err = w.writeRules(rules, comments)
		if err != nil {
			return err
		}
	}

	for i, c := range comments {
		w.writeLine(c.String())
		if i == len(comments)-1 {
			w.write("\n")
		}
	}

	return nil
}

func trimTrailingWhitespaceInComments(comments []*ast.Comment) []*ast.Comment {
	for _, c := range comments {
		c.Text = bytes.TrimRightFunc(c.Text, unicode.IsSpace)
	}

	return comments
}

func (w *writer) writePackage(pkg *ast.Package, comments []*ast.Comment) ([]*ast.Comment, error) {
	var err error
	comments, err = w.insertComments(comments, pkg.Location)
	if err != nil {
		return nil, err
	}

	w.startLine()

	// Omit head as all packages have the DefaultRootDocument prepended at parse time.
	path := make(ast.Ref, len(pkg.Path)-1)
	if len(path) == 0 {
		w.errs = append(w.errs, ast.NewError(ast.FormatErr, pkg.Location, "invalid package path: %s", pkg.Path))
		return comments, nil
	}

	path[0] = ast.VarTerm(string(pkg.Path[1].Value.(ast.String)))
	copy(path[1:], pkg.Path[2:])

	w.write("package ")
	_, err = w.writeRef(path, nil)
	if err != nil {
		return nil, err
	}

	w.blankLine()

	return comments, nil
}

func (w *writer) writeComments(comments []*ast.Comment) error {
	var inMetadataBlock bool
	for i := range comments {
		if i > 0 {
			l, err := locCmpOrError(comments[i], comments[i-1])
			if err != nil {
				return err
			}
			if l > 1 {
				w.blankLine()
				inMetadataBlock = false
			} else if l == 1 {
				// if next comment is a metadata header and previous comment
				// was part of a metadata block, add a blank line to separate them
				if inMetadataBlock && ast.IsMetadataComment(comments[i]) {
					w.blankLine()
				}
			}
		}

		if ast.IsMetadataComment(comments[i]) {
			inMetadataBlock = true
		}

		w.writeLine(comments[i].String())
	}

	return nil
}

func (w *writer) writeRules(rules []*ast.Rule, comments []*ast.Comment) ([]*ast.Comment, error) {
	for i, rule := range rules {
		var err error
		comments, err = w.insertComments(comments, rule.Location)
		if err != nil && !errors.As(err, &unexpectedCommentError{}) {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}

		comments, err = w.writeRule(rule, false, comments)
		if err != nil && !errors.As(err, &unexpectedCommentError{}) {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}

		if i < len(rules)-1 && w.groupableOneLiner(rule) {
			next := rules[i+1]
			if w.groupableOneLiner(next) && next.Location.Row == rule.Location.Row+1 {
				// Current rule and the next are both groupable one-liners, and
				// adjacent in the original policy (i.e. no extra newlines between them).
				continue
			}
		}
		w.blankLine()
	}
	return comments, nil
}

// groupableOneLiner reports whether rule is written on a single line, and so may
// be grouped with an adjacent rule instead of being followed by a blank line.
// These conditions must agree with the inline body branch of writeRule, which
// doesn't end the line after the closing brace of a multi-line body.
func (w *writer) groupableOneLiner(rule *ast.Rule) bool {
	// Location required to determine if two rules are adjacent in the policy.
	// If not, we respect line breaks between rules.
	if len(rule.Body) > 1 || rule.Default || rule.Location == nil {
		return false
	}

	// An else block is always written on a line of its own, so the rule spans
	// multiple lines even when its own body is written inline.
	if rule.Else != nil {
		return false
	}

	// A lone set term body keeps its enclosing braces, and so is written as a
	// multi-line block.
	if len(rule.Body) == 1 && isSetTerm(rule.Body[0]) {
		return false
	}

	partialSetException := w.fmtOpts.contains || rule.Head.Value != nil

	return (w.fmtOpts.regoV1 || w.fmtOpts.ifs) && partialSetException
}

func (w *writer) writeRule(rule *ast.Rule, isElse bool, comments []*ast.Comment) ([]*ast.Comment, error) {
	if rule == nil {
		return comments, nil
	}

	if !isElse {
		w.startLine()
	}

	if rule.Default {
		w.write("default ")
	}

	// OPA transforms lone bodies like `foo = {"a": "b"}` into rules of the form
	// `foo = {"a": "b"} { true }` in the AST. We want to preserve that notation
	// in the formatted code instead of expanding the bodies into rules, so we
	// pretend that the rule has no body in this case.
	isExpandedConst := rule.Body.Equal(expandedConst) && rule.Else == nil
	w.writeCommentOnFinalLine = isExpandedConst

	var err error
	var unexpectedComment bool
	comments, err = w.writeHead(rule.Head, rule.Default, isExpandedConst, comments)
	if err != nil {
		if errors.As(err, &unexpectedCommentError{}) {
			unexpectedComment = true
		} else {
			return nil, err
		}
	}

	if len(rule.Body) == 0 || isExpandedConst {
		w.endLine()
		return comments, nil
	}

	w.writeCommentOnFinalLine = true

	// this excludes partial sets UNLESS `contains` is used
	partialSetException := w.fmtOpts.contains || rule.Head.Value != nil

	usesIf := (w.fmtOpts.regoV1 || w.fmtOpts.ifs) && partialSetException

	if usesIf {
		w.write(" if")
		if len(rule.Body) == 1 {
			// Keep `if <term>` on one line when the single body term sits on the
			// same line as the end of the head. Comparing against the head's
			// start row would wrongly expand the condition into a block whenever
			// the head value spans multiple lines (e.g. a multi-line call).
			//
			// Additionally, a single set term must not be stripped of the outer body
			// braces, as that would semantically change the inner set to a body:
			// `p if { { x } }` -> p if { x }
			headEndRow, _ := location.EndOf(rule.Head.Location.Row, rule.Head.Location.Col, rule.Head.Location.Text)
			if rule.Body[0].Location.Row == headEndRow && !isSetTerm(rule.Body[0]) {
				w.write(" ")
				var err error
				comments, err = w.writeExpr(rule.Body[0], comments)
				if err != nil {
					return nil, err
				}
				w.endLine()
				if rule.Else != nil {
					comments, err = w.writeElse(rule, comments)
					if err != nil {
						return nil, err
					}
				}
				return comments, nil
			}
		}
	}
	if unexpectedComment && len(comments) > 0 {
		w.write(" { ")
	} else {
		w.write(" {")
		w.endLine()
	}

	// A leading set union renders as `x | y`, which the parser reads as a
	// comprehension at the brace of a `p if { ... }` body, so it is parenthesized.
	// An `else` body has no such ambiguity: its braces always open a body.
	if usesIf && !isElse {
		w.markUnionLead(rule.Body[0])
	}

	w.up()

	comments, err = w.writeBody(rule.Body, comments)
	if err != nil {
		// the unexpected comment error is passed up to be handled by writeHead
		if !errors.As(err, &unexpectedCommentError{}) {
			return nil, err
		}
	}

	var closeLoc *ast.Location

	if len(rule.Head.Args) > 0 {
		closeLoc = closingLoc('(', ')', '{', '}', rule.Location)
	} else if rule.Head.Key != nil {
		closeLoc = closingLoc('[', ']', '{', '}', rule.Location)
	} else {
		closeLoc = closingLoc(0, 0, '{', '}', rule.Location)
	}

	comments, err = w.insertComments(comments, closeLoc)
	if err != nil {
		return nil, err
	}

	if err := w.down(); err != nil {
		return nil, err
	}
	w.startLine()
	w.write("}")
	if rule.Else != nil {
		comments, err = w.writeElse(rule, comments)
		if err != nil {
			return nil, err
		}
	}
	return comments, nil
}

func (w *writer) writeElse(rule *ast.Rule, comments []*ast.Comment) ([]*ast.Comment, error) {
	// If there was nothing else on the line before the "else" starts
	// then preserve this style of else block, otherwise it will be
	// started as an "inline" else eg:
	//
	//     p {
	//     	...
	//     }
	//
	//     else {
	//     	...
	//     }
	//
	// versus
	//
	//     p {
	// 	    ...
	//     } else {
	//     	...
	//     }
	//
	// Note: This doesn't use the `close` as it currently isn't accurate for all
	// types of values. Checking the actual line text is the most consistent approach.
	wasInline := false
	ruleLines := bytes.Split(rule.Location.Text, []byte("\n"))
	relativeElseRow := rule.Else.Location.Row - rule.Location.Row
	if relativeElseRow > 0 && relativeElseRow < len(ruleLines) {
		elseLine := ruleLines[relativeElseRow]
		if !bytes.HasPrefix(bytes.TrimSpace(elseLine), []byte("else")) {
			wasInline = true
		}
	}

	// If there are any comments between the closing brace of the previous rule and the start
	// of the else block we will always insert a new blank line between them.
	hasCommentAbove := len(comments) > 0 && comments[0].Location.Row-rule.Else.Head.Location.Row < 0 || w.beforeEnd != nil

	if !hasCommentAbove && wasInline {
		w.write(" ")
	} else {
		w.blankLine()
		w.startLine()
	}

	rule.Else.Head.Name = "else" // NOTE(sr): whaaat

	elseHeadReference := ast.VarTerm("else")             // construct a reference for the term
	elseHeadReference.Location = rule.Else.Head.Location // and set the location to match the rule location

	rule.Else.Head.Reference = ast.Ref{elseHeadReference}
	rule.Else.Head.Args = nil
	var err error
	comments, err = w.insertComments(comments, rule.Else.Head.Location)
	if err != nil {
		return nil, err
	}

	if hasCommentAbove && !wasInline {
		// The comments would have ended the line, be sure to start one again
		// before writing the rest of the "else" rule.
		w.startLine()
	}

	// For backwards compatibility adjust the rule head value location
	// TODO: Refactor the logic for inserting comments, or special
	// case comments in a rule head value so this can be removed
	if rule.Else.Head.Value != nil {
		rule.Else.Head.Value.Location = rule.Else.Head.Location
	}

	return w.writeRule(rule.Else, true, comments)
}

func (w *writer) writeHead(head *ast.Head, isDefault bool, isExpandedConst bool, comments []*ast.Comment) ([]*ast.Comment, error) {
	ref := head.Ref()
	if head.Key != nil && head.Value == nil && !head.HasDynamicRef() {
		ref = ref.GroundPrefix()
	}
	if w.fmtOpts.refHeads || len(ref) == 1 {
		var err error
		comments, err = w.writeRef(ref, comments)
		if err != nil {
			return nil, err
		}
	} else {
		// if there are comments within the object in the rule head, don't format it
		if len(comments) > 0 && ref[1].Location.Row == comments[0].Location.Row {
			comments, err := w.writeUnformatted(head.Location, comments)
			if err != nil {
				return nil, err
			}
			return comments, nil
		}

		w.write(ref[0].String())
		w.write("[")
		w.write(ref[1].String())
		w.write("]")
	}

	if len(head.Args) > 0 {
		w.write("(")
		var args []any
		for _, arg := range head.Args {
			args = append(args, arg)
		}
		var err error
		comments, err = w.writeIterable(args, head.Location, closingLoc(0, 0, '(', ')', head.Location), comments, w.listWriter(false))
		w.write(")")
		if err != nil {
			return comments, err
		}
	}
	if head.Key != nil {
		if w.fmtOpts.contains && head.Value == nil {
			w.write(" contains ")
			var err error
			comments, err = w.writeTerm(head.Key, comments)
			if err != nil {
				return comments, err
			}
		} else if head.Value == nil { // no `if` for p[x] notation
			w.write("[")
			var err error
			comments, err = w.writeTerm(head.Key, comments)
			if err != nil {
				return comments, err
			}
			w.write("]")
		}
	}

	if head.Value != nil &&
		(head.Key != nil || !ast.InternedTerm(true).Equal(head.Value) || isExpandedConst || isDefault) {

		// in rego v1, explicitly print value for ref-head constants that aren't partial set assignments, e.g.:
		// * a -> parser error, won't reach here
		// * a.b -> a contains "b"
		// * a.b.c -> a.b.c := true
		// * a.b.c.d -> a.b.c.d := true
		isRegoV1RefConst := w.fmtOpts.regoV1 && isExpandedConst && head.Key == nil && len(head.Args) == 0

		if head.Location == head.Value.Location &&
			head.Name != "else" &&
			ast.InternedTerm(true).Equal(head.Value) &&
			!isRegoV1RefConst {
			// If the value location is the same as the location of the head,
			// we know that the value is generated, i.e. f(1)
			// Don't print the value (` = true`) as it is implied.
			return comments, nil
		}

		if head.Assign || w.fmtOpts.regoV1 {
			// preserve assignment operator, and enforce it if formatting for Rego v1
			w.write(" := ")
		} else {
			w.write(" = ")
		}
		var err error
		comments, err = w.writeTerm(head.Value, comments)
		if err != nil {
			return comments, err
		}
	}
	return comments, nil
}

func (w *writer) insertComments(comments []*ast.Comment, loc *ast.Location) ([]*ast.Comment, error) {
	before, at, comments := partitionComments(comments, loc)

	err := w.writeComments(before)
	if err != nil {
		return nil, err
	}
	if len(before) > 0 && loc.Row-before[len(before)-1].Location.Row > 1 {
		w.blankLine()
	}

	return comments, w.beforeLineEnd(at)
}

func (w *writer) writeBody(body ast.Body, comments []*ast.Comment) ([]*ast.Comment, error) {
	var err error
	comments, err = w.insertComments(comments, body.Loc())
	if err != nil {
		return comments, err
	}
	for i, expr := range body {
		// Insert a blank line in before the expression if it was not right
		// after the previous expression.
		if i > 0 {
			lastRow := body[i-1].Location.Row
			for _, c := range body[i-1].Location.Text {
				if c == '\n' {
					lastRow++
				}
			}
			if expr.Location.Row > lastRow+1 {
				w.blankLine()
			}
		}
		w.startLine()

		comments, err = w.writeExpr(expr, comments)
		if err != nil && !errors.As(err, &unexpectedCommentError{}) {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
		w.endLine()
	}
	return comments, nil
}

func (w *writer) writeExpr(expr *ast.Expr, comments []*ast.Comment) ([]*ast.Comment, error) {
	parenTerms := w.parenExpr == expr
	if parenTerms {
		w.parenExpr = nil
	}

	var err error
	comments, err = w.insertComments(comments, expr.Location)
	if err != nil {
		return comments, err
	}
	if !w.inline {
		w.startLine()
	}

	// `not` binds tighter than `and`/`or`, so a negated logical expression is
	// parenthesized. Only reachable through programmatically built ASTs; the
	// parser represents `not (a or b)` as an *ast.Not.
	negatedLogical := expr.Negated && isLogicalExpr(expr)

	if expr.Negated {
		w.write("not ")
		if negatedLogical {
			w.write("(")
		}
	}

	if parenTerms {
		w.write("(")
	}

	switch t := expr.Terms.(type) {
	case *ast.SomeDecl:
		comments, err = w.writeSomeDecl(t, comments)
		if err != nil {
			return nil, err
		}
	case *ast.Every:
		comments, err = w.writeEvery(t, expr.Loc(), comments)
		if err != nil {
			return nil, err
		}
	case *ast.Not:
		comments, err = w.writeNot(t, expr.Loc(), comments)
		if err != nil {
			return nil, err
		}
	case *ast.LogicalAnd, *ast.LogicalOr:
		comments, err = w.writeLogical(expr, comments)
		if err != nil {
			return nil, err
		}
	case []*ast.Term:
		comments, err = w.writeFunctionCall(expr, comments)
		if err != nil {
			return comments, err
		}
	case *ast.Term:
		comments, err = w.writeTerm(t, comments)
		if err != nil {
			return comments, err
		}
	}

	if parenTerms {
		w.write(")")
	}

	if negatedLogical {
		w.write(")")
	}

	if len(expr.With) == 0 {
		return comments, nil
	}

	withs := expr.With
	// Compare against the row where the expression's terms end (its
	// closing-bracket row), not where they begin. For a multi-line expression
	// the lone leading `with` sits on the closing-bracket line, which differs
	// from the start row, and comparing against the start would eject it onto
	// its own indented line (see issue #8804).
	lastRow := exprTermsEndRow(expr)

	// Print on same row if already there, otherwise increase indent a print remaining
	if withs[0].Location.Row == lastRow {
		if comments, err = w.writeWith(withs[0], comments, false); err != nil {
			return comments, err
		}
		lastRow, withs = withs[0].Location.Row, withs[1:]
	}

	if len(withs) > 0 {
		var indented bool

		for _, with := range withs {
			indent := with.Location.Row > lastRow
			if indent {
				if !indented {
					w.up()
					defer w.down() //nolint:errcheck
					indented = true
				}
				w.endLine()
				w.startLine()
				lastRow = with.Location.Row
			}
			if comments, err = w.writeWith(with, comments, indent); err != nil {
				return comments, err
			}
		}
	}

	return comments, nil
}

// exprTermsEndRow returns the row of the last source line occupied by the
// expression's own terms, ignoring any trailing `with` modifiers. For a
// single-line expression this equals expr.Location.Row; for a multi-line one
// (e.g. a wrapped function call ending in `)` or an `every` block ending in
// `}`) it is the closing-bracket row. expr.Location.Text spans the `with`
// clauses too, so they are trimmed off via the first `with`'s offset before
// the remaining term text is measured.
func exprTermsEndRow(expr *ast.Expr) int {
	loc := expr.Location
	if loc == nil {
		return 0
	}
	text := loc.Text
	if len(expr.With) > 0 && expr.With[0].Location != nil {
		if off := expr.With[0].Location.Offset - loc.Offset; off > 0 && off <= len(text) {
			text = text[:off]
		}
	}
	text = bytes.TrimRight(text, " \t\r\n")
	endRow, _ := location.EndOf(loc.Row, loc.Col, text)
	return endRow
}

// isSetTerm reports whether expr is a non-negated set term.
func isSetTerm(expr *ast.Expr) bool {
	if expr.IsNegated() {
		return false
	}

	term, ok := expr.Terms.(*ast.Term)
	if !ok {
		return false
	}

	_, ok = term.Value.(ast.Set)
	return ok
}

func (w *writer) writeSomeDecl(decl *ast.SomeDecl, comments []*ast.Comment) ([]*ast.Comment, error) {
	var err error
	comments, err = w.insertComments(comments, decl.Location)
	if err != nil {
		return nil, err
	}
	w.write("some ")

	row := decl.Location.Row

	for i, term := range decl.Symbols {
		switch val := term.Value.(type) {
		case ast.Var:
			if term.Location.Row > row {
				w.endLine()
				w.startLine()
				w.write(w.indent)
				row = term.Location.Row
			} else if i > 0 {
				w.write(" ")
			}

			comments, err = w.writeTerm(term, comments)
			if err != nil {
				return nil, err
			}

			if i < len(decl.Symbols)-1 {
				w.write(",")
			}
		case ast.Call:
			comments, err = w.writeInOperator(false, val[1:], comments, decl.Location, ast.BuiltinMap[val[0].String()].Decl)
			if err != nil {
				return nil, err
			}
		}
	}

	return comments, nil
}

func (w *writer) writeEvery(every *ast.Every, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	if loc == nil {
		loc = every.Loc()
	}

	var err error
	comments, err = w.insertComments(comments, loc)
	if err != nil {
		return nil, err
	}
	w.write("every ")
	if every.Key != nil {
		comments, err = w.writeTerm(every.Key, comments)
		if err != nil {
			return nil, err
		}
		w.write(", ")
	}
	comments, err = w.writeTerm(every.Value, comments)
	if err != nil {
		return nil, err
	}
	w.write(" in ")
	comments, err = w.writeTerm(every.Domain, comments)
	if err != nil {
		return nil, err
	}
	w.write(" {")
	comments, err = w.writeComprehensionBody('{', '}', every.Body, loc, loc, comments)
	if err != nil {
		// the unexpected comment error is passed up to be handled by writeHead
		if !errors.As(err, &unexpectedCommentError{}) {
			return nil, err
		}
	}

	if len(every.Body) == 1 &&
		every.Body[0].Location.Row == every.Location.Row {
		w.write(" ")
	}
	w.write("}")
	return comments, nil
}

func (w *writer) writeNot(not *ast.Not, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	if loc == nil {
		loc = not.Loc()
	}

	var err error
	comments, err = w.insertComments(comments, loc)
	if err != nil {
		return nil, err
	}

	w.write("not ")

	if not.ExplicitBody || len(not.Body) > 1 {
		// A leading set union renders as `x | y`, which the parser reads as a
		// comprehension at the brace, so it is parenthesized.
		if isUnionExpr(not.Body[0]) {
			w.parenExpr = not.Body[0]
		}

		w.write("{")
		comments, err = w.writeComprehensionBody('{', '}', not.Body, loc, loc, comments)
		if err != nil {
			if !errors.As(err, &unexpectedCommentError{}) {
				return nil, err
			}
		}

		if last := not.Body[len(not.Body)-1]; last.Location != nil && last.Location.Row == loc.Row {
			w.write(" ")
		}
		w.write("}")
	} else {
		parens := notBodyNeedsParens(not.Body[0])
		if parens {
			w.write("(")
		}

		comments, err = w.writeExpr(not.Body[0], comments)
		if err != nil {
			if !errors.As(err, &unexpectedCommentError{}) {
				return nil, err
			}
		}

		if parens {
			w.write(")")
		}
	}

	return comments, nil
}

// notBodyNeedsParens reports whether the sole expression of an implicit `not`
// body must be parenthesized to be read back as that same expression. Mirrors
// notBodyNeedsParens in the ast package.
func notBodyNeedsParens(expr *ast.Expr) bool {
	// A `with` on a bare operand of `not` binds to the whole `not` expression.
	if len(expr.With) > 0 {
		return true
	}

	// `not` binds tighter than `and`/`or`.
	if isLogicalExpr(expr) {
		return true
	}

	// `not not x` doesn't parse: a nested negation must be parenthesized to be
	// read back as a body.
	if _, ok := expr.Terms.(*ast.Not); ok {
		return true
	}

	// A value that renders brace-led would be re-read as an explicit body.
	return exprRendersBraceLead(expr)
}

// logicalOperand is one operand of an `and`/`or` chain.
type logicalOperand struct {
	body ast.Body

	// explicit is set for `{...}` operands, which scope their contents and are
	// always written braced.
	explicit bool

	// parens is set for implicit operands that must be parenthesized to be read
	// back as the same expression.
	parens bool

	// brace is the location of the operand's opening `{`, for explicit operands.
	brace *ast.Location
}

// logicalStep is one operator application of an `and`/`or` chain.
type logicalStep struct {
	op  string
	rhs logicalOperand

	// lhsEndRow is the row on which everything to the left of the operator ends.
	lhsEndRow int
}

// breaksLine reports whether the rhs operand is written on a line of its own,
// i.e. starts on a later row than the end of everything left of the operator.
// An explicit operand starts at its opening brace, an implicit one at its sole
// expression; a missing location leaves the row unknown, so no break.
func (s logicalStep) breaksLine() bool {
	var start *ast.Location
	if s.rhs.explicit {
		start = s.rhs.brace
	} else if len(s.rhs.body) > 0 {
		start = s.rhs.body[0].Location
	}

	return start != nil && start.Row > s.lhsEndRow
}

func (w *writer) writeLogical(expr *ast.Expr, comments []*ast.Comment) ([]*ast.Comment, error) {
	lhs, steps := flattenLogical(expr)

	comments, err := w.writeLogicalOperand(lhs, comments)
	if err != nil && !errors.As(err, &unexpectedCommentError{}) {
		return comments, err
	}

	var indented bool

	for _, s := range steps {
		w.write(" " + s.op)

		if s.breaksLine() {
			if !indented {
				w.up()
				defer w.down() //nolint:errcheck
				indented = true
			}
			w.endLine()
			w.startLine()
		} else {
			w.write(" ")
		}

		comments, err = w.writeLogicalOperand(s.rhs, comments)
		if err != nil && !errors.As(err, &unexpectedCommentError{}) {
			return comments, err
		}
	}

	return comments, nil
}

func (w *writer) writeLogicalOperand(o logicalOperand, comments []*ast.Comment) ([]*ast.Comment, error) {
	if !o.explicit {
		if o.parens {
			w.write("(")
			defer w.write(")")
		}

		return w.writeExpr(o.body[0], comments)
	}

	if len(o.body) == 0 {
		w.write("{}")
		return comments, nil
	}

	// A leading set union renders as `x | y`, which the parser reads as a
	// comprehension at the brace, so it is parenthesized.
	if isUnionExpr(o.body[0]) {
		w.parenExpr = o.body[0]
	}

	w.write("{")
	comments, err := w.writeComprehensionBody('{', '}', o.body, o.brace, o.brace, comments)
	if err != nil {
		if !errors.As(err, &unexpectedCommentError{}) {
			return comments, err
		}
	}

	if last := o.body[len(o.body)-1]; last.Location != nil && last.Location.Row == o.brace.Row {
		w.write(" ")
	}
	w.write("}")

	return comments, nil
}

// flattenLogical returns the leading operand and the operator applications of an
// `and`/`or` chain. Chains are left-associative, so the operands of
// `a and b and c` -- And{And{a, b}, c} -- are collected into a single chain,
// written with one level of continuation indent. A nested node that requires
// parens stays an operand of its own.
func flattenLogical(expr *ast.Expr) (logicalOperand, []logicalStep) {
	op, lhs, rhs, explicitLhs, explicitRhs := logicalParts(expr)

	step := logicalStep{
		op:  op,
		rhs: newLogicalOperand(rhs, explicitRhs, op, true, expr.Location),
	}

	if !explicitLhs && len(lhs) == 1 && isLogicalExpr(lhs[0]) && !logicalOperandNeedsParens(lhs[0], op, false) {
		step.lhsEndRow = bodyEndRow(lhs)
		first, steps := flattenLogical(lhs[0])

		return first, append(steps, step)
	}

	first := newLogicalOperand(lhs, explicitLhs, op, false, expr.Location)
	step.lhsEndRow = logicalOperandEndRow(first)

	return first, []logicalStep{step}
}

func logicalParts(expr *ast.Expr) (op string, lhs, rhs ast.Body, explicitLhs, explicitRhs bool) {
	switch t := expr.Terms.(type) {
	case *ast.LogicalAnd:
		return "and", t.Lhs, t.Rhs, t.ExplicitLhs, t.ExplicitRhs
	case *ast.LogicalOr:
		return "or", t.Lhs, t.Rhs, t.ExplicitLhs, t.ExplicitRhs
	}

	return "", nil, nil, false, false
}

func newLogicalOperand(b ast.Body, explicit bool, parentOp string, rhs bool, node *ast.Location) logicalOperand {
	if explicit || len(b) != 1 {
		return logicalOperand{body: b, explicit: true, brace: operandBraceLoc(node, b)}
	}

	return logicalOperand{body: b, parens: logicalOperandNeedsParens(b[0], parentOp, rhs)}
}

// logicalOperandNeedsParens reports whether an implicit operand of parentOp must
// be parenthesized to be read back as that same expression. Mirrors
// logicalOperandNeedsParens in the ast package.
func logicalOperandNeedsParens(expr *ast.Expr, parentOp string, rhs bool) bool {
	// A `with` on a bare operand binds to the whole and/or expression.
	if len(expr.With) > 0 {
		return true
	}

	switch expr.Terms.(type) {
	case *ast.LogicalOr:
		// `or` binds looser than `and`: always parenthesize under `and`; under
		// `or`, parenthesize only the rhs to preserve right-nesting.
		return parentOp == "and" || rhs
	case *ast.LogicalAnd:
		// `and` binds tighter: no parens under `or`; under `and`, parenthesize
		// only the rhs to preserve right-nesting.
		return parentOp == "and" && rhs
	}

	// A value that renders brace-led would be re-read as an explicit body.
	return exprRendersBraceLead(expr)
}

func logicalOperandEndRow(o logicalOperand) int {
	if o.explicit {
		if row := closingLoc(0, 0, '{', '}', o.brace).Row; row > 0 {
			return row
		}
	}

	return bodyEndRow(o.body)
}

// bodyEndRow returns the row of the last source line occupied by b.
func bodyEndRow(b ast.Body) int {
	if len(b) == 0 {
		return 0
	}

	loc := b[len(b)-1].Location
	if loc == nil {
		return 0
	}

	return loc.Row + bytes.Count(bytes.TrimRight(loc.Text, " \t\r\n"), []byte{'\n'})
}

// operandBraceLoc returns the location of the `{` opening an explicit operand
// body, derived from the location of the enclosing and/or node. The node
// location is returned as-is if the brace can't be located, e.g. for default
// locations.
func operandBraceLoc(node *ast.Location, b ast.Body) *ast.Location {
	if node == nil || len(b) == 0 || b[0].Location == nil {
		return node
	}

	i := min(b[0].Location.Offset-node.Offset, len(node.Text))

	for i--; i >= 0; i-- {
		if node.Text[i] != '{' {
			continue
		}

		cpy := *node
		cpy.Row = node.Row + bytes.Count(node.Text[:i], []byte{'\n'})
		cpy.Offset = node.Offset + i
		cpy.Text = node.Text[i:]

		return &cpy
	}

	return node
}

func isLogicalExpr(expr *ast.Expr) bool {
	return expr.IsAnd() || expr.IsOr()
}

func (w *writer) writeFunctionCall(expr *ast.Expr, comments []*ast.Comment) ([]*ast.Comment, error) {

	terms := expr.Terms.([]*ast.Term)
	operator := terms[0].Value.String()

	switch operator {
	case ast.Member.Name, ast.MemberWithKey.Name:
		return w.writeInOperator(false, terms[1:], comments, terms[0].Location, ast.BuiltinMap[terms[0].String()].Decl)
	}

	bi, ok := ast.BuiltinMap[operator]
	if !ok || bi.Infix == "" {
		return w.writeFunctionCallPlain(terms, comments)
	}

	numDeclArgs := bi.Decl.Arity()
	numCallArgs := len(terms) - 1

	var err error
	switch numCallArgs {
	case numDeclArgs: // Print infix where result is unassigned (e.g., x != y)
		comments, err = w.writeTerm(terms[1], comments)
		if err != nil {
			return nil, err
		}
		w.write(" " + bi.Infix + " ")
		return w.writeTerm(terms[2], comments)
	case numDeclArgs + 1: // Print infix where result is assigned (e.g., z = x + y)
		comments, err = w.writeTerm(terms[3], comments)
		if err != nil {
			return nil, err
		}
		w.write(" " + ast.Equality.Infix + " ")
		comments, err = w.writeTerm(terms[1], comments)
		if err != nil {
			return nil, err
		}
		w.write(" " + bi.Infix + " ")
		comments, err = w.writeTerm(terms[2], comments)
		if err != nil {
			return nil, err
		}
		return comments, nil
	}
	// NOTE(Trolloldem): in this point we are operating with a built-in function with the
	// wrong arity even when the assignment notation is used
	w.errs = append(w.errs, ArityFormatMismatchError(terms[1:], terms[0].String(), terms[0].Location, bi.Decl))
	return w.writeFunctionCallPlain(terms, comments)
}

func (w *writer) writeFunctionCallPlain(terms []*ast.Term, comments []*ast.Comment) ([]*ast.Comment, error) {
	if r, ok := terms[0].Value.(ast.Ref); ok {
		if c, err := w.writeRef(r, comments); err != nil {
			return c, err
		}
	} else {
		w.write(terms[0].String())
	}
	w.write("(")
	defer w.write(")")

	args := util.ToSliceOfAny(terms[1:])
	loc := terms[0].Location
	var err error
	comments, err = w.writeIterable(args, loc, closingLoc(0, 0, '(', ')', loc), comments, w.listWriter(false))
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func (w *writer) writeWith(with *ast.With, comments []*ast.Comment, indented bool) ([]*ast.Comment, error) {
	var err error
	comments, err = w.insertComments(comments, with.Location)
	if err != nil {
		return nil, err
	}
	if !indented {
		w.write(" ")
	}
	w.write("with ")
	comments, err = w.writeTerm(with.Target, comments)
	if err != nil {
		return nil, err
	}
	w.write(" as ")
	comments, err = w.writeTerm(with.Value, comments)
	if err != nil {
		// An unexpectedCommentError from writeTerm signals that it fell
		// back to writing the term's original unformatted text — the value
		// was written successfully, so don't abort the surrounding chain
		// of `with` clauses (issue #8765).
		if !errors.As(err, &unexpectedCommentError{}) {
			return comments, err
		}
	}
	return comments, nil
}

// saveComments saves a copy of the comments slice in a pooled slice to and returns it.
// This is to avoid having to create a new slice every time we need to save comments.
// The caller is responsible for putting the slice back in the pool when done.
func saveComments(comments []*ast.Comment) *[]*ast.Comment {
	cmlen := len(comments)
	saved := commentsSlicePool.Get(cmlen)

	copy(*saved, comments)

	return saved
}

func (w *writer) writeTerm(term *ast.Term, comments []*ast.Comment) ([]*ast.Comment, error) {
	if len(comments) == 0 {
		return w.writeTermParens(false, term, comments)
	}

	currentLen := w.buf.Len()
	currentLevel := w.level
	currentComments := saveComments(comments)
	defer commentsSlicePool.Put(currentComments)

	comments, err := w.writeTermParens(false, term, comments)
	if err != nil {
		if errors.As(err, &unexpectedCommentError{}) {
			w.buf.Truncate(currentLen)
			w.level = currentLevel

			// If beforeEnd refers to a comment within the source text range, clear it
			// This prevents the comment from being written twice
			if w.beforeEnd != nil && len(term.Location.Text) > 0 {
				endRow, _ := location.EndOf(term.Location.Row, term.Location.Col, term.Location.Text)
				if w.beforeEnd.Location.Row >= term.Location.Row && w.beforeEnd.Location.Row <= endRow {
					w.beforeEnd = nil
				}
			}

			comments, uErr := w.writeUnformatted(term.Location, *currentComments)
			if uErr != nil {
				return nil, uErr
			}
			return comments, err
		}
		return nil, err
	}

	return comments, nil
}

// writeUnformatted writes the unformatted text instead and updates the comment state
func (w *writer) writeUnformatted(location *ast.Location, currentComments []*ast.Comment) ([]*ast.Comment, error) {
	if len(location.Text) == 0 {
		return nil, errors.New("original unformatted text is empty")
	}

	rowNum := bytes.Count(location.Text, []byte{'\n'}) + 1

	w.writeBytes(location.Text)

	comments := make([]*ast.Comment, 0, len(currentComments))
	for _, c := range currentComments {
		// if there is a body then wait to write the last comment
		if w.writeCommentOnFinalLine && c.Location.Row == location.Row+rowNum-1 {
			w.write(" ")
			w.writeBytes(c.Location.Text)
			continue
		}

		// drop comments that occur within the rule raw text
		if c.Location.Row < location.Row+rowNum-1 {
			continue
		}
		comments = append(comments, c)
	}
	return comments, nil
}

func (w *writer) writeTermParens(parens bool, term *ast.Term, comments []*ast.Comment) ([]*ast.Comment, error) {
	if w.parenTerm == term {
		w.parenTerm = nil
		parens = true
	}

	var err error
	comments, err = w.insertComments(comments, term.Location)
	if err != nil {
		return nil, err
	}
	if !w.inline {
		w.startLine()
	}

	switch x := term.Value.(type) {
	case ast.Ref:
		comments, err = w.writeRef(x, comments)
		if err != nil {
			return nil, err
		}
	case ast.Object:
		comments, err = w.writeObject(x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case *ast.Array:
		comments, err = w.writeArray(x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case ast.Set:
		comments, err = w.writeSet(x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case *ast.ArrayComprehension:
		comments, err = w.writeArrayComprehension(x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case *ast.ObjectComprehension:
		comments, err = w.writeObjectComprehension(x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case *ast.SetComprehension:
		comments, err = w.writeSetComprehension(x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case ast.String:
		if term.Location.Text[0] == '`' {
			// To preserve raw strings, we need to output the original text,
			w.writeBytes(term.Location.Text)
		} else {
			// x.String() cannot be used by default because it can change the input string "\u0000" to "\x00"
			var after, quote []byte
			var found bool
			// term.Location.Text could contain the prefix `else :=`, remove it
			switch term.Location.Text[len(term.Location.Text)-1] {
			case '"':
				quote = []byte{'"'}
				_, after, found = bytes.Cut(term.Location.Text, quote)
			case '`':
				quote = []byte{'`'}
				_, after, found = bytes.Cut(term.Location.Text, quote)
			}

			if !found {
				// If no quoted string was found, that means it is a key being formatted to a string
				// e.g. partial_set.y to partial_set["y"]
				w.write(x.String())
			} else {
				w.writeBytes(quote)
				w.writeBytes(after)
			}

		}
	case *ast.TemplateString:
		comments, err = w.writeTemplateString(x, comments)
		if err != nil {
			return nil, err
		}
	case ast.Var:
		w.write(w.formatVar(x))
	case ast.Call:
		comments, err = w.writeCall(parens, x, term.Location, comments)
		if err != nil {
			return nil, err
		}
	case fmt.Stringer:
		w.write(x.String())
	}

	if !w.inline {
		w.startLine()
	}
	return comments, nil
}

func (w *writer) writeTemplateString(ts *ast.TemplateString, comments []*ast.Comment) ([]*ast.Comment, error) {
	w.write("$")
	if ts.MultiLine {
		w.write("`")
	} else {
		w.write(`"`)
	}

	for i, p := range ts.Parts {
		switch x := p.(type) {
		case *ast.Expr:
			w.write("{")
			w.up()

			if w.beforeEnd != nil {
				// We have a comment on the same line as the opening template-expression brace '{'
				w.endLine()
				w.startLine()
			} else {
				// We might have comments to write; the first of which should be on the same line as the opening template-expression brace '{'
				before, _, _ := partitionComments(comments, x.Location)
				if len(before) > 0 {
					w.write(" ")
					w.inline = true
					if err := w.writeComments(before); err != nil {
						return nil, err
					}

					comments = comments[len(before):]
				}
			}

			var err error
			comments, err = w.writeExpr(x, comments)
			if err != nil {
				return comments, err
			}

			// write trailing comments
			if i+1 < len(ts.Parts) {
				before, _, _ := partitionComments(comments, ts.Parts[i+1].Loc())
				if len(before) > 0 {
					w.endLine()
					if err := w.writeComments(before); err != nil {
						return nil, err
					}

					comments = comments[len(before):]
					w.startLine()
				}
			}

			w.write("}")

			if err := w.down(); err != nil {
				return nil, err
			}
		case *ast.Term:
			if s, ok := x.Value.(ast.String); ok {
				if ts.MultiLine {
					w.write(ast.EscapeTemplateStringStringPart(string(s)))
				} else {
					str := ast.EscapeTemplateStringStringPart(s.String())
					w.write(str[1 : len(str)-1])
				}
			} else {
				s := x.String()
				s = strings.TrimPrefix(s, "\"")
				s = strings.TrimSuffix(s, "\"")
				w.write(s)
			}
		default:
			w.write("<invalid>")
		}
	}

	if ts.MultiLine {
		w.write("`")
	} else {
		w.write(`"`)
	}

	return comments, nil
}

func (w *writer) writeRef(x ast.Ref, comments []*ast.Comment) ([]*ast.Comment, error) {
	if len(x) > 0 {
		parens := false
		_, ok := x[0].Value.(ast.Call)
		if ok {
			parens = x[0].Location.Text[0] == 40 // Starts with "("
		}
		var err error
		comments, err = w.writeTermParens(parens, x[0], comments)
		if err != nil {
			return nil, err
		}
		path := x[1:]
		for _, t := range path {
			switch p := t.Value.(type) {
			case ast.String:
				w.writeRefStringPath(p, t.Location)
			case ast.Var:
				w.writeBracketed(w.formatVar(p))
			default:
				w.write("[")
				comments, err = w.writeTerm(t, comments)
				if err != nil {
					if errors.As(err, &unexpectedCommentError{}) {
						// add a new line so that the closing bracket isn't part of the unexpected comment
						w.write("\n")
					} else {
						return nil, err
					}
				}
				w.write("]")
			}
		}
	}

	return comments, nil
}

func (w *writer) writeBracketed(str string) {
	w.write("[" + str + "]")
}

func (w *writer) writeRefStringPath(s ast.String, l *ast.Location) {
	str := string(s)
	if w.shouldBracketRefTerm(str, l) {
		w.writeBracketed(s.String())
	} else {
		w.write("." + str)
	}
}

func (w *writer) shouldBracketRefTerm(s string, l *ast.Location) bool {
	if !ast.IsVarCompatibleString(s) {
		return true
	}

	if ast.IsInKeywords(s, w.fmtOpts.keywords()) {
		if !w.fmtOpts.allowKeywordsInRefs {
			return true
		}

		if l != nil && l.Text[0] == 34 { // If the original term text starts with '"', we preserve the brackets and quotes
			return true
		}
	}

	return false
}

func (*writer) formatVar(v ast.Var) string {
	if v.IsWildcard() {
		return ast.Wildcard.String()
	}
	return v.String()
}

func (w *writer) writeCall(parens bool, x ast.Call, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	bi, ok := ast.BuiltinMap[x[0].String()]
	if !ok || bi.Infix == "" {
		return w.writeFunctionCallPlain(x, comments)
	}

	if bi.Infix == "in" {
		// NOTE(sr): `in` requires special handling, mirroring what happens in the parser,
		// since there can be one or two lhs arguments.
		return w.writeInOperator(true, x[1:], comments, loc, bi.Decl)
	}

	// TODO(tsandall): improve to consider precedence?
	if parens {
		w.write("(")
	}

	// NOTE(Trolloldem): writeCall is only invoked when the function call is a term
	// of another function. The only valid arity is the one of the
	// built-in function
	if bi.Decl.Arity() != len(x)-1 {
		w.errs = append(w.errs, ArityFormatMismatchError(x[1:], x[0].String(), loc, bi.Decl))
		return comments, nil
	}

	var err error
	comments, err = w.writeTermParens(true, x[1], comments)
	if err != nil {
		return nil, err
	}
	w.write(" " + bi.Infix + " ")
	comments, err = w.writeTermParens(true, x[2], comments)
	if err != nil {
		return nil, err
	}
	if parens {
		w.write(")")
	}

	return comments, nil
}

func (w *writer) writeInOperator(parens bool, operands []*ast.Term, comments []*ast.Comment, loc *ast.Location, f *types.Function) ([]*ast.Comment, error) {

	if len(operands) != f.Arity() {
		// The number of operands does not math the arity of the `in` operator
		operator := ast.Member.Name
		if f.Arity() == 3 {
			operator = ast.MemberWithKey.Name
		}
		w.errs = append(w.errs, ArityFormatMismatchError(operands, operator, loc, f))
		return comments, nil
	}
	kw := "in"
	var err error
	switch len(operands) {
	case 2:
		comments, err = w.writeTermParens(true, operands[0], comments)
		if err != nil {
			return nil, err
		}
		w.write(" ")
		w.write(kw)
		w.write(" ")
		comments, err = w.writeTermParens(true, operands[1], comments)
		if err != nil {
			return nil, err
		}
	case 3:
		if parens {
			w.write("(")
			defer w.write(")")
		}
		comments, err = w.writeTermParens(true, operands[0], comments)
		if err != nil {
			return nil, err
		}
		w.write(", ")
		comments, err = w.writeTermParens(true, operands[1], comments)
		if err != nil {
			return nil, err
		}
		w.write(" ")
		w.write(kw)
		w.write(" ")
		comments, err = w.writeTermParens(true, operands[2], comments)
		if err != nil {
			return nil, err
		}
	}
	return comments, nil
}

func (w *writer) writeObject(obj ast.Object, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	w.write("{")
	defer w.write("}")

	var s []any
	obj.Foreach(func(k, v *ast.Term) {
		s = append(s, ast.Item(k, v))
	})
	return w.writeIterable(s, loc, closingLoc(0, 0, '{', '}', loc), comments, w.objectWriter())
}

func (w *writer) writeArray(arr *ast.Array, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	w.write("[")
	defer w.write("]")

	var s []any
	arr.Foreach(func(t *ast.Term) {
		s = append(s, t)
	})
	var err error
	comments, err = w.writeIterable(s, loc, closingLoc(0, 0, '[', ']', loc), comments, w.listWriter(true))
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func (w *writer) writeSet(set ast.Set, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {

	if set.Len() == 0 {
		w.write("set()")
		var err error
		comments, err = w.insertComments(comments, closingLoc(0, 0, '(', ')', loc))
		if err != nil {
			return nil, err
		}
		return comments, nil
	}

	w.write("{")
	defer w.write("}")

	var s []any
	set.Foreach(func(t *ast.Term) {
		s = append(s, t)
	})
	var err error
	comments, err = w.writeIterable(s, loc, closingLoc(0, 0, '{', '}', loc), comments, w.listWriter(true))
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func (w *writer) writeArrayComprehension(arr *ast.ArrayComprehension, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	w.write("[")
	defer w.write("]")

	return w.writeComprehension('[', ']', arr.Term, arr.Body, loc, comments)
}

func (w *writer) writeSetComprehension(set *ast.SetComprehension, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	w.write("{")
	defer w.write("}")

	return w.writeComprehension('{', '}', set.Term, set.Body, loc, comments)
}

func (w *writer) writeObjectComprehension(object *ast.ObjectComprehension, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	w.write("{")
	defer w.write("}")

	// Ensure the value is not written on the next line. writeComprehension
	// breaks before the term whenever the term's row is below the row the
	// comprehension opened on, so the value is given a location on that row
	// rather than its own, which may already be a row further down. Copying
	// the value's own location rather than the key's keeps Text intact, which
	// writeComprehension reads to decide whether a call term was parenthesised.
	valueLoc := *object.Value.Location
	valueLoc.Row = loc.Row
	object.Value.Location = &valueLoc

	paren := isUnionCall(object.Key)
	if paren {
		w.write("(")
	}

	var err error
	comments, err = w.writeTerm(object.Key, comments)
	if err != nil {
		return nil, err
	}
	if paren {
		w.write(")")
	}

	w.write(": ")
	return w.writeComprehension('{', '}', object.Value, object.Body, loc, comments)
}

func (w *writer) writeComprehension(openChar, closeChar byte, term *ast.Term, body ast.Body, loc *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	if term.Location.Row-loc.Row >= 1 {
		w.endLine()
		w.startLine()
	}

	parens := false
	if _, ok := term.Value.(ast.Call); ok {
		parens = isUnionCall(term) || term.Location.Text[0] == 40 // Starts with "("
	}
	var err error
	comments, err = w.writeTermParens(parens, term, comments)
	if err != nil {
		return nil, err
	}
	w.write(" |")

	return w.writeComprehensionBody(openChar, closeChar, body, term.Location, loc, comments)
}

func (w *writer) writeComprehensionBody(openChar, closeChar byte, body ast.Body, term, compr *ast.Location, comments []*ast.Comment) ([]*ast.Comment, error) {
	lines, err := w.groupIterable(util.ToSliceOfAny(body), term)
	if err != nil {
		return nil, err
	}

	if body.Loc().Row-term.Row > 0 || len(lines) > 1 {
		w.endLine()
		w.up()
		defer w.startLine()
		defer func() {
			if err := w.down(); err != nil {
				w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
			}
		}()

		var err error
		comments, err = w.writeBody(body, comments)
		if err != nil {
			return comments, err
		}
	} else {
		w.write(" ")
		i := 0
		for ; i < len(body)-1; i++ {
			comments, err = w.writeExpr(body[i], comments)
			if err != nil {
				return comments, err
			}
			w.write("; ")
		}
		comments, err = w.writeExpr(body[i], comments)
		if err != nil {
			return comments, err
		}
	}
	comments, err = w.insertComments(comments, closingLoc(0, 0, openChar, closeChar, compr))
	if err != nil {
		return nil, err
	}
	return comments, nil
}

func (w *writer) writeImports(imports []*ast.Import, comments []*ast.Comment) ([]*ast.Comment, error) {
	m, comments := mapImportsToComments(imports, comments)

	groups := groupImports(imports)
	for _, group := range groups {
		var err error
		comments, err = w.insertComments(comments, group[0].Loc())
		if err != nil {
			return nil, err
		}

		// Sort imports within a newline grouping.
		slices.SortFunc(group, (*ast.Import).Compare)
		for _, i := range group {
			w.startLine()
			err = w.writeImport(i)
			if err != nil {
				return nil, err
			}
			if c, ok := m[i]; ok {
				w.write(" " + c.String())
			}
			w.endLine()
		}
		w.blankLine()
	}

	return comments, nil
}

func (w *writer) writeImport(imp *ast.Import) error {
	path := imp.Path.Value.(ast.Ref)

	w.write("import ")

	if _, ok := future.WhichFutureKeyword(imp); ok {
		// We don't want to wrap future.keywords imports in parens, so we create a new writer that doesn't
		w2 := writer{
			buf: bytes.Buffer{},
			fmtOpts: fmtOpts{
				allowKeywordsInRefs: true,
			},
		}
		_, err := w2.writeRef(path, nil)
		if err != nil {
			return err
		}
		w.write(w2.buf.String())
	} else {
		_, err := w.writeRef(path, nil)
		if err != nil {
			return err
		}
	}

	if len(imp.Alias) > 0 {
		w.write(" as " + imp.Alias.String())
	}

	return nil
}

type entryWriter func(any, []*ast.Comment) ([]*ast.Comment, error)

func (w *writer) writeIterable(elements []any, last *ast.Location, close *ast.Location, comments []*ast.Comment, fn entryWriter) ([]*ast.Comment, error) {
	lines, err := w.groupIterable(elements, last)
	if err != nil {
		return nil, err
	}

	newlinePrecedesItem := false
	// If there are comments within the single line, don't collapse it and keep it as-is
	// Return an error so that writeTerm will write the original formatting
	if len(lines) == 1 {
		for _, c := range comments {
			if c.Location.Row > last.Row && c.Location.Row < close.Row {
				return comments, unexpectedCommentError{
					newComment:    truncatedString(c.String(), 100),
					newCommentRow: c.Location.Row,
				}
			}
		}
		if len(elements) > 0 {
			var first *ast.Term
			if term, ok := elements[0].(*ast.Term); ok {
				first = term
			} else if pair, ok := elements[0].([2]*ast.Term); ok {
				first = pair[0]
			}
			cut := bytes.Index(last.Text, first.Location.Text)
			if cut > 0 {
				txt := last.Text[:cut]
				newlinePrecedesItem = bytes.IndexByte(txt, '\n') > 0
			}
		}
	}

	isMultiline := len(lines) > 1 || (len(lines) == 1 && newlinePrecedesItem)

	if isMultiline {
		w.delayBeforeEnd()
		w.startMultilineSeq()
	}

	i := 0
	for ; i < len(lines)-1; i++ {
		comments, err = w.writeIterableLine(lines[i], comments, fn)
		if err != nil {
			return nil, err
		}
		w.write(",")

		w.endLine()
		w.startLine()
	}

	comments, err = w.writeIterableLine(lines[i], comments, fn)
	if err != nil {
		return nil, err
	}

	if isMultiline {
		w.write(",")
		w.endLine()
		comments, err = w.insertComments(comments, close)
		if err != nil {
			return nil, err
		}
		if err := w.down(); err != nil {
			return nil, err
		}
		w.startLine()
	}

	return comments, nil
}

func (w *writer) writeIterableLine(elements []any, comments []*ast.Comment, fn entryWriter) ([]*ast.Comment, error) {
	if len(elements) == 0 {
		return comments, nil
	}

	i := 0
	for ; i < len(elements)-1; i++ {
		var err error
		comments, err = fn(elements[i], comments)
		if err != nil {
			return nil, err
		}
		w.write(", ")
	}

	return fn(elements[i], comments)
}

// isUnionExpr reports whether expr is a set-union call that renders as a bare
// `x | y`, which is comprehension syntax at an operand brace.
func isUnionExpr(expr *ast.Expr) bool {
	terms, ok := expr.Terms.([]*ast.Term)
	return ok && len(terms) == 3 && ast.Interned.Refs.Or.Equal(terms[0].Value)
}

// markUnionLead parenthesizes the set union leading the rendering of expr, if
// there is one: a leading `x | y` reads as comprehension syntax at the brace of
// the body holding expr. The union is either the expression itself, or the
// leading operand of an infix call — one nested deeper is already parenthesized
// by writeCall.
func (w *writer) markUnionLead(expr *ast.Expr) {
	if expr.Negated {
		return
	}

	if isLogicalExpr(expr) {
		if lhs, _ := flattenLogical(expr); !lhs.explicit && !lhs.parens {
			w.markUnionLead(lhs.body[0])
		}

		return
	}

	if isUnionExpr(expr) {
		w.parenExpr = expr
		return
	}

	terms, ok := expr.Terms.([]*ast.Term)
	if !ok {
		return
	}

	// Infix calls render an operand first: the result for the assigned form
	// (`z = x | y`), otherwise the lhs (`x | y == z`).
	if bi, ok := ast.BuiltinMap[terms[0].Value.String()]; ok && bi.Infix != "" {
		var lead *ast.Term

		switch len(terms) {
		case bi.Decl.Arity() + 1:
			lead = terms[1]
		case bi.Decl.Arity() + 2:
			lead = terms[len(terms)-1]
		}

		if lead != nil && isUnionCall(lead) {
			w.parenTerm = lead
		}
	}
}

// exprRendersBraceLead reports whether expr renders starting with a `{`. Such an
// expression needs parens in an operand position, as bare braces there are read as
// an explicit body. Mirrors rendersWithLeadingBrace in the ast package.
func exprRendersBraceLead(expr *ast.Expr) bool {
	switch t := expr.Terms.(type) {
	case *ast.Term:
		return termRendersBraceLead(t)
	case []*ast.Term:
		// Infix calls render an operand first: the result for the assigned form
		// (`z = x | y`), otherwise the lhs (`{x} == y`).
		if bi, ok := ast.BuiltinMap[t[0].Value.String()]; ok && bi.Infix != "" {
			switch len(t) {
			case bi.Decl.Arity() + 1:
				return termRendersBraceLead(t[1])
			case bi.Decl.Arity() + 2:
				return termRendersBraceLead(t[len(t)-1])
			}
		}
	}

	return false
}

func termRendersBraceLead(t *ast.Term) bool {
	switch v := t.Value.(type) {
	case ast.Set:
		// The empty set renders as `set()`.
		return v.Len() > 0
	case ast.Object, *ast.SetComprehension, *ast.ObjectComprehension:
		return true
	case ast.Ref:
		return len(v) > 0 && termRendersBraceLead(v[0])
	case ast.Call:
		// An infix call renders an operand first, so a brace-led operand of a
		// nested call leads the whole rendering: `{1, 2} & s == set()`.
		if bi, ok := ast.BuiltinMap[v[0].Value.String()]; ok && bi.Infix != "" &&
			len(v) == bi.Decl.Arity()+1 {
			return termRendersBraceLead(v[1])
		}
	}

	return false
}

// isUnionCall returns true if the term is a call to the union built-in, whose
// infix form (`|`) is comprehension syntax when used inside a collection
// literal or as a comprehension term, and must be parenthesized there.
func isUnionCall(t *ast.Term) bool {
	call, ok := t.Value.(ast.Call)
	return ok && ast.Interned.Refs.Or.Equal(call[0].Value)
}

func (w *writer) objectWriter() entryWriter {
	return func(x any, comments []*ast.Comment) ([]*ast.Comment, error) {
		entry := x.([2]*ast.Term)

		paren := isUnionCall(entry[0])
		if paren {
			w.write("(")
		}

		var err error
		comments, err = w.writeTerm(entry[0], comments)
		if err != nil {
			return nil, err
		}
		if paren {
			w.write(")")
		}

		w.write(": ")

		if isUnionCall(entry[1]) {
			w.write("(")
			defer w.write(")")
		}

		return w.writeTerm(entry[1], comments)
	}
}

func (w *writer) listWriter(parenUnionCalls bool) entryWriter {
	return func(x any, comments []*ast.Comment) ([]*ast.Comment, error) {
		t, ok := x.(*ast.Term)
		if ok && isUnionCall(t) {
			if parenUnionCalls || t.Location.Text[0] == 40 { // Starts with "("
				w.write("(")
				defer w.write(")")
			}
		}

		return w.writeTerm(t, comments)
	}
}

// groupIterable will group the `elements` slice into slices according to their
// location: anything on the same line will be put into a slice.
func (w *writer) groupIterable(elements []any, last *ast.Location) ([][]any, error) {
	// Generated vars occur in the AST when we're rendering the result of
	// partial evaluation in a bundle build with optimization.
	// Those variables, and wildcard variables have the "default location",
	// set in `Ast()`). That is no proper file location, and the grouping
	// based on source location will yield a bad result.
	// Another case is generated variables: they do have proper file locations,
	// but their row/col information may no longer match their AST location.
	// So, for generated variables, we also don't trust the location, but
	// keep them ungrouped.
	def := false // default location found?
	for _, elem := range elements {
		ast.WalkTerms(elem, func(t *ast.Term) bool {
			if t.Location.File == defaultLocationFile {
				def = true
				return true
			}
			return false
		})
		ast.WalkVars(elem, func(v ast.Var) bool {
			if v.IsGenerated() {
				def = true
				return true
			}
			return false
		})
		if def { // return as-is
			return [][]any{elements}, nil
		}
	}

	slices.SortFunc(elements, func(i, j any) int {
		l, err := locCmpOrError(i, j)
		if err != nil {
			w.errs = append(w.errs, ast.NewError(ast.FormatErr, &ast.Location{}, "%s", err.Error()))
		}
		return l
	})

	var lines [][]any
	cur := make([]any, 0, len(elements))
	for i, t := range elements {
		elem := t
		loc, err := getLoc(elem)
		if err != nil {
			return nil, err
		}
		lineDiff := loc.Row - last.Row
		if lineDiff > 0 && i > 0 {
			lines = append(lines, cur)
			cur = nil
		}

		last = loc
		cur = append(cur, elem)
	}
	return append(lines, cur), nil
}

func mapImportsToComments(imports []*ast.Import, comments []*ast.Comment) (map[*ast.Import]*ast.Comment, []*ast.Comment) {
	var leftovers []*ast.Comment
	m := map[*ast.Import]*ast.Comment{}

	for _, c := range comments {
		matched := false
		for _, i := range imports {
			if c.Loc().Row == i.Loc().Row {
				m[i] = c
				matched = true
				break
			}
		}
		if !matched {
			leftovers = append(leftovers, c)
		}
	}

	return m, leftovers
}

func groupImports(imports []*ast.Import) [][]*ast.Import {
	switch len(imports) { // shortcuts
	case 0:
		return nil
	case 1:
		return [][]*ast.Import{imports}
	}
	// there are >=2 imports to group

	var groups [][]*ast.Import
	group := []*ast.Import{imports[0]}

	for _, i := range imports[1:] {
		last := group[len(group)-1]

		// nil-location imports have been sorted up to come first
		if i.Loc() != nil && last.Loc() != nil && // first import with a location, or
			i.Loc().Row-last.Loc().Row > 1 { // more than one row apart from previous import

			// start a new group
			groups = append(groups, group)
			group = []*ast.Import{}
		}
		group = append(group, i)
	}
	if len(group) > 0 {
		groups = append(groups, group)
	}

	return groups
}

func partitionComments(comments []*ast.Comment, l *ast.Location) ([]*ast.Comment, *ast.Comment, []*ast.Comment) {
	if len(comments) == 0 {
		return nil, nil, nil
	}

	numBefore, numAfter := 0, 0
	for _, c := range comments {
		switch cmp := c.Location.Row - l.Row; {
		case cmp < 0:
			numBefore++
		case cmp > 0:
			numAfter++
		}
	}

	if numAfter == len(comments) {
		return nil, nil, comments
	}

	var at *ast.Comment

	before := make([]*ast.Comment, 0, numBefore)
	after := make([]*ast.Comment, 0, numAfter)

	for _, c := range comments {
		switch cmp := c.Location.Row - l.Row; {
		case cmp < 0:
			before = append(before, c)
		case cmp > 0:
			after = append(after, c)
		default:
			at = c
		}
	}

	return before, at, after
}

func gatherImports(others []any) (imports []*ast.Import, rest []any) {
	i := 0
loop:
	for ; i < len(others); i++ {
		switch x := others[i].(type) {
		case *ast.Import:
			imports = append(imports, x)
		case *ast.Rule:
			break loop
		}
	}
	return imports, others[i:]
}

func gatherRules(others []any) (rules []*ast.Rule, rest []any) {
	i := 0
loop:
	for ; i < len(others); i++ {
		switch x := others[i].(type) {
		case *ast.Rule:
			rules = append(rules, x)
		case *ast.Import:
			break loop
		}
	}
	return rules, others[i:]
}

func locCmpOrError(a, b any) (int, error) {
	al, bl, err := getLocs(a, b)
	if err != nil {
		return 0, err
	}
	return locCmp(al, bl), nil
}

func locCmp(a, b *ast.Location) int {
	switch {
	case a == b:
		return 0
	case a == nil:
		return -1
	case b == nil:
		return 1
	}
	if cmp := a.Row - b.Row; cmp != 0 {
		return cmp
	}
	return a.Col - b.Col
}

func getLoc(x any) (*ast.Location, error) {
	switch x := x.(type) {
	case ast.Node: // *ast.Head, *ast.Expr, *ast.With, *ast.Term
		return x.Loc(), nil
	case *ast.Location:
		return x, nil
	case [2]*ast.Term: // Special case to allow for easy printing of objects.
		return x[0].Location, nil
	default:
		return nil, fmt.Errorf("unable to get location for type %v", x)
	}
}

func getLocs(a, b any) (*ast.Location, *ast.Location, error) {
	al, err1 := getLoc(a)
	bl, err2 := getLoc(b)
	return al, bl, errors.Join(err1, err2)
}

var negativeRow = &ast.Location{Row: -1}

func closingLoc(skipOpen, skipClose, openChar, closeChar byte, loc *ast.Location) *ast.Location {
	i, offset := 0, 0

	// Skip past parens/brackets/braces in rule heads.
	if skipOpen > 0 {
		i, offset = skipPast(skipOpen, skipClose, loc)
	}

	for ; i < len(loc.Text); i++ {
		if loc.Text[i] == openChar {
			break
		}
	}

	if i >= len(loc.Text) {
		return negativeRow
	}

	state := 1
	for state > 0 {
		i++
		if i >= len(loc.Text) {
			return negativeRow
		}

		switch loc.Text[i] {
		case openChar:
			state++
		case closeChar:
			state--
		case '\n':
			offset++
		}
	}

	return &ast.Location{Row: loc.Row + offset}
}

func skipPast(openChar, closeChar byte, loc *ast.Location) (int, int) {
	i := 0
	for ; i < len(loc.Text); i++ {
		if loc.Text[i] == openChar {
			break
		}
	}

	state := 1
	offset := 0
	for state > 0 {
		i++
		if i >= len(loc.Text) {
			return i, offset
		}

		switch loc.Text[i] {
		case openChar:
			state++
		case closeChar:
			state--
		case '\n':
			offset++
		}
	}

	return i, offset
}

// startLine begins a line with the current indentation level.
func (w *writer) startLine() {
	w.inline = true
	for range w.level {
		w.write(w.indent)
	}
}

// endLine ends a line with a newline.
func (w *writer) endLine() {
	w.inline = false
	if w.beforeEnd != nil && !w.delay {
		w.write(" " + w.beforeEnd.String())
		w.beforeEnd = nil
	}
	w.delay = false
	w.write("\n")
}

type unexpectedCommentError struct {
	newComment         string
	newCommentRow      int
	existingComment    string
	existingCommentRow int
}

func (u unexpectedCommentError) Error() string {
	return fmt.Sprintf("unexpected new comment (%s) on line %d because there is already a comment (%s) registered for line %d",
		u.newComment, u.newCommentRow, u.existingComment, u.existingCommentRow)
}

// beforeLineEnd registers a comment to be printed at the end of the current line.
func (w *writer) beforeLineEnd(c *ast.Comment) error {
	if w.beforeEnd != nil {
		if c == nil {
			return nil
		}

		existingComment := truncatedString(w.beforeEnd.String(), 100)
		existingCommentRow := w.beforeEnd.Location.Row
		newComment := truncatedString(c.String(), 100)
		w.beforeEnd = nil

		return unexpectedCommentError{
			newComment:         newComment,
			newCommentRow:      c.Location.Row,
			existingComment:    existingComment,
			existingCommentRow: existingCommentRow,
		}
	}
	w.beforeEnd = c
	return nil
}

func truncatedString(s string, max int) string {
	if len(s) > max {
		return s[:max-2] + "..."
	}
	return s
}

func (w *writer) delayBeforeEnd() {
	w.delay = true
}

// line prints a blank line. If the writer is currently in the middle of a line,
// line ends it and then prints a blank one.
func (w *writer) blankLine() {
	if w.inline {
		w.endLine()
	}
	w.write("\n")
}

// write writes string s to the buffer.
func (w *writer) write(s string) {
	w.buf.WriteString(s)
}

// writeBytes writes []byte b to the buffer.
func (w *writer) writeBytes(b []byte) {
	w.buf.Write(b)
}

// writeLine writes the string on a newly started line, then terminate the line.
func (w *writer) writeLine(s string) {
	if !w.inline {
		w.startLine()
	}
	w.write(s)
	w.endLine()
}

func (w *writer) startMultilineSeq() {
	w.endLine()
	w.up()
	w.startLine()
}

// up increases the indentation level
func (w *writer) up() {
	w.level++
}

// down decreases the indentation level
func (w *writer) down() error {
	if w.level == 0 {
		return errors.New("negative indentation level")
	}
	w.level--
	return nil
}

func ensureFutureKeywordImport(imps []*ast.Import, kw string) []*ast.Import {
	for _, imp := range imps {
		if future.IsAllFutureKeywords(imp) ||
			future.IsFutureKeyword(imp, kw) ||
			(future.IsFutureKeyword(imp, "every") && kw == "in") { // "every" implies "in", so we don't need to add both
			return imps
		}
	}
	imp := &ast.Import{
		Path: ast.MustParseTerm("future.keywords." + kw),
	}
	imp.Location = nextImportLoc(imps, imp)
	return append(imps, imp)
}

func nextImportLoc(imps []*ast.Import, node ast.Node) *ast.Location {
	maxRow := 0
	for _, imp := range imps {
		if imp.Loc() == nil {
			continue
		}
		if isFutureKeywordsImport(imp) || isRegoV1Compatible(imp) {
			if imp.Loc().Row > maxRow {
				maxRow = imp.Loc().Row
			}
		}
	}
	if maxRow == 0 {
		return defaultLocation(node)
	}
	return ast.NewLocation([]byte(node.String()), defaultLocationFile, maxRow+1, 1)
}

func isFutureKeywordsImport(imp *ast.Import) bool {
	path := imp.Path.Value.(ast.Ref)
	return len(path) >= 2 && ast.FutureRootDocument.Equal(path[0])
}

func isAddedImport(imp *ast.Import) bool {
	return imp.Loc() != nil && imp.Loc().File == defaultLocationFile
}

// addedImportFollowsRule reports whether an import added by the formatter would
// be written at or after the first rule.
func addedImportFollowsRule(others []any) bool {
	firstRule := -1
	for _, x := range others {
		if r, ok := x.(*ast.Rule); ok && r.Loc() != nil {
			if firstRule < 0 || r.Loc().Row < firstRule {
				firstRule = r.Loc().Row
			}
		}
	}
	if firstRule < 0 {
		return false
	}
	for _, x := range others {
		if imp, ok := x.(*ast.Import); ok && isAddedImport(imp) && imp.Loc().Row >= firstRule {
			return true
		}
	}
	return false
}

func ensureRegoV1Import(imps []*ast.Import) []*ast.Import {
	return ensureImport(imps, ast.RegoV1CompatibleRef)
}

func filterRegoV1Import(imps []*ast.Import) []*ast.Import {
	var ret []*ast.Import
	for _, imp := range imps {
		path := imp.Path.Value.(ast.Ref)
		if !ast.RegoV1CompatibleRef.Equal(path) {
			ret = append(ret, imp)
		}
	}
	return ret
}

func ensureImport(imps []*ast.Import, path ast.Ref) []*ast.Import {
	for _, imp := range imps {
		p := imp.Path.Value.(ast.Ref)
		if p.Equal(path) {
			return imps
		}
	}
	imp := &ast.Import{
		Path: ast.NewTerm(path),
	}
	imp.Location = nextImportLoc(imps, imp)
	return append(imps, imp)
}

// ArityFormatErrDetail but for `fmt` checks since compiler has not run yet.
type ArityFormatErrDetail struct {
	Have []string `json:"have"`
	Want []string `json:"want"`
}

// ArityFormatMismatchError but for `fmt` checks since the compiler has not run yet.
func ArityFormatMismatchError(operands []*ast.Term, operator string, loc *ast.Location, f *types.Function) *ast.Error {
	want := make([]string, f.Arity())
	for i, arg := range f.FuncArgs().Args {
		want[i] = types.Sprint(arg)
	}

	have := make([]string, len(operands))
	for i := range operands {
		have[i] = ast.ValueName(operands[i].Value)
	}
	err := ast.NewError(ast.TypeErr, loc, "%s: %s", operator, "arity mismatch")
	err.Details = &ArityFormatErrDetail{
		Have: have,
		Want: want,
	}
	return err
}

// Lines returns the string representation of the detail.
func (d *ArityFormatErrDetail) Lines() []string {
	return []string{
		"have: (" + strings.Join(d.Have, ",") + ")",
		"want: (" + strings.Join(d.Want, ",") + ")",
	}
}

// isRegoV1Compatible returns true if the passed *ast.Import is `rego.v1`
func isRegoV1Compatible(imp *ast.Import) bool {
	path := imp.Path.Value.(ast.Ref)
	return len(path) == 2 &&
		ast.RegoRootDocument.Equal(path[0]) &&
		path[1].Equal(ast.InternedTerm("v1"))
}
