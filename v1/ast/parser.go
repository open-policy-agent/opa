// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math/big"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"

	"github.com/open-policy-agent/opa/v1/ast/internal/scanner"
	"github.com/open-policy-agent/opa/v1/ast/internal/tokens"
	astJSON "github.com/open-policy-agent/opa/v1/ast/json"
	"github.com/open-policy-agent/opa/v1/ast/location"
	"github.com/open-policy-agent/opa/v1/util"
)

// RegoVersion defines the Rego syntax requirements for a module.
type RegoVersion uint8

const (
	// DefaultRegoVersion is the default Rego version for this OPA version.
	DefaultRegoVersion = RegoV1

	// DefaultMaxParsingRecursionDepth is the default maximum recursion depth for the parser
	DefaultMaxParsingRecursionDepth = 100000
)

const (
	// RegoUndefined represents a Rego version unknown to OPA, like for a policy that has
	// yet to be parsed and a no version information has been provided by other means.
	RegoUndefined RegoVersion = iota
	// RegoV0 is the original Rego syntax, which was used by default in OPA < 1.0.
	RegoV0
	// RegoV0CompatV1 requires modules to comply with both the RegoV0 and RegoV1
	// syntax (requiring Rego v1 imports in a module to use v1 keywords).
	// For more information, see https://www.openpolicyagent.org/docs/v0-compatibility
	RegoV0CompatV1
	// RegoV1 is the Rego syntax enforced by OPA 1.0 and later versions, including the following changes:
	// - Keywords `in`, `every`, `ìf` and `contains` now part of the default set, and don't require explicit import
	// - Using 'if' and 'contains' now required in rule heads
	// - Most compiler checks previously enabled in "strict mode" now enabled by default
	// For more information, see https://www.openpolicyagent.org/docs/v0-upgrade
	RegoV1
)

var (
	// ErrMaxParsingRecursionDepthExceeded is returned when the parser
	// recursion exceeds the maximum allowed depth
	ErrMaxParsingRecursionDepthExceeded = errors.New("max parsing recursion depth exceeded")

	RegoV1CompatibleRef = Ref{VarTerm("rego"), InternedTerm("v1")}

	// this is the name to use for instantiating an empty set, e.g., `set()`.
	setConstructor = RefTerm(VarTerm("set"))

	preAllocWildcards = [...]Value{
		Var("$0"), Var("$1"), Var("$2"), Var("$3"), Var("$4"), Var("$5"),
		Var("$6"), Var("$7"), Var("$8"), Var("$9"), Var("$10"),
	}
	metadataBytes      = []byte("METADATA")
	metadataParserPool = util.NewSyncPool[metadataParser]()
)

func (v RegoVersion) Int() int {
	if v == RegoV1 {
		return 1
	}
	return 0
}

func (v RegoVersion) String() string {
	switch v {
	case RegoV0:
		return "v0"
	case RegoV1:
		return "v1"
	case RegoV0CompatV1:
		return "v0v1"
	default:
		return "unknown"
	}
}

func RegoVersionFromInt(i int) RegoVersion {
	if i == 1 {
		return RegoV1
	}
	return RegoV0
}

// Note: This state is kept isolated from the parser so that we
// can do efficient shallow copies of these values when doing a
// save() and restore().
type state struct {
	errors    Errors
	comments  []*Comment
	hints     []string
	s         *scanner.Scanner
	loc       Location
	lit       string
	lastEnd   int
	tokEnd    int
	wildcard  int
	tok       tokens.Token
	skippedNL bool
}

func (s *state) String() string {
	return fmt.Sprintf("<s: %v, tok: %v, lit: %q, loc: %v, errors: %d, comments: %d>", s.s, s.tok, s.lit, s.loc, len(s.errors), len(s.comments))
}

func (s *state) Loc() *location.Location {
	cpy := s.loc
	return &cpy
}

func (s *state) Text(offset, end int) []byte {
	bs := s.s.Bytes()
	if offset >= 0 && offset < len(bs) {
		if end >= offset && end <= len(bs) {
			return bs[offset:end]
		}
	}
	return nil
}

// Parser is used to parse Rego statements.
type Parser struct {
	r                 io.Reader
	s                 *state
	po                ParserOptions
	cache             parsedTermCache
	recursionDepth    int
	maxRecursionDepth int
	notBodies         bool
}

type parsedTermCacheItem struct {
	t      *Term
	post   *state // post is the post-state that's restored on a cache-hit
	offset int
	next   *parsedTermCacheItem
}

type parsedTermCache struct {
	m *parsedTermCacheItem
}

func (c parsedTermCache) String() string {
	s := strings.Builder{}
	s.WriteRune('{')
	var e *parsedTermCacheItem
	for e = c.m; e != nil; e = e.next {
		s.WriteString(e.String())
	}
	s.WriteRune('}')
	return s.String()
}

func (e *parsedTermCacheItem) String() string {
	return fmt.Sprintf("<%d:%v>", e.offset, e.t)
}

// ParserOptions defines the options for parsing Rego statements.
type ParserOptions struct {
	Capabilities      *Capabilities
	ProcessAnnotation bool
	AllFutureKeywords bool
	FutureKeywords    []string
	SkipRules         bool
	// RegoVersion is the version of Rego to parse for.
	RegoVersion RegoVersion
}

// EffectiveRegoVersion returns the effective RegoVersion to use for parsing.
func (po *ParserOptions) EffectiveRegoVersion() RegoVersion {
	if po.RegoVersion == RegoUndefined {
		return DefaultRegoVersion
	}
	return po.RegoVersion
}

// NewParser creates and initializes a Parser.
func NewParser() *Parser {
	p := &Parser{
		s:                 &state{},
		po:                ParserOptions{},
		maxRecursionDepth: DefaultMaxParsingRecursionDepth,
	}
	return p
}

// WithMaxRecursionDepth sets the maximum recursion depth for the parser.
func (p *Parser) WithMaxRecursionDepth(depth int) *Parser {
	p.maxRecursionDepth = depth
	return p
}

// WithFilename provides the filename for Location details
// on parsed statements.
func (p *Parser) WithFilename(filename string) *Parser {
	p.s.loc.File = filename
	return p
}

// WithReader provides the io.Reader that the parser will
// use as its source.
func (p *Parser) WithReader(r io.Reader) *Parser {
	p.r = r
	return p
}

// WithProcessAnnotation enables or disables the processing of
// annotations by the Parser
func (p *Parser) WithProcessAnnotation(processAnnotation bool) *Parser {
	p.po.ProcessAnnotation = processAnnotation
	return p
}

// WithFutureKeywords enables "future" keywords, i.e., keywords that can
// be imported via
//
//	import future.keywords.kw
//	import future.keywords.other
//
// but in a more direct way. The equivalent of this import would be
//
//	WithFutureKeywords("kw", "other")
func (p *Parser) WithFutureKeywords(kws ...string) *Parser {
	p.po.FutureKeywords = kws
	return p
}

// WithAllFutureKeywords enables all "future" keywords, i.e., the
// ParserOption equivalent of
//
//	import future.keywords
func (p *Parser) WithAllFutureKeywords(yes bool) *Parser {
	p.po.AllFutureKeywords = yes
	return p
}

// WithCapabilities sets the capabilities structure on the parser.
func (p *Parser) WithCapabilities(c *Capabilities) *Parser {
	p.po.Capabilities = c
	return p
}

// WithSkipRules instructs the parser not to attempt to parse Rule statements.
func (p *Parser) WithSkipRules(skip bool) *Parser {
	p.po.SkipRules = skip
	return p
}

// WithJSONOptions sets the JSON options on the parser (now a no-op).
//
// Deprecated: Use SetOptions in the json package instead, where a longer description
// of why this is deprecated also can be found.
func (p *Parser) WithJSONOptions(_ *astJSON.Options) *Parser {
	return p
}

func (p *Parser) WithRegoVersion(version RegoVersion) *Parser {
	p.po.RegoVersion = version
	return p
}

func (p *Parser) parsedTermCacheLookup() (*Term, *state) {
	l := p.s.loc.Offset
	// stop comparing once the cached offsets are lower than l
	for h := p.cache.m; h != nil && h.offset >= l; h = h.next {
		if h.offset == l {
			return h.t, h.post
		}
	}
	return nil, nil
}

func (p *Parser) parsedTermCachePush(t *Term, s0 *state) {
	s1 := p.save()
	o0 := s0.loc.Offset
	entry := parsedTermCacheItem{t: t, post: s1, offset: o0}

	// find the first one whose offset is smaller than ours
	var e *parsedTermCacheItem
	for e = p.cache.m; e != nil; e = e.next {
		if e.offset < o0 {
			break
		}
	}
	entry.next = e
	p.cache.m = &entry
}

// futureParser returns a shallow copy of `p` with an empty
// cache, and a scanner that knows all future keywords.
// It's used to present hints in errors, when statements would
// only parse successfully if some future keyword is enabled.
func (p *Parser) futureParser() *Parser {
	q := *p
	q.s = p.save()
	q.s.s = p.s.s.WithKeywords(allFutureKeywords)
	q.cache = parsedTermCache{}
	return &q
}

// presentParser returns a shallow copy of `p` with an empty
// cache, and a scanner that knows none of the future keywords.
// It is used to successfully parse keyword imports, like
//
//	import future.keywords.in
//
// even when the parser has already been informed about the
// future keyword "in". This parser won't error out because
// "in" is an identifier.
func (p *Parser) presentParser() (*Parser, map[string]tokens.Token) {
	var cpy map[string]tokens.Token
	q := *p
	q.s = p.save()
	q.s.s, cpy = p.s.s.WithoutKeywords(allFutureKeywords)
	q.cache = parsedTermCache{}
	return &q, cpy
}

// Parse will read the Rego source and parse statements and
// comments as they are found. Any errors encountered while
// parsing will be accumulated and returned as a list of Errors.
func (p *Parser) Parse() ([]Statement, []*Comment, Errors) {
	if p.po.Capabilities == nil {
		p.po.Capabilities = CapabilitiesForThisVersion(CapabilitiesRegoVersion(p.po.RegoVersion))
	}

	allowedFutureKeywords := map[string]tokens.Token{}

	if p.po.EffectiveRegoVersion() == RegoV1 {
		if !p.po.Capabilities.ContainsFeature(FeatureRegoV1) {
			return nil, nil, Errors{
				&Error{Code: ParseErr, Message: "illegal capabilities: rego_v1 feature required for parsing v1 Rego"},
			}
		}

		// rego-v1 includes all v0 future keywords in the default language definition
		maps.Copy(allowedFutureKeywords, futureKeywordsV0)

		for _, kw := range p.po.Capabilities.FutureKeywords {
			if tok, ok := futureKeywords[kw]; ok {
				allowedFutureKeywords[kw] = tok
			} else {
				// For sake of error reporting, we still need to check that keywords in capabilities are known in v0
				if _, ok := futureKeywordsV0[kw]; !ok {
					return nil, nil, Errors{
						&Error{Code: ParseErr, Message: "illegal capabilities: unknown keyword: " + kw},
					}
				}
			}
		}

		// Check that explicitly requested future keywords are known.
		for _, kw := range p.po.FutureKeywords {
			if _, ok := allowedFutureKeywords[kw]; !ok {
				return nil, nil, Errors{&Error{Code: ParseErr, Message: "unknown future keyword: " + kw}}
			}
		}
	} else {
		for _, kw := range p.po.Capabilities.FutureKeywords {
			var ok bool
			allowedFutureKeywords[kw], ok = allFutureKeywords[kw]
			if !ok {
				return nil, nil, Errors{&Error{Code: ParseErr, Message: "illegal capabilities: unknown keyword: " + kw}}
			}
		}

		if p.po.Capabilities.ContainsFeature(FeatureRegoV1) {
			// rego-v1 includes all v0 future keywords in the default language definition
			maps.Copy(allowedFutureKeywords, futureKeywordsV0)
		}
	}

	var selected map[string]tokens.Token
	if p.po.AllFutureKeywords {
		selected = make(map[string]tokens.Token, len(allowedFutureKeywords))
		maps.Copy(selected, allowedFutureKeywords)
	} else {
		if p.po.EffectiveRegoVersion() == RegoV1 {
			selected = make(map[string]tokens.Token, len(futureKeywordsV0)+len(p.po.FutureKeywords))
			for kw := range futureKeywordsV0 {
				tok, ok := allowedFutureKeywords[kw]
				if !ok {
					return nil, nil, Errors{&Error{Code: ParseErr, Message: "unknown future keyword: " + kw}}
				}
				selected[kw] = tok
			}
		} else {
			selected = make(map[string]tokens.Token, len(p.po.FutureKeywords))
		}

		for _, kw := range p.po.FutureKeywords {
			tok, ok := allowedFutureKeywords[kw]
			if !ok {
				return nil, nil, Errors{&Error{Code: ParseErr, Message: "unknown future keyword: " + kw}}
			}
			selected[kw] = tok
		}
	}

	if _, ok := selected["not"]; ok {
		p.notBodies = true
	}

	var err error
	if p.s.s, err = scanner.New(p.r); err != nil {
		return nil, nil, Errors{&Error{Code: ParseErr, Message: err.Error()}}
	}

	for name, token := range selected {
		p.s.s.AddKeyword(name, token)
	}

	// read the first token to initialize the parser
	p.scan()

	var stmts []Statement

	// Read from the scanner until the last token is reached or no statements
	// can be parsed. Attempt to parse package statements, import statements,
	// rule statements, and then body/query statements (in that order). If a
	// statement cannot be parsed, restore the parser state before trying the
	// next type of statement. If a statement can be parsed, continue from that
	// point trying to parse packages, imports, etc. in the same order.
	for p.s.tok != tokens.EOF {
		var s *state
		if p.s.tok == tokens.Package {
			s = p.save()
			if pkg := p.parsePackage(); pkg != nil {
				stmts = append(stmts, pkg)
				continue
			} else if len(p.s.errors) > 0 {
				break
			}
			p.restore(s)
		}

		if p.s.tok == tokens.Import {
			s = p.save()
			if imp := p.parseImport(); imp != nil {
				if RegoRootDocument.Equal(imp.Path.Value.(Ref)[0]) {
					p.regoV1Import(imp)
				} else if FutureRootDocument.Equal(imp.Path.Value.(Ref)[0]) {
					p.futureImport(imp, allowedFutureKeywords)
				}
				stmts = append(stmts, imp)
				continue
			} else if len(p.s.errors) > 0 {
				break
			}
			p.restore(s)
		}

		if !p.po.SkipRules {
			s = p.save()
			if rules := p.parseRules(); rules != nil {
				for i := range rules {
					stmts = append(stmts, rules[i])
				}
				continue
			} else if len(p.s.errors) > 0 {
				break
			}
			p.restore(s)
		}

		if body := p.parseQuery(true, tokens.EOF); body != nil {
			stmts = append(stmts, body)
			continue
		}

		break
	}

	if p.po.ProcessAnnotation {
		stmts = p.parseAnnotations(stmts)
	}

	return stmts, p.s.comments, p.s.errors
}

func (p *Parser) parseAnnotations(stmts []Statement) []Statement {
	annotStmts, errs := parseAnnotations(p.s.comments)
	for _, err := range errs {
		p.error(err.Location, err.Message)
	}

	stmts = slices.Grow(stmts, len(annotStmts))
	for _, annotStmt := range annotStmts {
		stmts = append(stmts, annotStmt)
	}

	return stmts
}

func parseAnnotations(comments []*Comment) (stmts []*Annotations, errs Errors) {
	numBlocks := util.Count(IsMetadataComment, comments...)
	if numBlocks == 0 {
		return nil, nil
	}

	stmts = make([]*Annotations, 0, numBlocks)
	mdp := metadataParserPool.Get()
	if mdp.buf == nil {
		mdp.buf = &bytes.Buffer{}
	}

	for i := range comments {
		if IsMetadataComment(comments[i]) { // scan until end of block
			mdp.Reset(comments[i].Location)
			for i++; i < len(comments) && !blockBuster(comments[i], comments[i-1]); i++ {
				mdp.Append(comments[i])
			}

			if a, err := mdp.Parse(); err != nil {
				errs = append(errs, &Error{Code: ParseErr, Message: err.Error(), Location: mdp.loc})
			} else {
				stmts = append(stmts, a)
			}
		}
	}

	metadataParserPool.Put(mdp)

	return stmts, errs
}

func IsMetadataComment(c *Comment) bool {
	return c.Location.Col == 1 && bytes.HasPrefix(bytes.TrimSpace(c.Text), metadataBytes)
}

func blockBuster(curr, prev *Comment) bool { // or endOfBlock, but the name was too good to pass up
	return curr.Location.Col != 1 || curr.Location.Row-1 != prev.Location.Row || IsMetadataComment(curr)
}

func (p *Parser) parsePackage() *Package {
	if p.s.tok != tokens.Package {
		return nil
	}

	var pkg Package
	pkg.SetLoc(p.s.Loc())

	p.scanWS()

	// Make sure we allow the first term of refs to be the 'package' keyword.
	if p.s.tok == tokens.Dot || p.s.tok == tokens.LBrack {
		// This is a ref, not a package declaration.
		return nil
	}

	if p.s.tok == tokens.Whitespace {
		p.scan()
	}

	if !isIdentOrAllowedRefKeyword(p) {
		p.illegalToken()
		return nil
	}

	term := p.parseTerm()

	if term != nil {
		switch v := term.Value.(type) {
		case Var:
			pkg.Path = Ref{
				DefaultRootDocument.Copy().SetLocation(term.Location),
				StringTerm(string(v)).SetLocation(term.Location),
			}
		case Ref:
			pkg.Path = make(Ref, len(v)+1)
			pkg.Path[0] = DefaultRootDocument.Copy().SetLocation(v[0].Location)
			first, ok := v[0].Value.(Var)
			if !ok {
				p.errorf(v[0].Location, "unexpected %v token: expecting var", ValueName(v[0].Value))
				return nil
			}
			pkg.Path[1] = StringTerm(string(first)).SetLocation(v[0].Location)
			for i := 2; i < len(pkg.Path); i++ {
				switch v[i-1].Value.(type) {
				case String:
					pkg.Path[i] = v[i-1]
				default:
					p.errorf(v[i-1].Location, "unexpected %v token: expecting string", ValueName(v[i-1].Value))
					return nil
				}
			}
		default:
			p.illegalToken()
			return nil
		}
	}

	if pkg.Path == nil {
		if len(p.s.errors) == 0 {
			p.error(p.s.Loc(), "expected path")
		}
		return nil
	}

	return &pkg
}

func (p *Parser) parseImport() *Import {
	if p.s.tok != tokens.Import {
		return nil
	}

	var imp Import
	imp.SetLoc(p.s.Loc())

	p.scanWS()

	// Make sure we allow the first term of refs to be the 'import' keyword.
	if p.s.tok == tokens.Dot || p.s.tok == tokens.LBrack {
		// This is a ref, not an import declaration.
		return nil
	}

	if p.s.tok == tokens.Whitespace {
		p.scan()
	}

	if !isIdentOrAllowedRefKeyword(p) {
		p.illegalToken()
		return nil
	}

	q, prev := p.presentParser()
	term := q.parseTerm()
	if term != nil {
		switch v := term.Value.(type) {
		case Var:
			imp.Path = RefTerm(term).SetLocation(term.Location)
		case Ref:
			for i := 1; i < len(v); i++ {
				if _, ok := v[i].Value.(String); !ok {
					p.errorf(v[i].Location, "unexpected %v token: expecting string", ValueName(v[i].Value))
					return nil
				}
			}
			imp.Path = term
		}
	}
	// keep advanced parser state, reset known keywords
	p.s = q.s
	p.s.s = q.s.s.WithKeywords(prev)

	if imp.Path == nil {
		p.error(p.s.Loc(), "expected path")
		return nil
	}

	path := imp.Path.Value.(Ref)

	switch {
	case RootDocumentNames.Contains(path[0]):
	case FutureRootDocument.Equal(path[0]):
	case RegoRootDocument.Equal(path[0]):
	default:
		p.hint("if this is unexpected, try updating OPA")
		p.errorf(imp.Path.Location, "unexpected import path, must begin with one of: %v, got: %v",
			RootDocumentNames.Union(NewSet(FutureRootDocument, RegoRootDocument)),
			path[0])
		return nil
	}

	if p.s.tok == tokens.As {
		p.scan()

		if p.s.tok != tokens.Ident {
			p.illegal("expected var")
			return nil
		}

		if alias := p.parseTerm(); alias != nil {
			v, ok := alias.Value.(Var)
			if ok {
				imp.Alias = v
				return &imp
			}
		}
		p.illegal("expected var")
		return nil
	}

	if imp.Alias != "" {
		// Unreachable: parsing the alias var should already have generated an error.
		name := imp.Alias.String()
		if IsKeywordInRegoVersion(name, p.po.EffectiveRegoVersion()) {
			p.errorf(imp.Location, "unexpected import alias, must not be a keyword, got: %s", name)
		}
		return &imp
	}

	r := imp.Path.Value.(Ref)

	// Don't allow keywords in the tail path term unless it's a future import
	if len(r) == 1 {
		t := r[0]
		name := string(t.Value.(Var))
		if IsKeywordInRegoVersion(name, p.po.EffectiveRegoVersion()) {
			p.hint("import a different path or use an alias")
			p.errorf(t.Location, "unexpected import path, must not end with a keyword, got: %s", name)
		}
	} else if !FutureRootDocument.Equal(r[0]) {
		t := r[len(r)-1]
		name := string(t.Value.(String))
		if IsKeywordInRegoVersion(name, p.po.EffectiveRegoVersion()) {
			p.hint("import a different path or use an alias")
			p.errorf(t.Location, "unexpected import path, must not end with a keyword, got: %s", name)
		}
	}

	return &imp
}

// isIdentOrAllowedRefKeyword checks if the current token is an Ident or a keyword in the active rego-version.
// If a keyword, sets p.s.token to token.Ident
func isIdentOrAllowedRefKeyword(p *Parser) bool {
	if p.s.tok == tokens.Ident {
		return true
	}

	if p.isAllowedRefKeyword(p.s.tok) {
		p.s.tok = tokens.Ident
		return true
	}

	return false
}

func scanAheadRef(p *Parser) bool {
	if p.isAllowedRefKeyword(p.s.tok) {
		// scan ahead to check if we're parsing a ref
		s := p.save()
		p.scanWS()
		tok := p.s.tok
		p.restore(s)

		if tok == tokens.Dot || tok == tokens.LBrack {
			p.s.tok = tokens.Ident
			return true
		}
	}

	return false
}

// scanAheadLogicalCall rewrites an `and`/`or` keyword token to tokens.Ident when
// it's immediately followed by `(`. Only valid where a term is expected: there,
// the operator reading is impossible, so it must be a function (`&`/`|` set built-ins).
// Operator position is decided before any term is parsed, which is what keeps `x and (b)` a keyword.
func scanAheadLogicalCall(p *Parser) {
	if p.s.tok != tokens.LogicalAnd && p.s.tok != tokens.LogicalOr {
		return
	}

	s := p.save()
	p.scanWS()
	tok := p.s.tok
	p.restore(s)

	if tok == tokens.LParen {
		// This is a call to a function named `and`/`or`
		p.s.tok = tokens.Ident
	}
}

func (p *Parser) parseRules() []*Rule {

	var rule Rule
	rule.SetLoc(p.s.Loc())

	// This allows keywords in the first var term of the ref
	_ = scanAheadRef(p)

	if p.s.tok == tokens.Default {
		p.scan()
		rule.Default = true
		_ = scanAheadRef(p)
	}

	if p.s.tok != tokens.Ident {
		return nil
	}

	usesContains := false
	if rule.Head, usesContains = p.parseHead(rule.Default); rule.Head == nil {
		return nil
	}

	if usesContains {
		rule.Head.keywords = append(rule.Head.keywords, tokens.Contains)
	}

	if rule.Default {
		if !p.validateDefaultRuleValue(&rule) {
			return nil
		}

		if len(rule.Head.Args) > 0 {
			if !p.validateDefaultRuleArgs(&rule) {
				return nil
			}
		}

		rule.Body = NewBody(NewExpr(BooleanTerm(true).SetLocation(rule.Location)).SetLocation(rule.Location))
		return []*Rule{&rule}
	}

	// back-compat with `p[x] { ... }``
	hasIf := p.s.tok == tokens.If

	// p[x] if ...  becomes a single-value rule p[x]
	if hasIf && !usesContains && len(rule.Head.Ref()) == 2 {
		v := rule.Head.Ref()[1]
		_, isRef := v.Value.(Ref)
		if (!v.IsGround() || isRef) && len(rule.Head.Args) == 0 {
			rule.Head.Key = rule.Head.Ref()[1]
		}

		if rule.Head.Value == nil {
			rule.Head.generatedValue = true
			rule.Head.Value = BooleanTerm(true).SetLocation(rule.Head.Location)
		} else {
			// p[x] = y if  becomes a single-value rule p[x] with value y, but needs name for compat
			v, ok := rule.Head.Ref()[0].Value.(Var)
			if !ok {
				return nil
			}
			rule.Head.Name = v
		}
	}

	// p[x]         becomes a multi-value rule p
	if !hasIf && !usesContains &&
		len(rule.Head.Args) == 0 && // not a function
		len(rule.Head.Ref()) == 2 { // ref like 'p[x]'
		v, ok := rule.Head.Ref()[0].Value.(Var)
		if !ok {
			return nil
		}
		rule.Head.Name = v
		rule.Head.Key = rule.Head.Ref()[1]
		if rule.Head.Value == nil {
			rule.Head.SetRef(rule.Head.Ref()[:len(rule.Head.Ref())-1])
		}
	}

	switch {
	case hasIf:
		rule.Head.keywords = append(rule.Head.keywords, tokens.If)
		p.scan()
		s := p.save()

		// Only a set term with a leading '{' is ambiguous with a body;
		// e.g.: 'not {...}' and 'set()' parses to a set literal, but have no ambiguous leading '{'
		leadingBrace := p.s.tok == tokens.LBrace

		if expr := p.parseLiteral(); expr != nil {
			// NOTE(sr): set literals are never false or undefined, so parsing this as
			//  p if { true }
			//       ^^^^^^^^ set of one element, `true`
			// isn't valid.
			isSetLiteral := false
			if t, ok := expr.Terms.(*Term); ok && leadingBrace {
				_, isSetLiteral = t.Value.(Set)
			}
			// expr.Term is []*Term or Every
			if !isSetLiteral {
				rule.Body.Append(expr)
				break
			}
		}

		if !leadingBrace {
			// Without a leading '{' there is no '{ BODY }' rule body to fall back to,
			// so the literal's own error is the useful one; restoring would drop it.
			return nil
		}

		// parsing as literal didn't work out, expect '{ BODY }'
		p.restore(s)
		fallthrough

	case p.s.tok == tokens.LBrace:
		p.scan()
		if rule.Body = p.parseBody(tokens.RBrace); rule.Body == nil {
			return nil
		}
		p.scan()

	case usesContains:
		rule.Body = NewBody(NewExpr(BooleanTerm(true).SetLocation(rule.Location)).SetLocation(rule.Location))
		rule.generatedBody = true
		rule.Location = rule.Head.Location

		return []*Rule{&rule}

	default:
		return nil
	}

	if p.s.tok == tokens.Else {
		// This might just be a refhead rule with a leading 'else' term.
		if !scanAheadRef(p) {
			if r := rule.Head.Ref(); len(r) > 1 && !r.IsGround() {
				p.error(p.s.Loc(), "else keyword cannot be used on rules with variables in head")
				return nil
			}
			if rule.Head.Key != nil {
				p.error(p.s.Loc(), "else keyword cannot be used on multi-value rules")
				return nil
			}

			if rule.Else = p.parseElse(rule.Head); rule.Else == nil {
				return nil
			}
		}
	}

	rule.Location.Text = p.s.Text(rule.Location.Offset, p.s.lastEnd)

	rules := []*Rule{&rule}

	for p.s.tok == tokens.LBrace {

		if rule.Else != nil {
			p.error(p.s.Loc(), "expected else keyword")
			return nil
		}

		loc := p.s.Loc()

		p.scan()
		var next Rule

		if next.Body = p.parseBody(tokens.RBrace); next.Body == nil {
			return nil
		}
		p.scan()

		loc.Text = p.s.Text(loc.Offset, p.s.lastEnd)
		next.SetLoc(loc)

		// Chained rule head's keep the original
		// rule's head AST but have their location
		// set to the rule body.
		next.Head = rule.Head.Copy()
		next.Head.keywords = rule.Head.keywords
		for i := range next.Head.Args {
			if v, ok := next.Head.Args[i].Value.(Var); ok && v.IsWildcard() {
				next.Head.Args[i].Value = p.genwildcard()
			}
		}
		setLocRecursive(next.Head, loc)

		rules = append(rules, &next)
	}

	return rules
}

func (p *Parser) parseElse(head *Head) *Rule {

	var rule Rule
	rule.SetLoc(p.s.Loc())

	rule.Head = head.Copy()
	rule.Head.generatedValue = false
	for i := range rule.Head.Args {
		if v, ok := rule.Head.Args[i].Value.(Var); ok && v.IsWildcard() {
			rule.Head.Args[i].Value = p.genwildcard()
		}
	}
	rule.Head.SetLoc(p.s.Loc())

	defer func() {
		rule.Location.Text = p.s.Text(rule.Location.Offset, p.s.lastEnd)
	}()

	p.scan()

	switch p.s.tok {
	case tokens.LBrace, tokens.If: // no value, but a body follows directly
		rule.Head.generatedValue = true
		rule.Head.Value = BooleanTerm(true)
	case tokens.Assign, tokens.Unify:
		rule.Head.Assign = tokens.Assign == p.s.tok
		p.scan()
		rule.Head.Value = p.parseTermInfixCall()
		if rule.Head.Value == nil {
			return nil
		}
		rule.Head.Location.Text = p.s.Text(rule.Head.Location.Offset, p.s.lastEnd)
	default:
		p.illegal("expected else value term or rule body")
		return nil
	}

	hasIf := p.s.tok == tokens.If
	hasLBrace := p.s.tok == tokens.LBrace

	if !hasIf && !hasLBrace {
		rule.Body = NewBody(NewExpr(BooleanTerm(true)))
		rule.generatedBody = true
		setLocRecursive(rule.Body, rule.Location)
		return &rule
	}

	if hasIf {
		rule.Head.keywords = append(rule.Head.keywords, tokens.If)
		p.scan()
	}

	if p.s.tok == tokens.LBrace {
		p.scan()
		if rule.Body = p.parseBody(tokens.RBrace); rule.Body == nil {
			return nil
		}
		p.scan()
	} else if p.s.tok != tokens.EOF {
		expr := p.parseLiteral()
		if expr == nil {
			return nil
		}
		rule.Body.Append(expr)
		setLocRecursive(rule.Body, rule.Location)
	} else {
		p.illegal("rule body expected")
		return nil
	}

	if p.s.tok == tokens.Else {
		if rule.Else = p.parseElse(head); rule.Else == nil {
			return nil
		}
	}
	return &rule
}

func (p *Parser) parseHead(defaultRule bool) (*Head, bool) {
	head := &Head{}
	loc := p.s.Loc()
	defer func() {
		if head != nil {
			head.SetLoc(loc)
			head.Location.Text = p.s.Text(head.Location.Offset, p.s.lastEnd)
		}
	}()

	term := p.parseVar()
	if term == nil {
		return nil, false
	}

	ref := p.parseHeadFinish(term, true)
	if ref == nil {
		p.illegal("expected rule head name")
		return nil, false
	}

	switch x := ref.Value.(type) {
	case Var:
		// TODO
		head = VarHead(x, ref.Location, nil)
	case Ref:
		head = RefHead(x)
	case Call:
		op, args := x[0], x[1:]
		var ref Ref
		switch y := op.Value.(type) {
		case Var:
			ref = Ref{op}
		case Ref:
			if _, ok := y[0].Value.(Var); !ok {
				p.illegal("rule head ref %v invalid", y)
				return nil, false
			}
			ref = y
		}
		head = RefHead(ref)
		head.Args = slices.Clone[[]*Term](args)

	default:
		return nil, false
	}

	name := head.Ref().String()

	switch p.s.tok {
	case tokens.Contains: // NOTE: no Value for `contains` heads, we return here
		// Catch error case of using 'contains' with a function definition rule head.
		if head.Args != nil {
			p.illegal("the contains keyword can only be used with multi-value rule definitions (e.g., %s contains <VALUE> { ... })", name)
		}
		p.scan()
		head.Key = p.parseTermInfixCall()
		if head.Key == nil {
			p.illegal("expected rule key term (e.g., %s contains <VALUE> { ... })", name)
		}
		return head, true

	case tokens.Unify:
		p.scan()
		head.Value = p.parseTermInfixCall()
		if head.Value == nil {
			// FIX HEAD.String()
			p.illegal("expected rule value term (e.g., %s[%s] = <VALUE> { ... })", name, head.Key)
		}
	case tokens.Assign:
		p.scan()
		head.Assign = true
		head.Value = p.parseTermInfixCall()
		if head.Value == nil {
			switch {
			case len(head.Args) > 0:
				p.illegal("expected function value term (e.g., %s(...) := <VALUE> { ... })", name)
			case head.Key != nil:
				p.illegal("expected partial rule value term (e.g., %s[...] := <VALUE> { ... })", name)
			case defaultRule:
				p.illegal("expected default rule value term (e.g., default %s := <VALUE>)", name)
			default:
				p.illegal("expected rule value term (e.g., %s := <VALUE> { ... })", name)
			}
		}
	}

	if head.Value == nil && head.Key == nil {
		if len(head.Ref()) != 2 || len(head.Args) > 0 {
			head.generatedValue = true
			head.Value = BooleanTerm(true).SetLocation(head.Location)
		}
	}
	return head, false
}

func (p *Parser) parseBody(end tokens.Token) Body {
	if !p.enter() {
		return nil
	}
	defer p.leave()
	return p.parseQuery(false, end)
}

func (p *Parser) parseQuery(requireSemi bool, end tokens.Token) Body {
	if p.s.tok == end {
		p.error(p.s.Loc(), "found empty body")
		return nil
	}

	body := Body{}

	for {
		expr := p.parseLiteral()
		if expr == nil {
			return nil
		}

		body.Append(expr)

		if p.s.tok == tokens.Semicolon {
			p.scan()
			continue
		}

		if p.s.tok == end || requireSemi {
			return body
		}

		if !p.s.skippedNL {
			// If there was already an error then don't pile this one on
			if len(p.s.errors) == 0 {
				p.illegal(`expected \n or %s or %s`, tokens.Semicolon, end)
			}
			return nil
		}
	}
}

func (p *Parser) parseLiteral() (expr *Expr) {

	offset := p.s.loc.Offset
	loc := p.s.Loc()

	defer func() {
		if expr != nil {
			loc.Text = p.s.Text(offset, p.s.lastEnd)
			expr.SetLoc(loc)
			// For implicit not-body wrapping (future.keywords.not), propagate
			// the outer `not <op>` span to the inner expression.
			if not, ok := expr.Terms.(*Not); ok && !not.ExplicitBody {
				for _, inner := range not.Body {
					inner.SetLoc(loc)
				}
			}
		}
	}()

	// LHS explicit-body operand of an `and`/`or` binary: `{ body } and/or ...`.
	// Speculatively parse `{...}`; if followed by an and/or operator, build the
	// binary. Otherwise, restore and fall through to regular handling.
	if p.s.tok == tokens.LBrace && p.logicalKeywordsActive() {
		s := p.save()
		braceOffset := p.s.loc.Offset
		bodyLoc := p.s.Loc()
		p.scan()
		body := p.parseBody(tokens.RBrace)
		if body != nil {
			p.scan() // consume `}`
			if p.s.tok == tokens.LogicalAnd || p.s.tok == tokens.LogicalOr {
				// Only now are the braces known to be an operand rather than a rule body.
				if isAmbiguousUnionBody(body) {
					p.errorAmbiguousUnionBody(bodyLoc, braceOffset, body, "")
					return nil
				}

				outer := p.parseLogicalOrChain(body, true, bodyLoc)
				if outer == nil {
					return nil
				}
				return p.attachWith(outer)
			}
		}
		p.restore(s)
	}

	// LHS/whole parenthesized group at statement start: `(a or b)`,
	// `(a or b) and c`, or `({a}) and c`. parseLogicalGroup only commits when the
	// parens hold or precede an and/or; otherwise (`({})`, `({a})`, `(a == b)`) it
	// restores and we fall through so parseExpr handles the term.
	if p.s.tok == tokens.LParen && p.logicalKeywordsActive() {
		if body, explicit, loc, committed := p.parseLogicalGroup(false, ""); committed {
			if body == nil {
				return nil
			}
			return p.foldLogicalTail(body, explicit, loc)
		}
	}

	// Check that we're not parsing a ref
	if p.isAllowedRefKeyword(p.s.tok) {
		// Scan ahead
		s := p.save()
		p.scanWS()
		tok := p.s.tok
		p.restore(s)

		if tok == tokens.Dot || tok == tokens.LBrack {
			p.s.tok = tokens.Ident
			return p.parseLiteralExpr(false, nil)
		}
	}

	var notLoc *Location
	negated := isNegated(p)
	if negated {
		notLoc = p.s.Loc()
		p.scan()
	}

	if negated && p.notBodies && p.s.tok == tokens.LBrace {
		nb := p.parseNotBody(notLoc)
		if nb == nil {
			return nil
		}
		// A not-body is a complete operand, so it may lead an and/or chain:
		// `not { x } and y`.
		return p.foldLogicalTail(NewBody(nb), false, nb.Location)
	}

	switch p.s.tok {
	case tokens.Some:
		if negated {
			p.illegal("illegal negation of 'some'")
			return nil
		}
		return p.parseSome()
	case tokens.Every:
		if negated {
			p.illegal("illegal negation of 'every'")
			return nil
		}
		return p.parseEvery()
	default:
		return p.parseLiteralExpr(negated, notLoc)
	}
}

func (p *Parser) isAllowedRefKeyword(t tokens.Token) bool {
	return p.isAllowedRefKeywordStr(t.String())
}

func (p *Parser) isAllowedRefKeywordStr(s string) bool {
	if p.po.Capabilities.ContainsFeature(FeatureKeywordsInRefs) {
		return IsKeywordInRegoVersion(s, p.po.EffectiveRegoVersion()) || p.s.s.IsKeyword(s)
	}

	return false
}

func (p *Parser) parseLiteralExpr(negated bool, notLoc *Location) *Expr {
	s := p.save()

	// Negated parenthesized group: `not (a or b)`. The parens are an operand of
	// `not`, so any `{...}` inside is a body.
	if negated && p.notBodies && p.s.tok == tokens.LParen {
		if body, explicit, _, committed := p.parseLogicalGroup(true, "not "); committed {
			if body == nil {
				return nil
			}

			spanned := p.extendLoc(notLoc)
			not := NewExpr(&Not{Body: body, ExplicitBody: explicit, Location: spanned}).SetLocation(spanned)

			return p.foldLogicalTail(NewBody(not), false, spanned)
		}
	}

	expr := p.parseExpr()
	if expr != nil {
		var withLoc *Location
		if p.s.tok == tokens.With {
			withLoc = p.s.Loc()
			if expr.With = p.parseWith(); expr.With == nil {
				return nil
			}
		}

		if p.isFutureKeyword("every") {
			// If we find a plain `every` identifier, attempt to parse an every expression,
			// add hint if it succeeds.
			if term, ok := expr.Terms.(*Term); ok && Var("every").Equal(term.Value) {
				var hint bool
				t := p.save()
				p.restore(s)
				if expr := p.futureParser().parseEvery(); expr != nil {
					_, hint = expr.Terms.(*Every)
				}
				p.restore(t)
				if hint {
					p.hint("`import future.keywords.every` for `every x in xs { ... }` expressions")
				}
			}
		}

		if negated && p.notBodies {
			// Move 'with' statement to outer not expr
			w := expr.With
			expr.With = nil

			var spanned *Location
			if notLoc != nil {
				// Extend the location to also include the 'not ' prefix
				spanned = p.extendLoc(notLoc)
			}

			expr = NewExpr(&Not{Body: NewBody(expr), Location: spanned}).SetLocation(spanned)
			expr.With = w
		} else {
			expr.Negated = negated
		}

		if p.s.tok == tokens.LogicalAnd || p.s.tok == tokens.LogicalOr {
			if withLoc != nil {
				p.errWithOnOperand(withLoc, p.s.tok.String())
				return nil
			}

			if expr.Location == nil {
				startLoc := s.Loc()
				startLoc.Text = p.s.Text(startLoc.Offset, p.s.lastEnd)
				expr.SetLoc(startLoc)
			}

			if notLoc == nil && bytes.HasPrefix(expr.Location.Text, []byte("{")) {
				// `{}` on its own is an empty body
				if isEmptyObjectTerm(expr) {
					p.error(expr.Location, "found empty body")
					return nil
				}

				p.errorBraceLedOperand(expr.Location, expr.Location.Text, p.s.tok.String())
				return nil
			}

			outer := p.parseLogicalOrChain(NewBody(expr), false, expr.Location)
			if outer == nil {
				return nil
			}
			return p.attachWith(outer)
		}
	}
	return expr
}

func (p *Parser) parseWith() []*With {
	withs := []*With{}

	for {
		with := With{Location: p.s.Loc()}

		p.scan()

		if p.s.tok != tokens.Ident {
			p.illegal("expected ident")
			return nil
		}

		with.Target = p.parseTerm()
		if with.Target == nil {
			return nil
		}

		switch with.Target.Value.(type) {
		case Ref, Var:
			break
		default:
			p.illegal("expected with target path")
		}

		if p.s.tok != tokens.As {
			p.illegal("expected as keyword")
			return nil
		}

		p.scan()

		if with.Value = p.parseTermInfixCall(); with.Value == nil {
			return nil
		}

		with.Location.Text = p.s.Text(with.Location.Offset, p.s.lastEnd)

		withs = append(withs, &with)

		if p.s.tok != tokens.With {
			break
		}
	}

	return withs
}

func (p *Parser) attachWith(e *Expr) *Expr {
	if e != nil && p.s.tok == tokens.With {
		if e.With = p.parseWith(); e.With == nil {
			return nil
		}
	}
	return e
}

func (p *Parser) errWithOnOperand(loc *Location, kw string) {
	p.hint(fmt.Sprintf(
		"Wrap the operand in `(...)` or `{...}` to scope, or move `with` after the `%s` expression to apply it to the whole expression",
		kw))
	p.errorf(loc,
		"`with` modifier is not allowed on operand of `%s`",
		kw)
}

func (p *Parser) foldLogicalTail(body Body, explicit bool, loc *Location) *Expr {
	if p.s.tok == tokens.LogicalAnd || p.s.tok == tokens.LogicalOr {
		outer := p.parseLogicalOrChain(body, explicit, loc)
		if outer == nil {
			return nil
		}
		return p.attachWith(outer)
	}
	return p.attachWith(body[0])
}

func (p *Parser) parseSome() *Expr {

	decl := &SomeDecl{}
	decl.SetLoc(p.s.Loc())

	// Attempt to parse "some x in xs", which will end up in
	//   SomeDecl{Symbols: ["member(x, xs)"]}
	s := p.save()
	p.scan()
	if term := p.parseTermInfixCall(); term != nil {
		if call, ok := term.Value.(Call); ok {
			switch call[0].String() {
			case Member.Name:
				if len(call) != 3 {
					p.illegal("illegal domain")
					return nil
				}
			case MemberWithKey.Name:
				if len(call) != 4 {
					p.illegal("illegal domain")
					return nil
				}
			default:
				p.illegal("expected `x in xs` or `x, y in xs` expression")
				return nil
			}

			decl.Symbols = []*Term{term}
			expr := NewExpr(decl).SetLocation(decl.Location)
			if p.s.tok == tokens.With {
				if expr.With = p.parseWith(); expr.With == nil {
					return nil
				}
			}
			return expr
		}
	}

	p.restore(s)

	if p.isFutureKeyword("in") {
		s = p.save() // new copy for later
		var hint bool
		p.scan()
		if term := p.futureParser().parseTermInfixCall(); term != nil {
			if call, ok := term.Value.(Call); ok {
				switch call[0].String() {
				case Member.Name, MemberWithKey.Name:
					hint = true
				}
			}
		}

		// go on as before, it's `some x[...]` or illegal
		p.restore(s)
		if hint {
			p.hint("`import future.keywords.in` for `some x in xs` expressions")
		}
	}

	for { // collecting var args
		p.scan()

		if p.s.tok != tokens.Ident {
			p.illegal("expected var")
			return nil
		}

		decl.Symbols = append(decl.Symbols, p.parseVar())

		p.scan()

		if p.s.tok != tokens.Comma {
			break
		}
	}

	return NewExpr(decl).SetLocation(decl.Location)
}

func (p *Parser) parseNotBody(notLoc *Location) *Expr {
	braceOffset := p.s.loc.Offset
	braceLoc := p.s.Loc()
	s := p.save()
	p.scan() // consume `{`

	// `not {}` is an empty body, which parseBody reports precisely; only non-empty
	// braces are worth re-reading as a value.
	empty := p.s.tok == tokens.RBrace

	body := p.parseBody(tokens.RBrace)
	if body == nil {
		if empty {
			return nil
		}

		// The braces may hold a value rather than a body. If so, report the
		// contract and its escapes; if not, keep the body error.
		failed := p.save()
		p.restore(s)

		// The operand can extend past the braces (`{1, 2} & input.s == set()`),
		// and parens group rather than delimit, so it is the whole operand that has to be wrapped.
		if term := p.parseTermInfixCall(); term != nil {
			p.errorOperandBraceNeedsBody(braceLoc, p.s.Text(braceOffset, p.s.lastEnd), term, "not ")
			return nil
		}

		p.restore(failed)

		return nil
	}
	p.scan() // consume `}`

	if isAmbiguousUnionBody(body) {
		p.errorAmbiguousUnionBody(braceLoc, braceOffset, body, "not ")
		return nil
	}

	// Extend the location to also include the 'not ' prefix
	spanned := p.extendLoc(notLoc)
	not := &Not{Body: body, ExplicitBody: true, Location: spanned}
	return NewExpr(not).SetLocation(spanned)
}

// logicalKeywordsActive reports whether the scanner currently treats `and` or
// `or` as keywords.
func (p *Parser) logicalKeywordsActive() bool {
	return p.s.s.IsKeyword("and") || p.s.s.IsKeyword("or")
}

// parseLogicalOrChain folds a left-associative chain of `or` operators on top
// of the given lhs, with `and`-chains folded in first because `and` binds
// tighter. The lhs is supplied as a (body, explicit, location) triple so that
// both implicit single-expression operands and explicit `{...}` operands can
// be represented.
func (p *Parser) parseLogicalOrChain(lhsBody Body, lhsExplicit bool, lhsLoc *Location) *Expr {
	if p.s.tok != tokens.LogicalAnd && p.s.tok != tokens.LogicalOr {
		panic("expected logical and/or operator at p.s.tok")
	}

	if !p.enter() {
		return nil
	}
	defer p.leave()

	// Higher precedence first: fold any leading `and`-chain into the lhs.
	if p.s.tok == tokens.LogicalAnd {
		andExpr := p.parseLogicalAndChain(lhsBody, lhsExplicit, lhsLoc)
		if andExpr == nil {
			return nil
		}
		lhsBody = NewBody(andExpr)
		lhsExplicit = false
		lhsLoc = andExpr.Location
	}

	for p.s.tok == tokens.LogicalOr {
		p.scan()

		rhsBody, rhsExplicit, rhsLoc := p.parseLogicalOperand("or")
		if rhsBody == nil {
			return nil
		}

		// RHS may extend into a higher-precedence `and`-chain.
		if p.s.tok == tokens.LogicalAnd {
			andExpr := p.parseLogicalAndChain(rhsBody, rhsExplicit, rhsLoc)
			if andExpr == nil {
				return nil
			}
			rhsBody = NewBody(andExpr)
			rhsExplicit = false
		}

		p.checkVoidCallOperands(lhsBody, rhsBody, "or")

		exprLoc := p.extendLoc(lhsLoc)
		node := &LogicalOr{
			Lhs:         lhsBody,
			Rhs:         rhsBody,
			ExplicitLhs: lhsExplicit,
			ExplicitRhs: rhsExplicit,
			Location:    exprLoc,
		}
		wrapper := NewExpr(node).SetLocation(exprLoc)
		lhsBody = NewBody(wrapper)
		lhsExplicit = false
		lhsLoc = exprLoc
	}

	return lhsBody[0]
}

// parseLogicalAndChain folds a left-associative chain of `and` operators on
// top of the given lhs.
func (p *Parser) parseLogicalAndChain(lhsBody Body, lhsExplicit bool, lhsLoc *Location) *Expr {
	if p.s.tok != tokens.LogicalAnd {
		panic("expected logical and operator at p.s.tok")
	}

	if !p.enter() {
		return nil
	}
	defer p.leave()

	for p.s.tok == tokens.LogicalAnd {
		p.scan()

		rhsBody, rhsExplicit, _ := p.parseLogicalOperand("and")
		if rhsBody == nil {
			return nil
		}

		p.checkVoidCallOperands(lhsBody, rhsBody, "and")

		exprLoc := p.extendLoc(lhsLoc)
		node := &LogicalAnd{
			Lhs:         lhsBody,
			Rhs:         rhsBody,
			ExplicitLhs: lhsExplicit,
			ExplicitRhs: rhsExplicit,
			Location:    exprLoc,
		}
		wrapper := NewExpr(node).SetLocation(exprLoc)
		lhsBody = NewBody(wrapper)
		lhsExplicit = false
		lhsLoc = exprLoc
	}

	return lhsBody[0]
}

// extendLoc returns a copy of start with Text re-spanned from start.Offset
// to the scanner's current lastEnd.
func (p *Parser) extendLoc(start *Location) *Location {
	cpy := *start
	cpy.Text = p.s.Text(start.Offset, p.s.lastEnd)
	return &cpy
}

func isNegated(p *Parser) bool {
	if p.s.tok != tokens.Not {
		return false
	}
	// Distinguish the `not` keyword from a ref like `not.x`.
	s := p.save()
	p.scanWS()
	tok := p.s.tok
	p.restore(s)
	return tok != tokens.Dot && tok != tokens.LBrack
}

// parseLogicalOperand parses a single operand of an `and`/`or` expression. op is
// the operator the operand belongs to, or "" when the caller is speculating and
// will restore on failure.
func (p *Parser) parseLogicalOperand(op string) (Body, bool, *Location) {
	if p.s.tok == tokens.LBrace {
		braceOffset := p.s.loc.Offset
		loc := p.s.Loc()
		s := p.save()
		p.scan()

		// `{}` is an empty body, which parseBody reports precisely; only non-empty
		// braces are worth re-reading as a value.
		empty := p.s.tok == tokens.RBrace

		body := p.parseBody(tokens.RBrace)
		if body == nil {
			if empty || op == "" {
				return nil, false, nil
			}

			// The braces may hold a value rather than a body.
			failed := p.save()
			p.restore(s)

			// The operand can extend past the braces (`{1, 2} & input.s == set()`),
			// and parens group rather than delimit, so it is the whole operand that has to be wrapped.
			if term := p.parseTermInfixCall(); term != nil {
				p.errorBraceLedOperand(loc, p.s.Text(braceOffset, p.s.lastEnd), op)
				return nil, false, nil
			}

			p.restore(failed)

			return nil, false, nil
		}
		p.scan()

		if isAmbiguousUnionBody(body) {
			// Report, but hand the body back: if the caller is a paren group that
			// restores, the error is rolled back with it.
			p.errorAmbiguousUnionBody(loc, braceOffset, body, "")
		}

		return body, true, loc
	}

	var notLoc *Location
	negated := isNegated(p)
	if negated {
		notLoc = p.s.Loc()
		p.scan()
	}

	if negated && p.notBodies && p.s.tok == tokens.LBrace {
		nb := p.parseNotBody(notLoc)
		if nb == nil {
			return nil, false, nil
		}
		return NewBody(nb), false, nb.Location
	}

	// Parenthesized logical group operand: `(a or b)` or, when negated,
	// `not (a or b)`. This is an operand of and/or/not, so a `{...}` inside is a
	// body. If the parens don't hold a logical group parseLogicalGroup restores
	// state and we fall through so parseExpr can handle `(a == b)` as a term.
	if p.s.tok == tokens.LParen && p.logicalKeywordsActive() && (!negated || p.notBodies) {
		prefix := ""
		if negated {
			prefix = "not "
		}

		if body, explicit, loc, committed := p.parseLogicalGroup(true, prefix); committed {
			if body == nil {
				return nil, false, nil
			}

			if negated {
				spanned := p.extendLoc(notLoc)
				not := NewExpr(&Not{Body: body, ExplicitBody: explicit, Location: spanned}).SetLocation(spanned)
				return NewBody(not), false, spanned
			}

			return body, explicit, loc
		}
	}

	startOffset := p.s.loc.Offset
	startLoc := p.s.Loc()
	expr := p.parseExpr()
	if expr == nil {
		return nil, false, nil
	}

	if expr.Location == nil {
		startLoc.Text = p.s.Text(startOffset, p.s.lastEnd)
		expr.SetLoc(startLoc)
	}

	if negated && p.notBodies {
		// Don't attach any existing 'with' statements, they belong to the and/or, not the negated expression.
		spanned := p.extendLoc(notLoc)
		notNode := &Not{Body: NewBody(expr), Location: spanned}
		expr = NewExpr(notNode).SetLocation(spanned)
	} else if negated {
		expr.Negated = true
	}

	return NewBody(expr), false, expr.Location
}

// isAmbiguousUnionBody reports whether b is a single-expression body holding a
// bare infix `|` set union. Written that way, `{ ... | ... }` cannot be told apart
// from a set comprehension; the call form (`or(x, y)`) and the parenthesized form
// (`(x | y)`) can, and are left alone.
func isAmbiguousUnionBody(b Body) bool {
	if len(b) == 0 {
		return false
	}

	// The first expression decides: `{A | B; C}` also reads as a comprehension with
	// head A and body `B; C`, so trailing expressions don't disambiguate anything.
	terms, ok := b[0].Terms.([]*Term)
	if !ok || !Interned.Refs.Or.Equal(b[0].Operator()) {
		return false
	}

	// The operator's text is `|` for the infix form and `or` for the call form.
	if terms[0].Location == nil || string(terms[0].Location.Text) != "|" {
		return false
	}

	return b[0].Location == nil || !bytes.HasPrefix(bytes.TrimSpace(b[0].Location.Text), []byte("("))
}

// errorOperandBraceNeedsBody reports `{...}` in an operand position holding a value instead of expressions.
func (p *Parser) errorOperandBraceNeedsBody(loc *Location, operand []byte, term *Term, prefix string) {
	p.hint(fmt.Sprintf("write `%s(%s)` to negate the value, or `%s{%s}` for a body holding it",
		prefix, operand, prefix, operand))
	p.errorf(loc, "`{...}` in an operand position must contain expression(s), got: %s", ValueName(braceLedValue(term)))
}

// braceLedValue returns the value opened by the leading `{` of t. An infix call
// renders its lhs operand first, so `{1, 2} & s` is brace-led by the set; refs are
// left alone, as `{"a": 1}["a"]` is reported as the ref it is.
func braceLedValue(t *Term) Value {
	if call, ok := t.Value.(Call); ok && len(call) > 0 {
		if bi, ok := BuiltinMap[call[0].String()]; ok && bi.Infix != "" && len(call) == bi.Decl.Arity()+1 {
			return braceLedValue(call[1])
		}
	}

	return t.Value
}

// isEmptyObjectTerm reports whether expr is exactly `{}`. In an operand position
// those braces open a body, so an empty one is an empty body - not the empty
// object the term parser read.
func isEmptyObjectTerm(expr *Expr) bool {
	if len(expr.With) > 0 {
		return false
	}

	t, ok := expr.Terms.(*Term)
	if !ok {
		return false
	}

	obj, ok := t.Value.(Object)

	return ok && obj.Len() == 0
}

// errorBraceLedOperand reports an `and`/`or` operand whose leading `{` opens a
// value rather than a body. In an operand position the braces are read as an
// explicit body, so the value form has to be parenthesized, on both sides of the
// operator.
func (p *Parser) errorBraceLedOperand(loc *Location, operand []byte, op string) {
	p.hint(fmt.Sprintf("wrap the operand to keep the value: `(%s) %s ...`", operand, op))
	p.errorf(loc, "operand of `%s` cannot begin with `{` unless the braces hold a body", op)
}

func (p *Parser) checkVoidCallOperands(lhs, rhs Body, op string) {
	if name, loc := voidCallOperand(lhs); name != "" {
		p.errorVoidCallOperand(loc, name, op)
	}

	if name, loc := voidCallOperand(rhs); name != "" {
		p.errorVoidCallOperand(loc, name, op)
	}
}

func (p *Parser) errorVoidCallOperand(loc *Location, name, op string) {
	p.hint(fmt.Sprintf("`%s` produces no value and always succeeds, so the operand can never fail; move it out of the operand, or add an expression that can fail", name))
	p.errorf(loc, "operand of `%s` cannot consist only of calls to `%s`", op, name)
}

// voidCallOperand returns the name and location of the first void builtin called by
// an operand whose body does nothing else; negated operands are left alone.
func voidCallOperand(body Body) (string, *Location) {
	var name string
	var loc *Location

	for _, expr := range body {
		if expr.Negated || !expr.IsCall() {
			return "", nil
		}

		bi, ok := BuiltinMap[expr.Operator().String()]
		if !ok || bi.Decl == nil || bi.Decl.Result() != nil {
			return "", nil
		}

		if name == "" {
			name, loc = bi.Name, expr.Location
		}
	}

	return name, loc
}

// errorParensCannotWrapBody reports `(...)` holding expressions rather than a value.
func (p *Parser) errorParensCannotWrapBody(loc *Location, braces []byte, prefix string) {
	p.hint(fmt.Sprintf("drop the parens to keep the body: `%s%s`", prefix, braces))
	p.error(loc, "`(...)` in an operand position cannot contain a body")
}

func (p *Parser) errorAmbiguousUnionBody(loc *Location, braceOffset int, body Body, prefix string) {
	braces := p.s.Text(braceOffset, p.s.lastEnd)

	// Parenthesizing only the union keeps any trailing expressions of the body.
	union := string(braces)
	if e := body[0].Location; e != nil {
		if rel := e.Offset - braceOffset; rel > 0 && rel+len(e.Text) <= len(braces) {
			union = fmt.Sprintf("%s(%s)%s", braces[:rel], e.Text, braces[rel+len(e.Text):])
		}
	}

	p.hint(fmt.Sprintf("write `%s(%s)` for the comprehension, or `%s%s` for the set union",
		prefix, braces, prefix, union))
	p.error(loc, "ambiguous `{ ... | ... }` operand: read as a body holding a set-union expression, not as a comprehension")
}

// isLogicalBody reports whether b is a single-expression body wrapping a
// LogicalAnd/LogicalOr node, i.e. the result of a parenthesized or nested group.
func isLogicalBody(b Body) bool {
	if len(b) != 1 {
		return false
	}
	switch b[0].Terms.(type) {
	case *LogicalAnd, *LogicalOr:
		return true
	}
	return false
}

// isNegatedOperand reports whether b is a single negated operand, e.g. `not a`
// (either a *Not node or an expression with Negated set).
func isNegatedOperand(b Body) bool {
	if len(b) != 1 {
		return false
	}

	if b[0].Negated {
		return true
	}

	_, ok := b[0].Terms.(*Not)
	return ok
}

// expectRParen consumes the closing `)` of a group, reporting an error if the
// current token is not `)`.
func (p *Parser) expectRParen() bool {
	if p.s.tok != tokens.RParen {
		p.error(p.s.Loc(), "expected ) to close parenthesized group")
		return false
	}
	p.scan()
	return true
}

// parseLogicalGroup attempts to parse a parenthesized grouping of `and`/`or`/`not`
// operands starting at the current `(`.
//
// operandContext reports whether the `(` is already an operand of `and`/`or`/`not`.
func (p *Parser) parseLogicalGroup(operandContext bool, prefix string) (Body, bool, *Location, bool) {
	if !p.enter() {
		return nil, false, nil, true
	}
	defer p.leave()

	s := p.save()
	openLoc := p.s.Loc()
	p.scan() // consume `(`

	if p.s.tok == tokens.RParen {
		if operandContext {
			p.error(openLoc, "empty parenthesized group")
			return nil, false, nil, true
		}
		p.restore(s)
		return nil, false, nil, false
	}

	lhsBody, lhsExplicit, lhsLoc := p.parseLogicalOperand("")
	if lhsBody == nil {
		// Parens are not an operand, so a `{...}` that can't be a body is a value:
		// restore and let the term parser read it, e.g. `not ({})` is an empty object.
		p.restore(s)

		return nil, false, nil, false
	}

	switch {
	case p.s.tok == tokens.LogicalAnd || p.s.tok == tokens.LogicalOr:
		expr := p.parseLogicalOrChain(lhsBody, lhsExplicit, lhsLoc)
		if expr == nil {
			return nil, false, nil, true
		}

		// A trailing `with` binds to the whole group, e.g. `(a and b with x)`.
		if expr = p.attachWith(expr); expr == nil {
			return nil, false, nil, true
		}

		if !p.expectRParen() {
			return nil, false, nil, true
		}

		return NewBody(expr), false, p.extendLoc(openLoc), true

	case p.s.tok == tokens.With && !lhsExplicit && len(lhsBody) == 1:
		// Single-operand group carrying a `with`, e.g. `(a with x)`; the `with`
		// binds to the sole operand.
		withLoc := p.s.Loc()
		if p.attachWith(lhsBody[0]) == nil {
			return nil, false, nil, true
		}

		// A `with` on the operand followed by `and`/`or` is ambiguous.
		if p.s.tok == tokens.LogicalAnd || p.s.tok == tokens.LogicalOr {
			p.errWithOnOperand(withLoc, p.s.tok.String())
			return nil, false, nil, true
		}

		if !p.expectRParen() {
			return nil, false, nil, true
		}

		if operandContext || p.s.tok == tokens.LogicalAnd || p.s.tok == tokens.LogicalOr {
			return lhsBody, false, p.extendLoc(openLoc), true
		}

		p.restore(s)
		return nil, false, nil, false

	case lhsExplicit:
		// `({ body })`: parens don't wrap a body. Without a top-level `and`/`or`
		// -- handled above -- the braces are a value, so restore and let the term
		// parser read them.
		braces := p.s.Text(lhsLoc.Offset, p.s.lastEnd)
		p.restore(s)

		probe := p.save()
		p.scan() // consume `(`
		term := p.parseTerm()
		p.restore(probe)

		if term == nil {
			p.errorParensCannotWrapBody(openLoc, braces, prefix)
			return nil, false, nil, true
		}

		return nil, false, nil, false

	case isLogicalBody(lhsBody):
		// `(( ... ))`; redundant parens around a nested group.
		if !p.expectRParen() {
			return nil, false, nil, true
		}
		return lhsBody, false, p.extendLoc(openLoc), true

	case isNegatedOperand(lhsBody):
		// `(not ...)`
		if !p.expectRParen() {
			return nil, false, nil, true
		}
		return lhsBody, false, p.extendLoc(openLoc), true

	default:
		// Single non-logical operand, e.g. `(a == b)`: not a group.
		p.restore(s)
		return nil, false, nil, false
	}
}

func (p *Parser) parseEvery() *Expr {
	qb := &Every{}
	qb.SetLoc(p.s.Loc())

	// TODO(sr): We'd get more accurate error messages if we didn't rely on
	// parseTermInfixCall here, but parsed "var [, var] in term" manually.
	p.scan()
	term := p.parseTermInfixCall()
	if term == nil {
		return nil
	}
	call, ok := term.Value.(Call)
	if !ok {
		p.illegal("expected `x[, y] in xs { ... }` expression")
		return nil
	}
	switch call[0].String() {
	case Member.Name: // x in xs
		if len(call) != 3 {
			p.illegal("illegal domain")
			return nil
		}
		qb.Value = call[1]
		qb.Domain = call[2]
	case MemberWithKey.Name: // k, v in xs
		if len(call) != 4 {
			p.illegal("illegal domain")
			return nil
		}
		qb.Key = call[1]
		qb.Value = call[2]
		qb.Domain = call[3]
		if _, ok := qb.Key.Value.(Var); !ok {
			p.illegal("expected key to be a variable")
			return nil
		}
	default:
		p.illegal("expected `x[, y] in xs { ... }` expression")
		return nil
	}
	if _, ok := qb.Value.Value.(Var); !ok {
		p.illegal("expected value to be a variable")
		return nil
	}
	if p.s.tok == tokens.LBrace { // every x in xs { ... }
		p.scan()
		body := p.parseBody(tokens.RBrace)
		if body == nil {
			return nil
		}
		p.scan()
		qb.Body = body
		expr := NewExpr(qb).SetLocation(qb.Location)

		if p.s.tok == tokens.With {
			if expr.With = p.parseWith(); expr.With == nil {
				return nil
			}
		}
		return expr
	}

	p.illegal("missing body")
	return nil
}

func (p *Parser) parseExpr() *Expr {

	lhs := p.parseTermInfixCall()
	if lhs == nil {
		return nil
	}

	if op := p.parseTermOp(tokens.Assign, tokens.Unify); op != nil {
		if rhs := p.parseTermInfixCall(); rhs != nil {
			return NewExpr([]*Term{op, lhs, rhs})
		}
		return nil
	}

	// NOTE(tsandall): the top-level call term is converted to an expr because
	// the evaluator does not support the call term type (nested calls are
	// rewritten by the compiler.)
	if call, ok := lhs.Value.(Call); ok {
		return NewExpr([]*Term(call))
	}

	return NewExpr(lhs)
}

// parseTermInfixCall consumes the next term from the input and returns it. If a
// term cannot be parsed the return value is nil and error will be recorded. The
// scanner will be advanced to the next token before returning.
// By starting out with infix relations (==, !=, <, etc) and further calling the
// other binary operators (|, &, arithmetics), it constitutes the binding
// precedence.
func (p *Parser) parseTermInfixCall() *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	return p.parseTermIn(nil, true, p.s.loc.Offset)
}

func (p *Parser) parseTermInfixCallInList() *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	return p.parseTermIn(nil, false, p.s.loc.Offset)
}

func (p *Parser) parseTermIn(lhs *Term, keyVal bool, offset int) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	// NOTE(sr): `in` is a bit special: besides `lhs in rhs`, it also
	// supports `key, val in rhs`, so it can have an optional second lhs.
	// `keyVal` triggers if we attempt to parse a second lhs argument (`mhs`).
	if lhs == nil {
		lhs = p.parseTermRelation(nil, offset)
	}
	if lhs != nil {
		if keyVal && p.s.tok == tokens.Comma { // second "lhs", or "middle hand side"
			s := p.save()
			p.scan()
			if mhs := p.parseTermRelation(nil, offset); mhs != nil {

				if op := p.parseTermOpName(Interned.Refs.MemberWithKey, tokens.In); op != nil {
					if rhs := p.parseTermRelation(nil, p.s.loc.Offset); rhs != nil {
						call := p.setLoc(CallTerm(op, lhs, mhs, rhs), lhs.Location, offset, p.s.lastEnd)
						switch p.s.tok {
						case tokens.In:
							return p.parseTermIn(call, keyVal, offset)
						default:
							return call
						}
					}
				}
			}
			p.restore(s)
		}

		_ = scanAheadRef(p)

		if op := p.parseTermOpName(Interned.Refs.Member, tokens.In); op != nil {
			if rhs := p.parseTermRelation(nil, p.s.loc.Offset); rhs != nil {
				call := p.setLoc(CallTerm(op, lhs, rhs), lhs.Location, offset, p.s.lastEnd)
				switch p.s.tok {
				case tokens.In:
					return p.parseTermIn(call, keyVal, offset)
				default:
					return call
				}
			}
		}
	}
	return lhs
}

func (p *Parser) parseTermRelation(lhs *Term, offset int) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	if lhs == nil {
		lhs = p.parseTermOr(nil, offset)
	}
	if lhs != nil {
		if op := p.parseTermOp(tokens.Equal, tokens.Neq, tokens.Lt, tokens.Gt, tokens.Lte, tokens.Gte); op != nil {
			if rhs := p.parseTermOr(nil, p.s.loc.Offset); rhs != nil {
				call := p.setLoc(CallTerm(op, lhs, rhs), lhs.Location, offset, p.s.lastEnd)
				switch p.s.tok {
				case tokens.Equal, tokens.Neq, tokens.Lt, tokens.Gt, tokens.Lte, tokens.Gte:
					return p.parseTermRelation(call, offset)
				default:
					return call
				}
			}
		}
	}
	return lhs
}

func (p *Parser) parseTermOr(lhs *Term, offset int) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	if lhs == nil {
		lhs = p.parseTermAnd(nil, offset)
	}
	if lhs != nil {
		if op := p.parseTermOp(tokens.Or); op != nil {
			if rhs := p.parseTermAnd(nil, p.s.loc.Offset); rhs != nil {
				call := p.setLoc(CallTerm(op, lhs, rhs), lhs.Location, offset, p.s.lastEnd)
				switch p.s.tok {
				case tokens.Or:
					return p.parseTermOr(call, offset)
				default:
					return call
				}
			}
		}
		return lhs
	}
	return nil
}

func (p *Parser) parseTermAnd(lhs *Term, offset int) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	if lhs == nil {
		lhs = p.parseTermArith(nil, offset)
	}
	if lhs != nil {
		if op := p.parseTermOp(tokens.And); op != nil {
			if rhs := p.parseTermArith(nil, p.s.loc.Offset); rhs != nil {
				call := p.setLoc(CallTerm(op, lhs, rhs), lhs.Location, offset, p.s.lastEnd)
				switch p.s.tok {
				case tokens.And:
					return p.parseTermAnd(call, offset)
				default:
					return call
				}
			}
		}
		return lhs
	}
	return nil
}

func (p *Parser) parseTermArith(lhs *Term, offset int) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	if lhs == nil {
		lhs = p.parseTermFactor(nil, offset)
	}
	if lhs != nil {
		if op := p.parseTermOp(tokens.Add, tokens.Sub); op != nil {
			if rhs := p.parseTermFactor(nil, p.s.loc.Offset); rhs != nil {
				call := p.setLoc(CallTerm(op, lhs, rhs), lhs.Location, offset, p.s.lastEnd)
				switch p.s.tok {
				case tokens.Add, tokens.Sub:
					return p.parseTermArith(call, offset)
				default:
					return call
				}
			}
		}
	}
	return lhs
}

func (p *Parser) parseTermFactor(lhs *Term, offset int) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	if lhs == nil {
		lhs = p.parseTerm()
	}
	if lhs != nil {
		if op := p.parseTermOp(tokens.Mul, tokens.Quo, tokens.Rem); op != nil {
			if rhs := p.parseTerm(); rhs != nil {
				call := p.setLoc(CallTerm(op, lhs, rhs), lhs.Location, offset, p.s.lastEnd)
				switch p.s.tok {
				case tokens.Mul, tokens.Quo, tokens.Rem:
					return p.parseTermFactor(call, offset)
				default:
					return call
				}
			}
		}
	}
	return lhs
}

func (p *Parser) parseTerm() *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	if term, s := p.parsedTermCacheLookup(); s != nil {
		p.restore(s)
		return term
	}
	s0 := p.save()

	var term *Term
	var unaryMinusLoc *Location

	// Check if an `and`/`or` token is actually a function call (`&`/`|` set built-ins).
	scanAheadLogicalCall(p)

	switch p.s.tok {
	case tokens.Null:
		term = NullTerm().SetLocation(p.s.Loc())
	case tokens.True:
		term = BooleanTerm(true).SetLocation(p.s.Loc())
	case tokens.False:
		term = BooleanTerm(false).SetLocation(p.s.Loc())
	case tokens.Sub:
		loc := p.s.Loc()
		s := p.save()
		p.scan()
		if p.s.tok == tokens.Ident || p.s.tok == tokens.Contains {
			// Unary minus on a reference: -ref → minus(0, ref).
			// parseTermFinish below will resolve the full ref (e.g. input.number),
			// after which we wrap the result in a minus call.
			unaryMinusLoc = loc
			term = p.parseVar()
		} else {
			p.restore(s)
			term = p.parseNumber()
		}
	case tokens.Dot, tokens.Number:
		term = p.parseNumber()
	case tokens.String:
		term = p.parseString()
	case tokens.TemplateStringPart, tokens.TemplateStringEnd:
		term = p.parseTemplateString(false)
	case tokens.RawTemplateStringPart, tokens.RawTemplateStringEnd:
		term = p.parseTemplateString(true)
	case tokens.Ident, tokens.Contains: // NOTE(sr): contains anywhere BUT in rule heads gets no special treatment
		term = p.parseVar()
	case tokens.LBrack:
		term = p.parseArray()
	case tokens.LBrace:
		term = p.parseSetOrObject()
	case tokens.LParen:
		offset := p.s.loc.Offset
		p.scan()
		if r := p.parseTermInfixCall(); r != nil {
			if p.s.tok == tokens.RParen {
				r.Location.Text = p.s.Text(offset, p.s.tokEnd)
				term = r
			} else {
				p.error(p.s.Loc(), "non-terminated expression")
			}
		}
	default:
		p.illegalToken()
	}

	term = p.parseTermFinish(term, false)
	if unaryMinusLoc != nil && term != nil {
		zero := IntNumberTerm(0).SetLocation(unaryMinusLoc)
		term = p.setLoc(Minus.Call(zero, term), unaryMinusLoc, unaryMinusLoc.Offset, p.s.lastEnd)
	}
	p.parsedTermCachePush(term, s0)
	return term
}

func (p *Parser) parseTermFinish(head *Term, skipws bool) *Term {
	if head == nil {
		return nil
	}
	offset := p.s.loc.Offset
	p.doScan(skipws, noScanOptions...)

	switch p.s.tok {
	case tokens.LParen, tokens.Dot, tokens.LBrack:
		return p.parseRef(head, offset)
	case tokens.Whitespace:
		p.scan()
		fallthrough
	default:
		if _, ok := head.Value.(Var); ok && RootDocumentNames.Contains(head) {
			return RefTerm(head).SetLocation(head.Location)
		}
		return head
	}
}

func (p *Parser) parseHeadFinish(head *Term, skipws bool) *Term {
	if head == nil {
		return nil
	}
	offset := p.s.loc.Offset
	p.scanWS()

	switch p.s.tok {
	case tokens.Add, tokens.Sub, tokens.Mul, tokens.Quo, tokens.Rem,
		tokens.And, tokens.Or,
		tokens.Equal, tokens.Neq, tokens.Gt, tokens.Gte, tokens.Lt, tokens.Lte:
		p.illegalToken()
	case tokens.Whitespace:
		p.doScan(skipws, noScanOptions...)
	}

	switch p.s.tok {
	case tokens.LParen, tokens.Dot, tokens.LBrack:
		return p.parseRef(head, offset)
	case tokens.Whitespace:
		p.scan()
	}

	if _, ok := head.Value.(Var); ok && RootDocumentNames.Contains(head) {
		return RefTerm(head).SetLocation(head.Location)
	}
	return head
}

func (p *Parser) parseNumber() *Term {
	var prefix string
	loc := p.s.Loc()

	// Handle negative sign
	if p.s.tok == tokens.Sub {
		prefix = "-"
		p.scan()
		switch p.s.tok {
		case tokens.Number, tokens.Dot:
			break
		default:
			p.illegal("expected number")
			return nil
		}
	}

	// Handle decimal point
	if p.s.tok == tokens.Dot {
		prefix += "."
		p.scan()
		if p.s.tok != tokens.Number {
			p.illegal("expected number")
			return nil
		}
	}

	// Validate leading zeros: reject numbers like "01", "007", etc.
	// Skip validation if prefix ends with '.' (like ".123")
	hasDecimalPrefix := len(prefix) > 0 && prefix[len(prefix)-1] == '.'

	if !hasDecimalPrefix && len(p.s.lit) > 1 && p.s.lit[0] == '0' {
		// These are the only valid cases starting with '0':
		isDecimal := p.s.lit[1] == '.'                                               // "0.123"
		isScientific := len(p.s.lit) > 2 && (p.s.lit[1] == 'e' || p.s.lit[1] == 'E') // "0e5", "0E-3"

		if !isDecimal && !isScientific {
			p.illegal("expected number without leading zero")
			return nil
		}
	}

	// Ensure that the number is valid
	s := prefix + p.s.lit
	f, ok := new(big.Float).SetString(s)
	if !ok {
		p.illegal("invalid float")
		return nil
	}

	// Put limit on size of exponent to prevent non-linear cost of String()
	// function on big.Float from causing denial of service: https://github.com/golang/go/issues/11068
	//
	// n == sign * mantissa * 2^exp
	// 0.5 <= mantissa < 1.0
	//
	// The limit is arbitrary.
	exp := f.MantExp(nil)
	if exp > 1e5 || exp < -1e5 || f.IsInf() { // +/- inf, exp is 0
		p.error(p.s.Loc(), "number too big")
		return nil
	}

	// Note: Use the original string, do *not* round trip from
	// the big.Float as it can cause precision loss.
	return NumberTerm(json.Number(s)).SetLocation(loc)
}

func (p *Parser) parseString() *Term {
	if p.s.lit[0] == '"' {
		if p.s.lit == "\"\"" {
			return NewTerm(InternedEmptyStringValue).SetLocation(p.s.Loc())
		}

		inner := p.s.lit[1 : len(p.s.lit)-1]
		if !strings.ContainsRune(inner, '\\') { // nothing to un-escape
			return StringTerm(inner).SetLocation(p.s.Loc())
		}

		var s string
		if err := json.Unmarshal([]byte(p.s.lit), &s); err != nil {
			p.errorf(p.s.Loc(), "illegal string literal: %s", p.s.lit)
			return nil
		}
		return StringTerm(s).SetLocation(p.s.Loc())
	}
	return p.parseRawString()
}

func (p *Parser) parseRawString() *Term {
	if len(p.s.lit) < 2 {
		return nil
	}
	return StringTerm(p.s.lit[1 : len(p.s.lit)-1]).SetLocation(p.s.Loc())
}

func templateStringPartToStringLiteral(tok tokens.Token, lit string) (string, error) {
	switch tok {
	case tokens.TemplateStringPart, tokens.TemplateStringEnd:
		inner := lit[1 : len(lit)-1]
		if !strings.ContainsRune(inner, '\\') { // nothing to un-escape
			return inner, nil
		}

		buf := make([]byte, 0, len(inner)+2)
		buf = append(buf, '"')
		buf = append(buf, inner...)
		buf = append(buf, '"')
		var s string
		if err := json.Unmarshal(buf, &s); err != nil {
			return "", fmt.Errorf("illegal template-string part: %s", lit)
		}
		return s, nil
	case tokens.RawTemplateStringPart, tokens.RawTemplateStringEnd:
		return lit[1 : len(lit)-1], nil
	default:
		return "", errors.New("expected template-string part")
	}
}

func (p *Parser) parseTemplateString(multiLine bool) *Term {
	loc := p.s.Loc()

	if !p.po.Capabilities.ContainsFeature(FeatureTemplateStrings) {
		p.errorf(loc, "template strings are not supported by current capabilities")
		return nil
	}

	var parts []Node

	for {
		s, err := templateStringPartToStringLiteral(p.s.tok, p.s.lit)
		if err != nil {
			p.error(p.s.Loc(), err.Error())
			return nil
		}

		// Don't add empty strings
		if len(s) > 0 {
			parts = append(parts, StringTerm(s).SetLocation(p.s.Loc()))
		}

		if p.s.tok == tokens.TemplateStringEnd || p.s.tok == tokens.RawTemplateStringEnd {
			break
		}

		numCommentsBefore := len(p.s.comments)
		p.scan()
		numCommentsAfter := len(p.s.comments)

		expr := p.parseLiteral()
		if expr == nil {
			p.error(p.s.Loc(), "invalid template-string expression")
			return nil
		}

		if expr.Negated {
			p.errorf(expr.Loc(), "unexpected negation ('%s') in template-string expression", tokens.KeywordFor(tokens.Not))
			return nil
		}

		// Note: Actually unification
		if expr.IsEquality() {
			p.errorf(expr.Loc(), "unexpected unification ('=') in template-string expression")
			return nil
		}

		if expr.IsAssignment() {
			p.errorf(expr.Loc(), "unexpected assignment (':=') in template-string expression")
			return nil
		}

		if expr.IsEvery() {
			p.errorf(expr.Loc(), "unexpected '%s' in template-string expression", tokens.KeywordFor(tokens.Every))
			return nil
		}

		if expr.IsSome() {
			p.errorf(expr.Loc(), "unexpected '%s' in template-string expression", tokens.KeywordFor(tokens.Some))
			return nil
		}

		// FIXME: Can we optimize for collections and comprehensions too? To qualify, they must not contain refs or calls.
		var nonOptional bool
		if term, ok := expr.Terms.(*Term); ok && numCommentsAfter == numCommentsBefore {
			switch term.Value.(type) {
			case String, Number, Boolean, Null:
				nonOptional = true
				parts = append(parts, term)
			}
		}

		if !nonOptional {
			parts = append(parts, expr)
		}

		if p.s.tok != tokens.RBrace {
			p.errorf(p.s.Loc(), "expected %s to end template string expression", tokens.RBrace)
			return nil
		}

		p.doScan(false, scanner.ContinueTemplateString(multiLine))
	}

	// When there are template-expressions, the initial location will only contain the text up to the first expression
	loc.Text = p.s.Text(loc.Offset, p.s.tokEnd)

	return TemplateStringTerm(multiLine, parts...).SetLocation(loc)
}

func (p *Parser) parseCall(operator *Term, offset int) (term *Term) {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	loc := operator.Location
	var end int

	defer func() {
		p.setLoc(term, loc, offset, end)
	}()

	p.scan() // steps over '('

	if p.s.tok == tokens.RParen { // no args, i.e. set() or any.func()
		end = p.s.tokEnd
		p.scanWS()
		if operator.Equal(setConstructor) {
			return SetTerm()
		}
		return CallTerm(operator)
	}

	if r := p.parseTermList(tokens.RParen, []*Term{operator}); r != nil {
		end = p.s.tokEnd
		p.scanWS()
		return CallTerm(r...)
	}

	return nil
}

func (p *Parser) parseRef(head *Term, offset int) (term *Term) {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	loc := head.Location
	var end int

	defer func() {
		p.setLoc(term, loc, offset, end)
	}()

	switch h := head.Value.(type) {
	case Var, *Array, Object, Set, *ArrayComprehension, *ObjectComprehension, *SetComprehension, Call:
		// ok
	default:
		p.errorf(loc, "illegal ref (head cannot be %v)", ValueName(h))
	}

	ref := []*Term{head}

	for {
		switch p.s.tok {
		case tokens.Dot:
			p.scanWS()
			if p.s.tok != tokens.Ident && !p.isAllowedRefKeyword(p.s.tok) {
				p.illegal("expected %v", tokens.Ident)
				return nil
			}
			ref = append(ref, StringTerm(p.s.lit).SetLocation(p.s.Loc()))
			p.scanWS()
		case tokens.LParen:
			term = p.parseCall(p.setLoc(RefTerm(ref...), loc, offset, p.s.loc.Offset), offset)
			if term != nil {
				switch p.s.tok {
				case tokens.Whitespace:
					p.scan()
					end = p.s.lastEnd
					return term
				case tokens.Dot, tokens.LBrack:
					term = p.parseRef(term, offset)
				}
			}
			end = p.s.lastEnd
			return term
		case tokens.LBrack:
			p.scan()
			if term := p.parseTermInfixCall(); term != nil {
				if p.s.tok != tokens.RBrack {
					p.illegal("expected %v", tokens.LBrack)
					return nil
				}
				ref = append(ref, term)
				p.scanWS()
			} else {
				return nil
			}
		case tokens.Whitespace:
			end = p.s.lastEnd
			p.scan()
			return RefTerm(ref...)
		default:
			end = p.s.lastEnd
			return RefTerm(ref...)
		}
	}
}

func (p *Parser) parseArray() (term *Term) {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	loc := p.s.Loc()
	offset := p.s.loc.Offset

	defer func() {
		p.setLoc(term, loc, offset, p.s.tokEnd)
	}()

	p.scan()

	if p.s.tok == tokens.RBrack {
		return ArrayTerm()
	}

	potentialComprehension := true

	// Skip leading commas, eg [, x, y]
	// Supported for backwards compatibility. In the future
	// we should make this a parse error.
	if p.s.tok == tokens.Comma {
		potentialComprehension = false
		p.scan()
	}

	s := p.save()

	// NOTE(tsandall): The parser cannot attempt a relational term here because
	// of ambiguity around comprehensions. For example, given:
	//
	//  {1 | 1}
	//
	// Does this represent a set comprehension or a set containing binary OR
	// call? We resolve the ambiguity by prioritizing comprehensions.
	head := p.parseTerm()
	if head == nil {
		return nil
	}

	switch p.s.tok {
	case tokens.RBrack:
		return ArrayTerm(head)
	case tokens.Comma:
		p.scan()
		if terms := p.parseTermList(tokens.RBrack, []*Term{head}); terms != nil {
			return ArrayTerm(terms...)
		}
		return nil
	case tokens.Or:
		if potentialComprehension {
			// Try to parse as if it is an array comprehension
			p.scan()
			if body := p.parseBody(tokens.RBrack); body != nil {
				return ArrayComprehensionTerm(head, body)
			}
			if p.s.tok != tokens.Comma {
				return nil
			}
		}
		// fall back to parsing as a normal array definition
	}

	p.restore(s)

	if terms := p.parseTermList(tokens.RBrack, nil); terms != nil {
		return ArrayTerm(terms...)
	}
	return nil
}

func (p *Parser) parseSetOrObject() (term *Term) {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	loc := p.s.Loc()
	offset := p.s.loc.Offset

	defer func() {
		p.setLoc(term, loc, offset, p.s.tokEnd)
	}()

	p.scan()

	if p.s.tok == tokens.RBrace {
		return ObjectTerm()
	}

	potentialComprehension := true

	// Skip leading commas, eg {, x, y}
	// Supported for backwards compatibility. In the future
	// we should make this a parse error.
	if p.s.tok == tokens.Comma {
		potentialComprehension = false
		p.scan()
	}

	s := p.save()

	// Try parsing just a single term first to give comprehensions higher
	// priority to "or" calls in ambiguous situations. Eg: { a | b }
	// will be a set comprehension.
	//
	// Note: We don't know yet if it is a set or object being defined.
	head := p.parseTerm()
	if head == nil {
		return nil
	}

	switch p.s.tok {
	case tokens.Or:
		if potentialComprehension {
			return p.parseSet(s, head, potentialComprehension)
		}
	case tokens.RBrace, tokens.Comma:
		return p.parseSet(s, head, potentialComprehension)
	case tokens.Colon:
		return p.parseObject(head, potentialComprehension)
	}

	p.restore(s)

	head = p.parseTermInfixCallInList()
	if head == nil {
		return nil
	}

	switch p.s.tok {
	case tokens.RBrace, tokens.Comma:
		return p.parseSet(s, head, false)
	case tokens.Colon:
		// It still might be an object comprehension, eg { a+1: b | ... }
		return p.parseObject(head, potentialComprehension)
	}

	p.illegal("non-terminated set")
	return nil
}

func (p *Parser) parseSet(s *state, head *Term, potentialComprehension bool) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	switch p.s.tok {
	case tokens.RBrace:
		return SetTerm(head)
	case tokens.Comma:
		p.scan()
		if terms := p.parseTermList(tokens.RBrace, []*Term{head}); terms != nil {
			return SetTerm(terms...)
		}
	case tokens.Or:
		if potentialComprehension {
			// Try to parse as if it is a set comprehension
			p.scan()
			if body := p.parseBody(tokens.RBrace); body != nil {
				return SetComprehensionTerm(head, body)
			}
			if p.s.tok != tokens.Comma {
				return nil
			}
		}
		// Fall back to parsing as normal set definition
		p.restore(s)
		if terms := p.parseTermList(tokens.RBrace, nil); terms != nil {
			return SetTerm(terms...)
		}
	}
	return nil
}

func (p *Parser) parseObject(k *Term, potentialComprehension bool) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	// NOTE(tsandall): Assumption: this function is called after parsing the key
	// of the head element and then receiving a colon token from the scanner.
	// Advance beyond the colon and attempt to parse an object.
	if p.s.tok != tokens.Colon {
		panic("expected colon")
	}
	p.scan()

	s := p.save()

	// NOTE(sr): We first try to parse the value as a term (`v`), and see
	// if we can parse `{ x: v | ...}` as a comprehension.
	// However, if we encounter either a Comma or an RBace, it cannot be
	// parsed as a comprehension -- so we save double work further down
	// where `parseObjectFinish(k, v, false)` would only exercise the
	// same code paths once more.
	v := p.parseTerm()
	if v == nil {
		return nil
	}

	potentialRelation := true
	if potentialComprehension {
		switch p.s.tok {
		case tokens.RBrace, tokens.Comma:
			potentialRelation = false
			fallthrough
		case tokens.Or:
			if term := p.parseObjectFinish(k, v, true); term != nil {
				return term
			}
		}
	}

	p.restore(s)

	if potentialRelation {
		v := p.parseTermInfixCallInList()
		if v == nil {
			return nil
		}

		switch p.s.tok {
		case tokens.RBrace, tokens.Comma:
			return p.parseObjectFinish(k, v, false)
		}
	}

	p.illegal("non-terminated object")
	return nil
}

func (p *Parser) parseObjectFinish(key, val *Term, potentialComprehension bool) *Term {
	if !p.enter() {
		return nil
	}
	defer p.leave()

	switch p.s.tok {
	case tokens.RBrace:
		return ObjectTerm([2]*Term{key, val})
	case tokens.Or:
		if potentialComprehension {
			p.scan()
			if body := p.parseBody(tokens.RBrace); body != nil {
				return ObjectComprehensionTerm(key, val, body)
			}
		} else {
			p.illegal("non-terminated object")
		}
	case tokens.Comma:
		p.scan()
		if r := p.parseTermPairList(tokens.RBrace, [][2]*Term{{key, val}}); r != nil {
			return ObjectTerm(r...)
		}
	}
	return nil
}

func (p *Parser) parseTermList(end tokens.Token, r []*Term) []*Term {
	if p.s.tok == end {
		return r
	}
	for {
		term := p.parseTermInfixCallInList()
		if term != nil {
			r = append(r, term)
			switch p.s.tok {
			case end:
				return r
			case tokens.Comma:
				p.scan()
				if p.s.tok == end {
					return r
				}
				continue
			default:
				p.illegal("expected %q or %q", tokens.Comma, end)
				return nil
			}
		}
		return nil
	}
}

func (p *Parser) parseTermPairList(end tokens.Token, r [][2]*Term) [][2]*Term {
	if p.s.tok == end {
		return r
	}
	for {
		key := p.parseTermInfixCallInList()
		if key != nil {
			switch p.s.tok {
			case tokens.Colon:
				p.scan()
				if val := p.parseTermInfixCallInList(); val != nil {
					r = append(r, [2]*Term{key, val})
					switch p.s.tok {
					case end:
						return r
					case tokens.Comma:
						p.scan()
						if p.s.tok == end {
							return r
						}
						continue
					default:
						p.illegal("expected %q or %q", tokens.Comma, end)
						return nil
					}
				}
			default:
				p.illegal("expected %q", tokens.Colon)
				return nil
			}
		}
		return nil
	}
}

func (p *Parser) parseTermOp(values ...tokens.Token) *Term {
	if slices.Contains(values, p.s.tok) {
		loc := p.s.Loc()
		r := RefTerm(VarTerm(p.s.tok.String()).SetLocation(loc)).SetLocation(loc)
		p.scan()
		return r
	}
	return nil
}

func (p *Parser) parseTermOpName(ref Ref, values ...tokens.Token) *Term {
	if slices.Contains(values, p.s.tok) {
		cp := ref.Copy()
		loc := p.s.Loc()
		for _, r := range cp {
			r.SetLocation(loc)
		}
		t := RefTerm(cp...)
		t.SetLocation(loc)
		p.scan()
		return t
	}
	return nil
}

func (p *Parser) parseVar() *Term {
	if p.s.lit == WildcardString {
		// Update wildcard values with unique identifiers
		return NewTerm(p.genwildcard()).SetLocation(p.s.Loc())
	}

	return NewTerm(InternedVarValue(p.s.lit)).SetLocation(p.s.Loc())
}

func (p *Parser) genwildcard() Value {
	var v Value
	if p.s.wildcard < len(preAllocWildcards) {
		v = preAllocWildcards[p.s.wildcard]
	} else {
		v = Var(WildcardPrefix + strconv.Itoa(p.s.wildcard))
	}
	p.s.wildcard++

	return v
}

func writeHints(msg *strings.Builder, hints []string) {
	switch len(hints) {
	case 0: // nothing to do
	case 1:
		msg.WriteString(" (hint: ")
		msg.WriteString(hints[0])
		msg.WriteByte(')')
	default:
		msg.WriteString(" (hints: ")
		for i, h := range hints {
			if i > 0 {
				msg.WriteString(", ")
			}
			msg.WriteString(h)
		}
		msg.WriteByte(')')
	}
}

func (p *Parser) error(loc *location.Location, reason string) {
	if len(p.s.hints) > 0 {
		sb := &strings.Builder{}
		sb.WriteString(reason)
		writeHints(sb, p.s.hints)
		reason = sb.String()
	}

	p.s.errors = append(p.s.errors, &Error{
		Code:     ParseErr,
		Message:  reason,
		Location: loc,
		Details:  newParserErrorDetail(p.s.s.Bytes(), loc.Offset),
	})
	p.s.hints = nil
}

func (p *Parser) errorf(loc *location.Location, f string, a ...any) {
	msg := &strings.Builder{}
	fmt.Fprintf(msg, f, a...)

	if len(p.s.hints) > 0 {
		writeHints(msg, p.s.hints)
	}

	p.s.errors = append(p.s.errors, &Error{
		Code:     ParseErr,
		Message:  msg.String(),
		Location: loc,
		Details:  newParserErrorDetail(p.s.s.Bytes(), loc.Offset),
	})
	p.s.hints = nil
}

func (p *Parser) hint(s string) {
	p.s.hints = append(p.s.hints, s)
}

func (p *Parser) illegal(note string, a ...any) {
	if p.s.tok == tokens.Illegal {
		p.errorf(p.s.Loc(), "illegal token")
		return
	}

	tok := p.s.tok.String()

	tokType := "token"
	if tokens.IsKeyword(p.s.tok) || isFutureKeywordToken(p.s.tok) {
		tokType = "keyword"
	}

	if len(note) > 0 {
		p.errorf(p.s.Loc(), "unexpected %s %s: %s", tok, tokType, fmt.Sprintf(note, a...))
	} else {
		p.errorf(p.s.Loc(), "unexpected %s %s", tok, tokType)
	}
}

func (p *Parser) illegalToken() {
	p.illegal("")
}

var noScanOptions []scanner.ScanOption

func (p *Parser) scan() {
	p.doScan(true, noScanOptions...)
}

func (p *Parser) scanWS() {
	p.doScan(false, noScanOptions...)
}

func (p *Parser) doScan(skipws bool, scanOpts ...scanner.ScanOption) {

	// NOTE(tsandall): the last position is used to compute the "text" field for
	// complex AST nodes. Whitespace never affects the last position of an AST
	// node so do not update it when scanning.
	if p.s.tok != tokens.Whitespace {
		p.s.lastEnd = p.s.tokEnd
		p.s.skippedNL = false
	}

	var errs []scanner.Error
	for {
		var pos scanner.Position
		p.s.tok, pos, p.s.lit, errs = p.s.s.Scan(scanOpts...)

		p.s.tokEnd = pos.End
		p.s.loc.Row = pos.Row
		p.s.loc.Col = pos.Col
		p.s.loc.Offset = pos.Offset
		p.s.loc.Text = p.s.Text(pos.Offset, pos.End)
		p.s.loc.Tabs = pos.Tabs

		if len(errs) > 0 {
			for _, err := range errs {
				p.error(p.s.Loc(), err.Message)
			}
			p.s.tok = tokens.Illegal
		}

		if p.s.tok == tokens.Whitespace {
			if p.s.lit == "\n" {
				p.s.skippedNL = true
			}
			if skipws {
				continue
			}
		}

		if p.s.tok != tokens.Comment {
			break
		}

		var comment *Comment
		if len(p.s.loc.Text) != 0 {
			// if location has text, use that to avoid allocating for string->[]byte
			comment = NewComment(commentFromLocText(p.s.loc.Text[1:]))
		} else {
			comment = NewComment([]byte(p.s.lit[1:]))
		}
		comment.SetLoc(p.s.Loc())
		p.s.comments = append(p.s.comments, comment)
	}
}

func commentFromLocText(commentText []byte) []byte {
	l := len(commentText)
	if l == 1 && commentText[0] == '\r' {
		commentText, l = nil, 0 // special case - remove lone '\r'
	}
	for l > 1 && commentText[l-1] == '\r' { // trim trailing '\r' until the last char
		commentText = commentText[:l-1]
		l--
	}
	return commentText
}

func (p *Parser) save() *state {
	cpy := *p.s
	s := *cpy.s
	cpy.s = &s
	return &cpy
}

func (p *Parser) restore(s *state) {
	p.s = s
}

func setLocRecursive(x any, loc *location.Location) {
	WalkNodes(x, func(n Node) bool {
		n.SetLoc(loc)
		return false
	})
}

func (p *Parser) setLoc(term *Term, loc *location.Location, offset, end int) *Term {
	if term != nil {
		cpy := *loc
		term.Location = &cpy
		term.Location.Text = p.s.Text(offset, end)
	}
	return term
}

func (p *Parser) validateDefaultRuleValue(rule *Rule) bool {
	if rule.Head.Value == nil {
		p.error(rule.Loc(), "illegal default rule (must have a value)")
		return false
	}

	valid := true
	vis := NewGenericVisitor(func(x any) bool {
		switch x.(type) {
		case *ArrayComprehension, *ObjectComprehension, *SetComprehension: // skip closures
			return true
		case Ref, Var, Call:
			p.error(rule.Loc(), fmt.Sprintf("illegal default rule (value cannot contain %v)", TypeName(x)))
			valid = false
			return true
		}
		return false
	})

	vis.Walk(rule.Head.Value.Value)
	return valid
}

func (p *Parser) validateDefaultRuleArgs(rule *Rule) bool {

	valid := true
	vars := NewVarSet()

	vis := NewGenericVisitor(func(x any) bool {
		switch x := x.(type) {
		case Var:
			if vars.Contains(x) {
				p.error(rule.Loc(), fmt.Sprintf("illegal default rule (arguments cannot be repeated %v)", x))
				valid = false
				return true
			}
			vars.Add(x)

		case *Term:
			switch v := x.Value.(type) {
			case Var: // do nothing
			default:
				p.error(rule.Loc(), fmt.Sprintf("illegal default rule (arguments cannot contain %v)", ValueName(v)))
				valid = false
				return true
			}
		}

		return false
	})

	vis.Walk(rule.Head.Args)
	return valid
}

// We explicitly use yaml unmarshalling, to accommodate for the '_' in 'related_resources',
// which isn't handled properly by json for some reason.
type rawAnnotation struct {
	Scope            string           `yaml:"scope"`
	Title            string           `yaml:"title"`
	Entrypoint       bool             `yaml:"entrypoint"`
	Description      string           `yaml:"description"`
	Organizations    []string         `yaml:"organizations"`
	RelatedResources []any            `yaml:"related_resources"`
	Authors          []any            `yaml:"authors"`
	Schemas          []map[string]any `yaml:"schemas"`
	Compile          map[string]any   `yaml:"compile"`
	Custom           map[string]any   `yaml:"custom"`
	Labels           map[string]any   `yaml:"labels"`
}

type metadataParser struct {
	comments []*Comment
	buf      *bytes.Buffer
	loc      *location.Location
}

func (b *metadataParser) Reset(loc *location.Location) {
	b.comments = b.comments[:0]
	b.loc = loc
	if b.buf != nil {
		b.buf.Reset()
	}
}

func (b *metadataParser) Append(c *Comment) {
	b.buf.Write(bytes.TrimPrefix(c.Text, []byte(" ")))
	b.buf.WriteByte('\n')
	b.comments = append(b.comments, c)
}

var yamlLineErrRegex = regexp.MustCompile(`^yaml:(?: unmarshal errors:[\n\s]*)? line ([[:digit:]]+):`)

// endLoc returns the location of the last comment in the METADATA block, or nil
// if there are none. Only this location is retained on Annotations (for
// EndLoc), so the comment slice itself is never aliased onto the result.
func endLoc(comments []*Comment) *location.Location {
	if len(comments) == 0 {
		return nil
	}
	return comments[len(comments)-1].Location
}

func (b *metadataParser) Parse() (result *Annotations, err error) {
	if len(bytes.TrimSpace(b.buf.Bytes())) == 0 {
		return nil, errors.New("expected METADATA block, found whitespace")
	}

	var raw rawAnnotation
	if err := yaml.Unmarshal(b.buf.Bytes(), &raw); err != nil {
		var comment *Comment
		match := yamlLineErrRegex.FindStringSubmatch(err.Error())
		if len(match) == 2 {
			if index, ok := util.Atoi(match[1]); ok {
				if index >= len(b.comments) {
					comment = b.comments[len(b.comments)-1]
				} else {
					comment = b.comments[index]
				}
				b.loc = comment.Location
			}
		}

		if match == nil && len(b.comments) > 0 {
			b.loc = b.comments[0].Location
		}

		return nil, augmentYamlError(err, b.comments)
	}

	result = &Annotations{
		// NOTE: only the last comment's location is retained (as endLoc); the
		// comment slice itself is backed by a reused buffer (the metadataParser
		// is pooled and Reset truncates rather than reallocates), so it must not
		// be aliased here.
		endLoc:        endLoc(b.comments),
		Scope:         raw.Scope,
		Entrypoint:    raw.Entrypoint,
		Title:         raw.Title,
		Description:   raw.Description,
		Organizations: raw.Organizations,
	}

	for _, v := range raw.RelatedResources {
		rr, err := parseRelatedResource(v)
		if err != nil {
			return nil, fmt.Errorf("invalid related-resource definition %s: %w", v, err)
		}
		result.RelatedResources = append(result.RelatedResources, rr)
	}

	if raw.Compile != nil {
		result.Compile = &CompileAnnotation{}
		if unknowns, ok := raw.Compile["unknowns"]; ok {
			if unknowns, ok := unknowns.([]any); ok {
				result.Compile.Unknowns = make([]Ref, len(unknowns))
				for i := range unknowns {
					if unknown, ok := unknowns[i].(string); ok {
						ref, err := ParseRef(unknown)
						if err != nil {
							return nil, fmt.Errorf("invalid unknowns element %q: %w", unknown, err)
						}
						result.Compile.Unknowns[i] = ref
					}
				}
			}
		}
		if mask, ok := raw.Compile["mask_rule"]; ok {
			if mask, ok := mask.(string); ok {
				maskTerm, err := ParseTerm(mask)
				if err != nil {
					return nil, fmt.Errorf("invalid mask_rule annotation %q: %w", mask, err)
				}
				switch v := maskTerm.Value.(type) {
				case Var, String:
					result.Compile.MaskRule = Ref{maskTerm}
				case Ref:
					result.Compile.MaskRule = v
				default:
					return nil, fmt.Errorf("invalid mask_rule annotation type %q: %[1]T", mask)
				}
			}
		}
	}

	for _, pair := range raw.Schemas {
		k, v := unwrapPair(pair)

		var a SchemaAnnotation
		var err error

		a.Path, err = ParseRef(k)
		if err != nil {
			return nil, errors.New("invalid document reference")
		}

		switch v := v.(type) {
		case string:
			a.Schema, err = ParseSchemaRef(v)
			if err != nil {
				return nil, err
			}
		case map[string]any:
			w, err := convertYAMLMapKeyTypes(v, nil)
			if err != nil {
				return nil, fmt.Errorf("invalid schema definition: %w", err)
			}
			a.Definition = &w
		default:
			return nil, fmt.Errorf("invalid schema declaration for path %q", k)
		}

		result.Schemas = append(result.Schemas, &a)
	}

	for _, v := range raw.Authors {
		author, err := parseAuthor(v)
		if err != nil {
			return nil, fmt.Errorf("invalid author definition %s: %w", v, err)
		}
		result.Authors = append(result.Authors, author)
	}

	if raw.Custom != nil {
		result.Custom = make(map[string]any, len(raw.Custom))
		for k, v := range raw.Custom {
			if result.Custom[k], err = convertYAMLMapKeyTypes(v, nil); err != nil {
				return nil, err
			}
		}
	}

	if raw.Labels != nil {
		result.Labels = make(map[string]any, len(raw.Labels))
		for k, v := range raw.Labels {
			if result.Labels[k], err = convertYAMLMapKeyTypes(v, nil); err != nil {
				return nil, err
			}
		}
	}

	result.Location = b.loc

	// recreate original text of entire metadata block for location text attribute
	original := bytes.TrimSuffix(b.buf.Bytes(), []byte("\n"))
	numLines := bytes.Count(original, []byte("\n")) + 1
	preAlloc := len("# METADATA\n") + len(original) + numLines*2 // '# ' prefix added per line

	result.Location.Text = append(make([]byte, 0, preAlloc), "# METADATA\n"...)

	for line := range bytes.SplitAfterSeq(original, []byte("\n")) {
		result.Location.Text = append(result.Location.Text, "# "...)
		result.Location.Text = append(result.Location.Text, line...)
	}

	return result, err
}

// augmentYamlError augments a YAML error with hints intended to help the user figure out the cause of an otherwise
// cryptic error. These are hints, instead of proper errors, because they are educated guesses, and aren't guaranteed
// to be correct.
func augmentYamlError(err error, comments []*Comment) error {
	// Adding hints for when key/value ':' separator isn't suffixed with a legal YAML space symbol
	for _, comment := range comments {
		if bytes.IndexByte(comment.Text, ':') == -1 {
			continue
		}
		parts := bytes.Split(comment.Text, []byte{':'})[1:]

		var invalidSpaces []string
		for partIndex, part := range parts {
			if len(part) == 0 && partIndex == len(parts)-1 {
				break
			}

			r, _ := utf8.DecodeRune(part)
			if r == ' ' || r == '\t' {
				break
			}

			invalidSpaces = append(invalidSpaces, fmt.Sprintf("%+q", r))
		}
		if len(invalidSpaces) > 0 {
			err = fmt.Errorf(
				"%s\n  Hint: on line %d, symbol(s) %v immediately following a"+
					" key/value separator ':' is not a legal yaml space character",
				err.Error(), comment.Location.Row, invalidSpaces)
		}
	}
	return err
}

func unwrapPair(pair map[string]any) (string, any) {
	for k, v := range pair {
		return k, v
	}
	return "", nil
}

var errInvalidSchemaRef = errors.New("invalid schema reference")

// ParseSchemaRef parses a schema reference string into a Ref. Unlike
// ParseRef, it accepts the bare `schema` Var and Refs prefixed with the
// schema root document.
//
// NOTE(tsandall): 'schema' is not registered as a root because it's not
// supported by the compiler or evaluator today. Once we fix that, we can remove
// this function.
func ParseSchemaRef(s string) (Ref, error) {

	term, err := ParseTerm(s)
	if err == nil {
		switch v := term.Value.(type) {
		case Var:
			if term.Equal(SchemaRootDocument) {
				return SchemaRootRef.Copy(), nil
			}
		case Ref:
			if v.HasPrefix(SchemaRootRef) {
				return v, nil
			}
		}
	}

	return nil, errInvalidSchemaRef
}

func parseRelatedResource(rr any) (*RelatedResourceAnnotation, error) {
	rr, err := convertYAMLMapKeyTypes(rr, nil)
	if err != nil {
		return nil, err
	}

	switch rr := rr.(type) {
	case string:
		if len(rr) > 0 {
			u, err := url.Parse(rr)
			if err != nil {
				return nil, err
			}
			return &RelatedResourceAnnotation{Ref: *u}, nil
		}
		return nil, errors.New("ref URL may not be empty string")
	case map[string]any:
		description := strings.TrimSpace(getSafeString(rr, "description"))
		ref := strings.TrimSpace(getSafeString(rr, "ref"))
		if len(ref) > 0 {
			u, err := url.Parse(ref)
			if err != nil {
				return nil, err
			}
			return &RelatedResourceAnnotation{Description: description, Ref: *u}, nil
		}
		return nil, errors.New("'ref' value required in object")
	}

	return nil, errors.New("invalid value type, must be string or map")
}

func parseAuthor(a any) (*AuthorAnnotation, error) {
	a, err := convertYAMLMapKeyTypes(a, nil)
	if err != nil {
		return nil, err
	}

	switch a := a.(type) {
	case string:
		return parseAuthorString(a)
	case map[string]any:
		name := strings.TrimSpace(getSafeString(a, "name"))
		email := strings.TrimSpace(getSafeString(a, "email"))
		if len(name) > 0 || len(email) > 0 {
			return &AuthorAnnotation{name, email}, nil
		}
		return nil, errors.New("'name' and/or 'email' values required in object")
	}

	return nil, errors.New("invalid value type, must be string or map")
}

func getSafeString(m map[string]any, k string) string {
	if v, found := m[k]; found {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

const emailPrefix = "<"
const emailSuffix = ">"

// parseAuthor parses a string into an AuthorAnnotation. If the last word of the input string is enclosed within <>,
// it is extracted as the author's email. The email may not contain whitelines, as it then will be interpreted as
// multiple words.
func parseAuthorString(s string) (*AuthorAnnotation, error) {
	parts := strings.Fields(s)

	if len(parts) == 0 {
		return nil, errors.New("author is an empty string")
	}

	namePartCount := len(parts)
	trailing := parts[namePartCount-1]
	var email string
	if len(trailing) >= len(emailPrefix)+len(emailSuffix) && strings.HasPrefix(trailing, emailPrefix) &&
		strings.HasSuffix(trailing, emailSuffix) {
		email = trailing[len(emailPrefix):]
		email = email[:len(email)-len(emailSuffix)]
		namePartCount -= 1
	}

	name := strings.Join(parts[0:namePartCount], " ")

	return &AuthorAnnotation{Name: name, Email: email}, nil
}

func convertYAMLMapKeyTypes(x any, path []string) (any, error) {
	var err error
	switch x := x.(type) {
	case map[any]any:
		result := make(map[string]any, len(x))
		for k, v := range x {
			str, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("invalid map key type(s): %v", strings.Join(path, "/"))
			}
			result[str], err = convertYAMLMapKeyTypes(v, append(path, str))
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	case []any:
		for i := range x {
			x[i], err = convertYAMLMapKeyTypes(x[i], append(path, strconv.Itoa(i)))
			if err != nil {
				return nil, err
			}
		}
		return x, nil
	default:
		return x, nil
	}
}

// futureKeywords is the source of truth for future keywords that will
// eventually become standard keywords inside of Rego.
var futureKeywords = map[string]tokens.Token{
	"not": tokens.Not,
	"and": tokens.LogicalAnd,
	"or":  tokens.LogicalOr,
}

// futureKeywordsV0 is the source of truth for future keywords that were
// not yet a standard part of Rego in v0, and required importing.
var futureKeywordsV0 = map[string]tokens.Token{
	"in":       tokens.In,
	"every":    tokens.Every,
	"contains": tokens.Contains,
	"if":       tokens.If,
}

var allFutureKeywords map[string]tokens.Token

// experimentalFutureKeywords are future keywords that exist in the parser but are
// intentionally hidden from the default capabilities advertisement.
// They are only activated when a policy imports them AND the active
// capabilities explicitly list them. There are currently none.
var experimentalFutureKeywords = map[string]struct{}{}

var allFutureKeywordTokens map[tokens.Token]struct{}

func isFutureKeywordToken(tok tokens.Token) bool {
	_, ok := allFutureKeywordTokens[tok]
	return ok
}

func IsFutureKeyword(s string) bool {
	return IsFutureKeywordForRegoVersion(s, RegoV1)
}

func IsFutureKeywordForRegoVersion(s string, v RegoVersion) bool {
	var yes bool

	switch v {
	case RegoV0, RegoV0CompatV1:
		_, yes = futureKeywordsV0[s]
	case RegoV1:
		_, yes = futureKeywords[s]
	}

	return yes
}

// isFutureKeyword answers if keyword is from the "future" with the parser options set.
func (p *Parser) isFutureKeyword(s string) bool {
	return IsFutureKeywordForRegoVersion(s, p.po.RegoVersion)
}

func (p *Parser) futureImport(imp *Import, allowedFutureKeywords map[string]tokens.Token) {
	path := imp.Path.Value.(Ref)

	if len(path) == 1 || !path[1].Equal(InternedTerm("keywords")) {
		p.errorf(imp.Path.Location, "invalid import, must be `future.keywords`")
		return
	}

	if imp.Alias != "" {
		p.errorf(imp.Path.Location, "`future` imports cannot be aliased")
		return
	}

	kwds := util.Keys(allowedFutureKeywords)

	switch len(path) {
	case 2: // all keywords imported, nothing to do
	case 3: // one keyword imported
		kw, ok := path[2].Value.(String)
		if !ok {
			p.errorf(imp.Path.Location, "invalid import, must be `future.keywords.x`, e.g. `import future.keywords.in`")
			return
		}
		keyword := string(kw)
		if _, ok = allowedFutureKeywords[keyword]; !ok {
			p.errorf(imp.Path.Location, "unexpected keyword, must be one of %v", util.Sorted(kwds))
			return
		}

		kwds = []string{keyword} // overwrite
	}

	for _, kw := range kwds {
		if kw == "not" {
			p.notBodies = true
		}
		p.s.s.AddKeyword(kw, allowedFutureKeywords[kw])
	}
}

func (p *Parser) regoV1Import(imp *Import) {
	if !p.po.Capabilities.ContainsFeature(FeatureRegoV1Import) && !p.po.Capabilities.ContainsFeature(FeatureRegoV1) {
		p.errorf(imp.Path.Location, "invalid import, `%s` is not supported by current capabilities", RegoV1CompatibleRef)
		return
	}

	path := imp.Path.Value.(Ref)

	// v1 is only valid option
	if len(path) == 1 || !path[1].Equal(RegoV1CompatibleRef[1]) || len(path) > 2 {
		p.errorf(imp.Path.Location, "invalid import `%s`, must be `%s`", path, RegoV1CompatibleRef)
		return
	}

	if p.po.EffectiveRegoVersion() == RegoV1 {
		// We're parsing for Rego v1, where the 'rego.v1' import is a no-op.
		return
	}

	if imp.Alias != "" {
		p.errorf(imp.Path.Location, "`rego` imports cannot be aliased")
		return
	}

	// import all future keywords with the rego.v1 import
	kwds := util.Keys(futureKeywordsV0)

	p.s.s.SetRegoV1Compatible()
	for _, kw := range kwds {
		p.s.s.AddKeyword(kw, futureKeywordsV0[kw])
	}
}

func init() {
	allFutureKeywords = map[string]tokens.Token{}
	maps.Copy(allFutureKeywords, futureKeywords)
	maps.Copy(allFutureKeywords, futureKeywordsV0)

	allFutureKeywordTokens = make(map[tokens.Token]struct{}, len(allFutureKeywords))
	for _, tok := range allFutureKeywords {
		allFutureKeywordTokens[tok] = struct{}{}
	}
}

// enter increments the recursion depth counter and checks if it exceeds the maximum.
// Returns false if the maximum is exceeded, true otherwise.
// If p.maxRecursionDepth is 0 or negative, the check is effectively disabled.
func (p *Parser) enter() bool {
	p.recursionDepth++
	if p.maxRecursionDepth > 0 && p.recursionDepth > p.maxRecursionDepth {
		p.error(p.s.Loc(), ErrMaxParsingRecursionDepthExceeded.Error())
		p.recursionDepth--
		return false
	}
	return true
}

// leave decrements the recursion depth counter.
func (p *Parser) leave() {
	p.recursionDepth--
}
