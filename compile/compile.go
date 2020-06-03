// Copyright 2020 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

// Package compile implements bundles compilation and linking.
package compile

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"regexp"
	"sort"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/bundle"
	"github.com/open-policy-agent/opa/internal/ref"
	initload "github.com/open-policy-agent/opa/internal/runtime/init"
	"github.com/open-policy-agent/opa/loader"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage/inmem"
)

const (
	// TargetRego is the default target. The source rego is copied (potentially
	// rewritten for optimization purpsoes) into the bundle. The target supports
	// base documents.
	TargetRego = "rego"

	// TargetWasm is an alternative target that compiles the policy into a wasm
	// module instead of Rego. The target supports base documents.
	TargetWasm = "wasm"
)

const wasmResultVar = ast.Var("result")

var validTargets = map[string]struct{}{
	TargetRego: struct{}{},
	TargetWasm: struct{}{},
}

// Compiler implements bundle compilation and linking.
type Compiler struct {
	bundle            *bundle.Bundle   // the bundle that the compiler operates on
	revision          *string          // the revision to set on the output bundle
	asBundle          bool             // whether to assume bundle layout on file loading or not
	filter            loader.Filter    // filter to apply to file loader
	paths             []string         // file paths to load. TODO(tsandall): add support for supplying readers for embedded users.
	entrypoints       orderedStringSet // policy entrypoints required for optimization and certain targets
	optimizationLevel int              // how aggressive should optimization be
	target            string           // target type (wasm, rego, etc.)
	output            io.Writer        // output stream to write bundle to
	entrypointrefs    []*ast.Term      // validated entrypoints computed from default decision or manually supplied entrypoints
	compiler          *ast.Compiler    // rego ast compiler used for semantic checks and rewriting
	debug             *debugEvents     // debug information produced during build
}

type debugEvents struct {
	debug []Debug
}

func (d *debugEvents) Add(info Debug) {
	if d != nil {
		d.debug = append(d.debug, info)
	}
}

// Debug contains debugging information produced by the build about optimizations and other operations.
type Debug struct {
	Location *ast.Location
	Message  string
}

func (d Debug) String() string {
	return fmt.Sprintf("%v: %v", d.Location, d.Message)
}

// New returns a new compiler instance that can be invoked.
func New() *Compiler {
	return &Compiler{
		asBundle:          false,
		optimizationLevel: 0,
		target:            TargetRego,
		output:            ioutil.Discard,
		debug:             &debugEvents{},
	}
}

// Debug returns a list of debug events produced by the compiler.
func (c *Compiler) Debug() []Debug {
	return c.debug.debug
}

// WithRevision sets the revision to include in the output bundle manifest.
func (c *Compiler) WithRevision(r string) *Compiler {
	c.revision = &r
	return c
}

// WithAsBundle sets file loading mode on the compiler.
func (c *Compiler) WithAsBundle(enabled bool) *Compiler {
	c.asBundle = enabled
	return c
}

// WithEntrypoints sets the policy entrypoints on the compiler. Entrypoints tell the
// compiler what rules to expect and where optimizations can be targetted. The wasm
// target requires at least one entrypoint as does optimization.
func (c *Compiler) WithEntrypoints(e ...string) *Compiler {
	c.entrypoints = c.entrypoints.Append(e...)
	return c
}

// WithOptimizationLevel sets the optimization level on the compiler. By default
// optimizations are disabled. Higher levels apply more aggressive optimizations
// but can take longer. Currently only two levels are supported: 0 (disabled) and
// 1 (enabled).
func (c *Compiler) WithOptimizationLevel(n int) *Compiler {
	c.optimizationLevel = n
	return c
}

// WithTarget sets the output target type to use.
func (c *Compiler) WithTarget(t string) *Compiler {
	c.target = t
	return c
}

// WithOutput sets the output stream to write the bundle to.
func (c *Compiler) WithOutput(w io.Writer) *Compiler {
	c.output = w
	return c
}

// WithPaths adds input filepaths to read policy and data from.
func (c *Compiler) WithPaths(p ...string) *Compiler {
	c.paths = append(c.paths, p...)
	return c
}

// WithFilter sets the loader filter to use when reading non-bundle input files.
func (c *Compiler) WithFilter(filter loader.Filter) *Compiler {
	c.filter = filter
	return c
}

// Build compiles and links the input files and outputs a bundle to the writer.
func (c *Compiler) Build(ctx context.Context) error {

	if err := c.init(); err != nil {
		return err
	}

	if err := c.initBundle(); err != nil {
		return err
	}

	if err := c.optimize(ctx); err != nil {
		return err
	}

	if c.target == TargetWasm {
		if err := c.compileWasm(ctx); err != nil {
			return err
		}
	}

	if c.revision != nil {
		c.bundle.Manifest.Revision = *c.revision
	}

	return bundle.NewWriter(c.output).Write(*c.bundle)
}

func (c *Compiler) init() error {

	if _, ok := validTargets[c.target]; !ok {
		return fmt.Errorf("invalid target %q", c.target)
	}

	for _, e := range c.entrypoints {

		r, err := ref.ParseDataPath(e)
		if err != nil {
			return fmt.Errorf("entrypoint %v not valid: use <package>/<rule>", e)
		}

		if len(r) <= 2 {
			return fmt.Errorf("entrypoint %v too short: use <package>/<rule>", e)
		}

		c.entrypointrefs = append(c.entrypointrefs, ast.NewTerm(r))
	}

	if c.optimizationLevel > 0 && len(c.entrypointrefs) == 0 {
		return errors.New("bundle optimizations require at least one entrypoint")
	}

	if c.target == TargetWasm && len(c.entrypointrefs) != 1 {
		return errors.New("wasm compilation requires exactly one entrypoint")
	}

	return nil
}

func (c *Compiler) initBundle() error {

	// TODO(tsandall): the metrics object should passed through here so we that
	// we can track read and parse times.
	load, err := initload.LoadPaths(c.paths, c.filter, c.asBundle)
	if err != nil {
		return err
	}

	if c.asBundle {
		var names []string

		for k := range load.Bundles {
			names = append(names, k)
		}

		sort.Strings(names)
		var bundles []*bundle.Bundle

		for _, k := range names {
			bundles = append(bundles, load.Bundles[k])
		}

		result, err := bundle.Merge(bundles)
		if err != nil {
			return fmt.Errorf("bundle merge failed: %v", err)
		}

		c.bundle = result
		return nil
	}

	// TODO(tsandall): add support for controlling roots. Either the caller could
	// supply them or the compiler could infer them based on the packages and data
	// contents. The latter would require changes to the loader to preserve the
	// locations where base documents were mounted under data.
	result := &bundle.Bundle{}
	result.Manifest.Init()
	result.Data = load.Files.Documents

	var modules []string

	for k := range load.Files.Modules {
		modules = append(modules, k)
	}

	sort.Strings(modules)

	for _, module := range modules {
		result.Modules = append(result.Modules, bundle.ModuleFile{
			URL:    load.Files.Modules[module].Name,
			Path:   load.Files.Modules[module].Name,
			Parsed: load.Files.Modules[module].Parsed,
			Raw:    load.Files.Modules[module].Raw,
		})
	}

	c.bundle = result

	return nil
}

func (c *Compiler) optimize(ctx context.Context) error {

	if c.optimizationLevel <= 0 {
		var err error
		c.compiler, err = compile(c.bundle)
		return err
	}

	o := newOptimizer(c.bundle).
		WithEntrypoints(c.entrypointrefs).
		WithDebug(c.debug)

	err := o.Do(ctx)
	if err != nil {
		return err
	}

	c.bundle = o.Bundle()

	return nil
}

func (c *Compiler) compileWasm(ctx context.Context) error {

	// Lazily compile the modules if needed. If optimizations were run, the
	// AST compiler will not be set because the default target does not require it.
	if c.compiler == nil {
		var err error
		c.compiler, err = compile(c.bundle)
		if err != nil {
			return err
		}
	}

	store := inmem.NewFromObject(c.bundle.Data)
	resultSym := ast.NewTerm(wasmResultVar)

	cr, err := rego.New(
		rego.ParsedQuery(ast.NewBody(ast.Equality.Expr(resultSym, c.entrypointrefs[0]))),
		rego.Compiler(c.compiler),
		rego.Store(store),
	).Compile(ctx)

	if err != nil {
		return err
	}

	c.bundle.Wasm = cr.Bytes

	return nil
}

type undefinedEntrypointErr struct {
	Entrypoint *ast.Term
}

func (err undefinedEntrypointErr) Error() string {
	return fmt.Sprintf("undefined entrypoint %v", err.Entrypoint)
}

type optimizer struct {
	bundle          *bundle.Bundle
	compiler        *ast.Compiler
	entrypoints     []*ast.Term
	nsprefix        string
	resultsymprefix string
	outputprefix    string
	debug           *debugEvents
}

func newOptimizer(b *bundle.Bundle) *optimizer {
	return &optimizer{
		bundle:          b,
		nsprefix:        "partial",
		resultsymprefix: ast.WildcardPrefix,
		outputprefix:    "optimized",
	}
}

func (o *optimizer) WithDebug(debug *debugEvents) *optimizer {
	o.debug = debug
	return o
}

func (o *optimizer) WithEntrypoints(es []*ast.Term) *optimizer {
	o.entrypoints = es
	return o
}

func (o *optimizer) Do(ctx context.Context) error {

	// TODO(tsandall): implement optimization levels. These will just be params on partial evaluation for now.
	//
	// Level 1: PE w/ constant folding. Only inline rules that are completely known.
	// Level 2: L1 except inlining of rules with unknowns.
	// Level 3: L2 except aggressive inlining using negation and copy propagation optimizations.

	// NOTE(tsandall): if there are multiple entrypoints, copy the bundle because
	// if any of the optimization steps fail, we do not want to leave the caller's
	// bundle in a partially modified state.
	if len(o.entrypoints) > 1 {
		cpy := o.bundle.Copy()
		o.bundle = &cpy
	}

	// initialize other inputs to the optimization process (store, symbols, etc.)
	data := o.bundle.Data
	if data == nil {
		data = map[string]interface{}{}
	}

	store := inmem.NewFromObject(data)
	resultsym := ast.VarTerm(o.resultsymprefix + "__result__")
	usedFilenames := map[string]int{}

	// NOTE(tsandall): the entrypoints are optimized in order so that the optimization
	// of entrypoint[1] sees the optimization of entrypoint[0] and so on. This is needed
	// because otherwise the optimization outputs (e.g., support rules) would have to
	// merged somehow. Instead of dealing with that, just run the optimizations in the
	// order the user supplied the entrypoints in.
	for i, e := range o.entrypoints {

		var err error
		o.compiler, err = compile(o.bundle)
		if err != nil {
			return err
		}

		r := rego.New(
			rego.ParsedQuery(ast.NewBody(ast.Equality.Expr(resultsym, e))),
			rego.PartialNamespace(o.nsprefix),
			rego.DisableInlining(o.findRequiredDocuments(e)),
			rego.SkipPartialNamespace(true),
			rego.Compiler(o.compiler),
			rego.Store(store),
		)

		pq, err := r.Partial(ctx)
		if err != nil {
			return err
		}

		// NOTE(tsandall): this might be a bit too strict but in practice it's
		// unlikely users will want to ignore undefined entrypoints. make this
		// optional in the future.
		if len(pq.Queries) == 0 {
			return undefinedEntrypointErr{Entrypoint: e}
		}

		if module := o.getSupportForEntrypoint(pq.Queries, e, resultsym); module != nil {
			pq.Support = append(pq.Support, module)
		}

		modules := make([]bundle.ModuleFile, len(pq.Support))

		for j := range pq.Support {
			fileName := o.getSupportModuleFilename(usedFilenames, pq.Support[j], i, j)
			modules[j] = bundle.ModuleFile{
				URL:    fileName,
				Path:   fileName,
				Parsed: pq.Support[j],
			}
		}

		o.bundle.Modules = o.merge(o.bundle.Modules, modules)
	}

	sort.Slice(o.bundle.Modules, func(i, j int) bool {
		return o.bundle.Modules[i].URL < o.bundle.Modules[j].URL
	})

	// NOTE(tsandall): prune out rules and data that are not referenced in the bundle
	// in the future.
	o.bundle.Manifest.AddRoot(o.nsprefix)
	o.bundle.Manifest.Revision = ""

	return nil
}

func (o *optimizer) Bundle() *bundle.Bundle {
	return o.bundle
}

func (o *optimizer) findRequiredDocuments(ref *ast.Term) []string {

	keep := map[string]*ast.Location{}
	deps := map[*ast.Rule]struct{}{}

	for _, r := range o.compiler.GetRules(ref.Value.(ast.Ref)) {
		transitiveDependents(o.compiler, r, deps)
	}

	for rule := range deps {
		ast.WalkExprs(rule, func(expr *ast.Expr) bool {
			for _, with := range expr.With {
				// TODO(tsandall): this should be improved to exclude refs that are
				// marked as unknown. Since the build command does not allow users to
				// set unknowns, we can hardcode to assume 'input'.
				if !with.Target.Value.(ast.Ref).HasPrefix(ast.InputRootRef) {
					keep[with.Target.String()] = with.Target.Location
				}
			}
			return false
		})
	}

	var result []string

	for k := range keep {
		result = append(result, k)
	}

	sort.Strings(result)

	for _, k := range result {
		o.debug.Add(Debug{
			Location: keep[k],
			Message:  fmt.Sprintf("disables inlining of %v", k),
		})
	}

	return result
}

func (o *optimizer) getSupportForEntrypoint(queries []ast.Body, e *ast.Term, resultsym *ast.Term) *ast.Module {

	path := e.Value.(ast.Ref)
	name := ast.Var(path[len(path)-1].Value.(ast.String))
	module := &ast.Module{Package: &ast.Package{Path: path[:len(path)-1]}}

	for _, query := range queries {
		// NOTE(tsandall): when the query refers to the original entrypoint, throw it
		// away since this would create a recursive rule--this occurs if the entrypoint
		// cannot be partially evaluated.
		stop := false
		ast.WalkRefs(query, func(x ast.Ref) bool {
			if !stop {
				if x.HasPrefix(path) {
					stop = true
				}
			}
			return stop
		})
		if stop {
			return nil
		}
		module.Rules = append(module.Rules, &ast.Rule{
			Head:   ast.NewHead(name, nil, resultsym),
			Body:   query,
			Module: module,
		})
	}

	return module
}

// merge combines two sets of modules and returns the result. The rules from modules
// in 'b' override rules from modules in 'a'. If all rules in a module in 'a' are overridden
// by rules in modules in 'b' then the module from 'a' is discarded.
func (o *optimizer) merge(a, b []bundle.ModuleFile) []bundle.ModuleFile {

	prefixes := ast.NewSet()

	for i := range b {
		// NOTE(tsandall): use a set to memoize the prefix add operation--it's only
		// needed once per rule set and constructing the path for every rule in the
		// module could expensive for PE output (which can contain hundreds of thousands
		// of rules.)
		seen := ast.NewVarSet()
		for _, rule := range b[i].Parsed.Rules {
			if _, ok := seen[rule.Head.Name]; !ok {
				prefixes.Add(ast.NewTerm(rule.Path()))
				seen.Add(rule.Head.Name)
			}
		}

	}

	for i := range a {

		var keep []*ast.Rule

		// NOTE(tsandall): same as above--memoize keep/discard decision. If multiple
		// entrypoints are provided the dst module may contain a large number of rules.
		seen := ast.NewVarSet()
		discard := ast.NewVarSet()

		for _, rule := range a[i].Parsed.Rules {

			if _, ok := discard[rule.Head.Name]; ok {
				continue
			} else if _, ok := seen[rule.Head.Name]; ok {
				keep = append(keep, rule)
				continue
			}

			path := rule.Path()
			overlap := prefixes.Until(func(x *ast.Term) bool {
				ref := x.Value.(ast.Ref)
				return path.HasPrefix(ref)
			})

			if overlap {
				discard.Add(rule.Head.Name)
			} else {
				seen.Add(rule.Head.Name)
				keep = append(keep, rule)
			}
		}

		if len(keep) > 0 {
			a[i].Parsed.Rules = keep
			a[i].Raw = nil
			b = append(b, a[i])
		}

	}

	return b
}

func (o *optimizer) getSupportModuleFilename(used map[string]int, module *ast.Module, entrypointIndex int, supportIndex int) string {

	fileName, err := module.Package.Path.Ptr()

	if err == nil && safePathPattern.MatchString(fileName) {
		fileName = o.outputprefix + "/" + fileName
		if c, ok := used[fileName]; ok {
			fileName += fmt.Sprintf(".%d", c)
		}
		used[fileName]++
		fileName += ".rego"
		return fileName
	}

	return fmt.Sprintf("%v/%v/%v/%v.rego", o.outputprefix, o.nsprefix, entrypointIndex, supportIndex)
}

var safePathPattern = regexp.MustCompile(`^[\w-_/]+$`)

func compile(b *bundle.Bundle) (*ast.Compiler, error) {

	modules := map[string]*ast.Module{}

	for _, mf := range b.Modules {
		modules[mf.URL] = mf.Parsed
	}

	c := ast.NewCompiler()
	c.Compile(modules)

	if c.Failed() {
		return nil, c.Errors
	}

	return c, nil
}

func transitiveDependents(compiler *ast.Compiler, rule *ast.Rule, deps map[*ast.Rule]struct{}) {
	for x := range compiler.Graph.Dependents(rule) {
		other := x.(*ast.Rule)
		deps[other] = struct{}{}
		transitiveDependents(compiler, other, deps)
	}
}

type orderedStringSet []string

func (ss orderedStringSet) Append(s ...string) orderedStringSet {
	for _, x := range s {
		var found bool
		for _, other := range ss {
			if x == other {
				found = true
			}
		}
		if !found {
			ss = append(ss, x)
		}
	}
	return ss
}
