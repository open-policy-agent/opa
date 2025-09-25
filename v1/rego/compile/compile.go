// Package compile collects a specialized interface to package rego, built for
// compiling policies into filters. It's a combination of simple evals (of any
// masking rule), and partial eval, equipped with the correct settings for some
// options; and paired with post-checks that determine if the result of partial
// evaluation can be translated into filter queries for certain targets/dialects.
// On success, the PE results are translated into queries, i.e. SQL WHERE clauses
// or UCAST expressions.
package compile

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/internal/compile"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/metrics"
	"github.com/open-policy-agent/opa/v1/rego"

	// "github.com/open-policy-agent/opa/v1/storage"
	"github.com/open-policy-agent/opa/v1/util"
)

type CompileOption func(*Compile)

type Compile struct {
	targets  []string
	dialects []string
	maskRule ast.Ref
	unknowns []*ast.Term // ast.Ref would be slightly more on-the-spot, but we follow what is done in v1/rego to minimise surprises.
	query    ast.Body
	mappings map[string]any
	metrics  metrics.Metrics

	regoOpts []func(*rego.Rego)
}

// Rego allows passing through common `*rego.Rego` options
func Rego(o ...func(*rego.Rego)) CompileOption {
	return func(c *Compile) {
		c.regoOpts = append(c.regoOpts, o...)
	}
}

// Target lets you control the targets of a filter compilation. If repeated,
// it'll apply constraints for all the targets simultaneously (i.e. the
// union of their constraints = the intersection of supported features).
func Target(target, dialect string) CompileOption {
	return func(c *Compile) {
		c.targets = append(c.targets, target)
		c.dialects = append(c.dialects, dialect)
	}
}

// MaskRule determines which rule of the provided modules is to be evaluated
// to determine the masking of columns. Applying those masking rules is an out-
// of-band concern, when processing the results of the query.
func MaskRule(rule ast.Ref) CompileOption {
	return func(c *Compile) {
		c.maskRule = rule
	}
}

// Mappings allows controlling the table and column names of the generated
// queries, if they don't match what's in the policy's unknowns.
// These can be simple maps, like
//
//	{
//	  fruit: {
//	    $self: "fruit_table",
//	    name: "name_col",
//	  }
//	}
//
// or per-target/per-dialect,
//
//	{
//	  sql: { // per-target
//		   fruit: {
//		     $self: "fruit_table",
//		     name: "name_col",
//		   }
//	  }
//	}
//
//	{
//	  postgresql: { // per-dialect
//	      fruit: {
//	        $self: "fruit_table",
//	        name: "name_col",
//	      }
//	   }
//	}
func Mappings(m map[string]any) CompileOption {
	return func(c *Compile) {
		c.mappings = m
	}
}

// Metrics allows passing the `metrics.Metrics` to use for recording timers.
// It's passed along to the underlying `rego.Rego` evals, too.
func Metrics(m metrics.Metrics) CompileOption {
	return func(c *Compile) {
		c.metrics = m
	}
}

// ParsedUnknowns lets you pass in the unknowns of this filter compilation.
func ParsedUnknowns(s ...*ast.Term) CompileOption {
	return func(c *Compile) {
		c.unknowns = s
	}
}

// ParsedQuery lets you pass in the main entrypoint of this filter compilation.
func ParsedQuery(q ast.Body) CompileOption {
	return func(c *Compile) {
		c.query = q
	}
}

// New creates a new `*compile.Compile` struct.
func New(opts ...CompileOption) *Compile {
	c := &Compile{}
	for i := range opts {
		opts[i](c)
	}
	c.regoOpts = append(c.regoOpts,
		rego.Metrics(c.metrics),
		// We require evaluating non-det builtins for the translated targets:
		// We're not able to meaningfully tanslate things like http.send, sql.send, or
		// io.jwt.decode_verify into SQL or UCAST, so we try to eval them out where possible.
		rego.NondeterministicBuiltins(true),
	)
	return c
}

// Prepared represents a ready-for-eval intermediate state where everything not depending
// on input has already been done.
type Prepared struct {
	compile *Compile // carry along

	constraintSet        *compile.ConstraintSet
	shorts               compile.Set[string]
	regoPrepareOptions   []rego.PrepareOption
	preparedMaskQuery    *rego.PreparedEvalQuery
	preparedPartialQuery *rego.PreparedPartialQuery
}

type PrepareOption func(*Prepared)

// RegoPrepareOptions lets you pass through any `rego.PrepareOption`.
func RegoPrepareOptions(o ...rego.PrepareOption) PrepareOption {
	return func(p *Prepared) {
		p.regoPrepareOptions = append(p.regoPrepareOptions, o...)
	}
}

// noop allows us to call c.timer() for metrics even if we have no metrics
type noop struct{}

var n = noop{}

type minimetrics interface {
	Start()
	Stop() int64
}

func (noop) Start()      {}
func (noop) Stop() int64 { return 0 }

func (c *Compile) timer(t string) minimetrics {
	if c.metrics != nil {
		return c.metrics.Timer(t)
	}
	return n
}

// Prepare evaluates as much as possible without knowing the (known) input yet
func (c *Compile) Prepare(ctx context.Context, po ...PrepareOption) (*Prepared, error) {
	p := &Prepared{
		compile: c,
	}
	for i := range po {
		po[i](p)
	}

	// constraints
	c.timer(metrics.CompileEvalConstraints).Start()
	defer c.timer(metrics.CompileEvalConstraints).Stop()
	constrs := make([]*compile.Constraint, len(p.compile.targets))
	for i := range p.compile.targets {
		var err error
		constrs[i], err = compile.NewConstraints(p.compile.targets[i], p.compile.dialects[i]) // NewConstraints validates the tuples
		if err != nil {
			return nil, err
		}
	}
	p.constraintSet = compile.NewConstraintSet(constrs...)
	p.shorts = compile.ShortsFromMappings(p.compile.mappings)

	// mask prep
	if c.maskRule != nil {
		rp, err := rego.New(append(c.regoOpts,
			rego.ParsedQuery(ast.NewBody(ast.NewExpr(ast.NewTerm(c.maskRule)))),
		)...).PrepareForEval(ctx, p.regoPrepareOptions...)
		if err != nil {
			return nil, fmt.Errorf("prepare for mask eval: %w", err)
		}
		p.preparedMaskQuery = &rp
	}

	// PE prep
	rp, err := rego.New(append(c.regoOpts,
		rego.ParsedQuery(c.query),
	)...).PrepareForPartial(ctx, p.regoPrepareOptions...)
	if err != nil {
		return nil, fmt.Errorf("prepare for partial: %w", err)
	}

	p.preparedPartialQuery = &rp
	return p, nil
}

// Filter represents the result of a policy-to-filter compilation for one
// specific target/dialect
type Filter struct {
	Query any
	Masks map[string]any
}

// Filters represents all the filters compiled from a policy. Can contain
// `compile.Filter` for various target/dialect combinations.
type Filters struct {
	filters map[string]map[string]Filter
}

func (f *Filters) push(target, dialect string, query any, masks map[string]any) {
	if f.filters == nil {
		f.filters = make(map[string]map[string]Filter)
	}
	if f.filters[target] == nil {
		f.filters[target] = make(map[string]Filter)
	}
	f.filters[target][dialect] = Filter{Query: query, Masks: masks}
}

// For is a helper method to retrieve the `compile.Filter` for a specific
// target/dialect.
func (f *Filters) For(target, dialect string) Filter {
	return f.filters[target][dialect]
}

// One is a helper that extracts a single `compile.Filter`. Only safe to use
// when filters have been compiled for a single target/dialect, otherwise it'll
// return a random result of the compile filters. Panics when there's no filter
// at all.
func (f *Filters) One() Filter {
	for i := range f.filters {
		for j := range f.filters[i] {
			return f.filters[i][j]
		}
	}
	panic("empty filters")
}

// Compile does all the steps needed to generate `*compile.Filters` from a
// prepared state (`*compile.Prepared`).
func (p *Prepared) Compile(ctx context.Context, eo ...rego.EvalOption) (*Filters, error) {
	// mask eval (non-partial)
	var maskResult map[string]any
	maskOpts := append(eo,
		rego.EvalMetrics(p.compile.metrics),
		rego.EvalRuleIndexing(false),
		rego.EvalNondeterministicBuiltins(true),
	)
	if p.preparedMaskQuery != nil {
		p.compile.timer(metrics.CompileEvalMaskRule).Start()
		rs, err := p.preparedMaskQuery.Eval(ctx, maskOpts...)
		if err != nil {
			return nil, fmt.Errorf("evaluate masks: %w", err)
		}
		if len(rs) != 0 {
			maskResultValue, err := ast.InterfaceToValue(rs[0].Expressions[0].Value)
			if err != nil {
				return nil, fmt.Errorf("convert masks: %w", err)
			}
			if err := util.Unmarshal([]byte(maskResultValue.String()), &maskResult); err != nil {
				return nil, fmt.Errorf("convert masks: %w", err)
			}
		}
		p.compile.timer(metrics.CompileEvalMaskRule).Stop()
	}

	// PE for conversion
	opts := append(maskOpts,
		rego.EvalParsedUnknowns(p.compile.unknowns),
	)
	pq, err := p.preparedPartialQuery.Partial(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("partial eval: %w", err)
	}

	if errs := compile.Check(pq, p.constraintSet, p.shorts).ASTErrors(); errs != nil {
		return nil, ast.Errors(errs)
	}

	p.compile.timer(metrics.CompileTranslateQueries).Start()
	defer p.compile.timer(metrics.CompileTranslateQueries).Start()

	ret := Filters{}
	for i := range p.compile.targets {
		target, dialect := p.compile.targets[i], p.compile.dialects[i]
		if pq.Queries == nil { // unconditional NO
			ret.push(target, dialect, nil, nil)
			continue
		}
		mappings, err := lookupMappings(p.compile.mappings, target, dialect)
		if err != nil {
			return nil, fmt.Errorf("mappings: %w", err)
		}
		switch target {
		case "ucast":
			query := compile.QueriesToUCAST(pq.Queries, mappings)
			ret.push(target, dialect, query.Map(), maskResult)
		case "sql":
			sql, err := compile.QueriesToSQL(pq.Queries, mappings, dialect)
			if err != nil {
				return nil, fmt.Errorf("convert to queries: %w", err)
			}
			ret.push(target, dialect, sql, maskResult)
		}
	}

	return &ret, nil
}

func lookupMappings(mappings map[string]any, target, dialect string) (map[string]any, error) {
	if mappings == nil {
		return nil, nil
	}

	if md := mappings[dialect]; md != nil {
		m, ok := md.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid mappings for dialect %s", dialect)
		}
		if m != nil {
			return m, nil
		}
	}

	if mt := mappings[target]; mt != nil {
		n, ok := mt.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid mappings for target %s", target)
		}
		return n, nil
	}
	return mappings, nil
}
