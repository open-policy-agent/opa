package ast

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"maps"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/open-policy-agent/opa/v1/types"
)

// ReuseKey returns the key under which ReusableRule finds a compiled rule of
// package pkg (as printed, e.g. "data.a.b"), given the source texts of its
// annotations and then of the rule itself.
func ReuseKey(pkg string, texts ...[]byte) string {
	h := sha256.New()
	var n [binary.MaxVarintLen64]byte
	for _, t := range append([][]byte{[]byte(pkg)}, texts...) {
		h.Write(n[:binary.PutUvarint(n[:], uint64(len(t)))])
		h.Write(t)
	}
	return string(h.Sum(nil))
}

// moduleKey keys a module by its source text, or if it has none (built in Go
// or decoded from JSON), by its printed form.
func moduleKey(m *Module) string {
	texts := [][]byte{[]byte(m.regoVersion.String())}
	add := func(l *Location) bool {
		if l == nil || len(l.Text) == 0 {
			return false
		}
		texts = append(texts, l.Text)
		return true
	}
	ok := add(m.Package.Location)
	for _, imp := range m.Imports {
		ok = ok && add(imp.Location)
	}
	for _, r := range m.Rules {
		ok = ok && add(r.Location)
	}
	for _, a := range m.Annotations {
		ok = ok && add(a.Location)
	}
	if !ok {
		return ReuseKey("printed", []byte(m.regoVersion.String()), []byte(m.String()))
	}
	for _, cm := range m.Comments {
		texts = append(texts, cm.Text)
	}
	return ReuseKey("", texts...)
}

func reuseKey(r *Rule) (string, bool) {
	if r.Location == nil || len(r.Location.Text) == 0 || r.Module == nil {
		return "", false
	}
	texts := make([][]byte, 0, len(r.Annotations)+1)
	for _, a := range r.Annotations {
		if a.Location == nil {
			return "", false
		}
		texts = append(texts, a.Location.Text)
	}
	return ReuseKey(r.Module.Package.Path.String(), append(texts, r.Location.Text)...), true
}

// WithRuleReuse makes Compile carry over prev's compiled rules whose source is
// unchanged, compiling only the added ones when that gives the same result as
// compiling everything. prev must have been compiled with rule reuse too.
func (c *Compiler) WithRuleReuse(prev *Compiler) *Compiler {
	c.reuse.enabled = true
	c.reuse.prev = prev
	return c
}

// WithReparser sets how Compile gets a fully parsed module back when it must
// compile everything but the module holds rules returned by ReusableRule.
func (c *Compiler) WithReparser(f func(name string) (*Module, error)) *Compiler {
	c.reuse.reparse = f
	return c
}

// ReusableRule returns the compiled rule for key, for a parser to put in place
// of a rule it skipped. Only a Compile with rule reuse accepts such modules.
func (c *Compiler) ReusableRule(key string) *Rule {
	return c.reuse.rules[key]
}

// RuleReuseStats describes what the last Compile with rule reuse did.
type RuleReuseStats struct {
	Enabled      bool // WithRuleReuse was set
	Incremental  bool // only added rules were compiled
	Reused       int  // compiled rules carried over
	ParseSkipped int  // rules a parser left out, see ReusableRule
}

// RuleReuseStats returns what the last Compile did about rule reuse.
func (c *Compiler) RuleReuseStats() RuleReuseStats {
	return RuleReuseStats{
		Enabled:      c.reuse.enabled,
		Incremental:  c.reuse.incremental,
		Reused:       c.reuse.reused,
		ParseSkipped: c.reuse.parseSkipped,
	}
}

type ruleReuseState struct {
	enabled      bool
	prev         *Compiler
	reparse      func(name string) (*Module, error)
	base         map[string]string       // source keys of the modules that aren't reusable
	pkgs         map[string]reusePackage // packages of reusable modules
	rules        map[string]*Rule        // compiled rules of reusable modules, by key
	reused       int
	incremental  bool
	parseSkipped int
}

type reusePackage struct {
	path        Ref
	imports     []*Import
	regoVersion RegoVersion
}

// Reusable modules of a package must agree on imports and rego-version, as
// keys don't cover them.
func splitReusable(modules map[string]*Module) (map[string][]string, map[string]*Module, map[string]reusePackage) {
	keys := map[string][]string{}
	base := make(map[string]*Module, len(modules))
	pkgs := map[string]reusePackage{}
	for _, name := range slices.Sorted(maps.Keys(modules)) {
		mod := modules[name]
		mkeys, ok := reusableKeys(mod)
		if ok {
			p := mod.Package.Path.String()
			if pkg, seen := pkgs[p]; !seen {
				pkgs[p] = reusePackage{mod.Package.Path, mod.Imports, mod.regoVersion}
			} else if !slices.EqualFunc(pkg.imports, mod.Imports, (*Import).Equal) || pkg.regoVersion != mod.regoVersion {
				ok = false
			}
		}
		if ok {
			keys[name] = mkeys
		} else {
			base[name] = mod
		}
	}
	return keys, base, pkgs
}

// Modules with functions aren't reusable: other rules call them, and added
// rules must only refer to what isn't reusable.
func reusableKeys(mod *Module) ([]string, bool) {
	if len(mod.Rules) == 0 {
		return nil, false
	}
	for _, a := range mod.Annotations {
		// A document annotation applies to the rules of other modules too.
		if _, ok := a.node.(*Rule); !ok || a.Scope == annotationScopeDocument {
			return nil, false
		}
	}
	keys := make([]string, len(mod.Rules))
	for i, r := range mod.Rules {
		k, ok := reuseKey(r)
		if !ok || r.Default || r.Else != nil || len(r.Head.Args) > 0 {
			return nil, false
		}
		keys[i] = k
	}
	return keys, true
}

func (c *Compiler) compileWithReuse(modules map[string]*Module) {
	keys, base, pkgs := splitReusable(modules)
	c.reuse.incremental = c.compileIncremental(modules, keys, base, pkgs)
	c.reuse.parseSkipped = 0
	if c.reuse.incremental {
		for _, m := range modules {
			for _, r := range m.Rules {
				if isPlaceholder(r) {
					c.reuse.parseSkipped++
				}
			}
		}
	} else {
		c.reuse.reused = 0
		reparsed, ok := c.reparsePlaceholders(modules)
		if !ok {
			return
		}
		if reparsed != nil {
			modules = reparsed
			keys, base, pkgs = splitReusable(modules)
		}
		if c.compileModules(modules); c.Failed() {
			return
		}
	}

	c.reuse.base = make(map[string]string, len(base))
	for name, m := range base {
		c.reuse.base[name] = moduleKey(m)
	}
	c.reuse.pkgs = pkgs
	c.reuse.rules = make(map[string]*Rule, len(c.reuse.rules))
	for name := range keys {
		m := c.Modules[name]
		m.reusable = true
		for _, r := range m.Rules {
			if k, ok := reuseKey(r); ok {
				c.reuse.rules[k] = r
			}
		}
	}
}

type reuseDelta struct {
	prev    *Compiler
	modules map[string]*Module
	keys    map[string][]string // of reusable modules, aligned with their rules
	base    map[string]*Module
	pkgs    map[string]reusePackage

	seen      map[string]struct{} // keys of all reusable rules
	added     map[string]*Module  // per reusable module, the rules prev lacks
	reused    int
	unchanged map[string]bool // reusable modules with the same rules as in prev
	dc        *Compiler       // compiled the added rules
	ext       []externalIndex
}

func (c *Compiler) compileIncremental(modules map[string]*Module, keys map[string][]string, base map[string]*Module, pkgs map[string]reusePackage) bool {
	d := &reuseDelta{prev: c.reuse.prev, modules: modules, keys: keys, base: base, pkgs: pkgs}
	return c.reuseApplies(d) && d.splitRules() && d.checkRemovals() && c.compileAdded(d) && c.assemble(d)
}

func (c *Compiler) reuseApplies(d *reuseDelta) bool {
	prev := d.prev
	if prev == nil || prev.reuse.rules == nil || len(c.after) > 0 ||
		c.moduleLoader != nil || c.keepModules || !c.sameOptions(prev) {
		return false
	}
	// Strict mode's unused-import check spans all rules of a module.
	if c.strict {
		return false
	}
	// Which refs are external decides how rules referring to them compile.
	if c.externalSources.Len() != prev.externalSources.Len() ||
		slices.ContainsFunc(c.externalSources.Keys(), func(r Ref) bool { _, ok := prev.externalSources.Get(r); return !ok }) {
		return false
	}
	if len(d.base) != len(prev.reuse.base) || len(d.pkgs) != len(prev.reuse.pkgs) {
		return false
	}
	for name, m := range d.base {
		if k, ok := prev.reuse.base[name]; !ok || k != moduleKey(m) {
			return false
		}
	}
	for p, pkg := range d.pkgs {
		pp, ok := prev.reuse.pkgs[p]
		if !ok || !slices.EqualFunc(pkg.imports, pp.imports, (*Import).Equal) || pkg.regoVersion != pp.regoVersion {
			return false
		}
	}
	return true
}

// sameOptions reports whether c compiles like prev.
func (c *Compiler) sameOptions(prev *Compiler) bool {
	switch {
	case c.enablePrintStatements != prev.enablePrintStatements:
	case c.strict != prev.strict:
	case c.useTypeCheckAnnotations != prev.useTypeCheckAnnotations:
	case c.allowUndefinedFuncCalls != prev.allowUndefinedFuncCalls:
	case c.evalMode != prev.evalMode:
	case c.rewriteTestRulesForTracing != prev.rewriteTestRulesForTracing:
	case c.defaultRegoVersion != prev.defaultRegoVersion:
	case c.schemaSet != prev.schemaSet:
	case c.injectedVirtual != nil || prev.injectedVirtual != nil:
	case !maps.Equal(c.unsafeBuiltinsMap, prev.unsafeBuiltinsMap):
	case !maps.Equal(c.skipStages, prev.skipStages):
	case !slices.Equal(c.capabilities.Features, prev.capabilities.Features):
	case !slices.Equal(c.capabilities.FutureKeywords, prev.capabilities.FutureKeywords):
	case !slices.Equal(c.capabilities.AllowNet, prev.capabilities.AllowNet):
	default:
		return maps.EqualFunc(c.builtins, prev.builtins, func(a, b *Builtin) bool {
			return a == b || a.Name == b.Name && types.Compare(a.Decl, b.Decl) == 0
		})
	}
	return false
}

func (d *reuseDelta) splitRules() bool {
	d.added = map[string]*Module{}
	d.seen = make(map[string]struct{}, len(d.prev.reuse.rules))
	for name, mkeys := range d.keys {
		mod := d.modules[name]
		var a *Module
		for i, r := range mod.Rules {
			k := mkeys[i]
			if _, dup := d.seen[k]; dup {
				return false
			}
			d.seen[k] = struct{}{}
			if _, ok := d.prev.reuse.rules[k]; ok {
				d.reused++
				continue
			}
			if isPlaceholder(r) {
				return false
			}
			if a == nil {
				a = &Module{Package: mod.Package, Imports: mod.Imports, regoVersion: mod.regoVersion}
				d.added[name] = a
			}
			a.Rules = append(a.Rules, r)
		}
		if a != nil {
			in := make(map[*Rule]struct{}, len(a.Rules))
			for _, r := range a.Rules {
				in[r] = struct{}{}
			}
			for _, ann := range mod.Annotations {
				if _, ok := in[ann.node.(*Rule)]; ok {
					a.Annotations = append(a.Annotations, ann)
				}
			}
		}
	}
	return true
}

// An emptied rule set changes the rules depending on it.
func (d *reuseDelta) checkRemovals() bool {
	removed := map[string][]*Rule{}
	for k, r := range d.prev.reuse.rules {
		if _, ok := d.seen[k]; !ok {
			ref := r.Ref().String()
			removed[ref] = append(removed[ref], r)
		}
	}
	for _, rs := range removed {
		ref := rs[0].Ref()
		if len(rs) >= len(d.prev.GetRulesExact(ref)) || hasUnion(d.prev.TypeEnv.Get(ref)) {
			return false
		}
	}

	d.unchanged = map[string]bool{}
	for name, mkeys := range d.keys {
		pm, ok := d.prev.Modules[name]
		if !ok || !pm.reusable || len(pm.Rules) != len(mkeys) || d.added[name] != nil {
			continue
		}
		d.unchanged[name] = true
		for i, k := range mkeys {
			if d.prev.reuse.rules[k] != pm.Rules[i] {
				d.unchanged[name] = false
				break
			}
		}
	}
	return true
}

func (c *Compiler) compileAdded(d *reuseDelta) bool {
	ext, ok := c.initExternalSources()
	if !ok {
		return false
	}
	d.ext = ext
	d.dc = c.newDeltaCompiler()
	for _, e := range ext {
		d.dc.externalSources.Put(e.ref, e)
	}
	in := make(map[string]*Module, len(d.base)+len(d.added))
	maps.Copy(in, d.base)
	maps.Copy(in, d.added)
	if d.dc.Compile(in); d.dc.Failed() {
		return false
	}
	var roots []*Rule
	for name := range d.added {
		for _, r := range d.dc.Modules[name].Rules {
			if !c.isCompatibleAddedRule(r, d.dc) {
				return false
			}
			roots = append(roots, r)
		}
	}
	return !c.reachesReusable(roots, d.dc)
}

// dc saw rules of other modules refer to reusable rules as unknown data, so an
// added rule must not depend on them, e.g. through a function.
func (c *Compiler) reachesReusable(roots []*Rule, dc *Compiler) bool {
	seen := map[*Rule]struct{}{}
	for len(roots) > 0 {
		r := roots[len(roots)-1]
		roots = roots[:len(roots)-1]
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		if c.refsReusable(r) {
			return true
		}
		for dep := range dc.Graph.Dependencies(r) {
			if dr, ok := dep.(*Rule); ok {
				roots = append(roots, dr)
			}
		}
	}
	return false
}

func (c *Compiler) refsReusable(x any) bool {
	found := false
	WalkRefs(x, func(ref Ref) bool {
		for _, pkg := range c.reuse.prev.reuse.pkgs {
			if ref.HasPrefix(pkg.path) || pkg.path.HasPrefix(ref) {
				found = true
			}
		}
		return found
	})
	return found
}

// prev keeps serving queries, so its rules are copied rather than re-pointed.
func (c *Compiler) assemble(d *reuseDelta) bool {
	prev, dc := d.prev, d.dc
	c.Modules = make(map[string]*Module, len(d.modules))
	c.imports = make(map[string][]*Import, len(d.modules))
	for name := range d.base {
		c.Modules[name] = prev.Modules[name]
		c.imports[name] = prev.imports[name]
	}
	annotations := map[*Module]map[*Rule][]*Annotations{}
	annotationsOf := func(r *Rule) []*Annotations {
		if annotations[r.Module] == nil {
			annotations[r.Module] = ruleAnnotations(r.Module)
		}
		return annotations[r.Module][r]
	}
	for name, mkeys := range d.keys {
		mod := d.modules[name]
		c.imports[name] = mod.Imports
		if d.unchanged[name] {
			c.Modules[name] = prev.Modules[name]
			continue
		}
		m := &Module{Package: mod.Package, regoVersion: mod.regoVersion, reusable: true}
		c.Modules[name] = m
		var compiled []*Rule
		if a := dc.Modules[name]; a != nil {
			compiled = a.Rules
		}
		for _, k := range mkeys {
			if r, ok := prev.reuse.rules[k]; ok {
				m.addRuleCopy(r, annotationsOf(r))
			} else {
				m.addRuleCopy(compiled[0], annotationsOf(compiled[0]))
				compiled = compiled[1:]
			}
		}
	}

	c.sorted = slices.Sorted(maps.Keys(c.Modules))
	c.Required = mergeRequired(prev.Required, dc.Required)
	c.RewrittenVars = make(map[Var]Var, len(prev.RewrittenVars)+len(dc.RewrittenVars))
	maps.Copy(c.RewrittenVars, prev.RewrittenVars)
	maps.Copy(c.RewrittenVars, dc.RewrittenVars)
	c.comprehensionIndices = make(map[*Term]*ComprehensionIndex, len(prev.comprehensionIndices)+len(dc.comprehensionIndices))
	maps.Copy(c.comprehensionIndices, prev.comprehensionIndices)
	maps.Copy(c.comprehensionIndices, dc.comprehensionIndices)

	c.setModuleTree()
	c.RuleTree = NewRuleTree(c.ModuleTree)
	for _, e := range d.ext {
		c.RuleTree.add(e.ref, e.index)
	}
	c.setGraph()

	vis := NewVarVisitor()
	for name := range d.added {
		vis.Walk(c.Modules[name])
	}
	exclude := prev.localvargen.exclude.Copy()
	exclude.Update(vis.vars)
	c.localvargen = &localVarGenerator{exclude: exclude, suffix: LocalVarPrefix}

	c.TypeEnv = c.rebindTypeEnv(prev.TypeEnv)
	c.setAnnotationSet()
	reuseRuleIndices(prev.RuleTree, c.RuleTree)
	c.buildRuleIndices()
	c.reuse.reused = d.reused
	// A conflict is reported by the full compile that follows.
	return !c.Failed() && (c.pathExists == nil || len(CheckPathConflicts(c, c.pathExists)) == 0)
}

type externalIndex struct {
	ref   Ref
	index ExternalRuleIndex
}

func (e externalIndex) Refs() []Ref { return []Ref{e.ref} }

func (e externalIndex) Init(context.Context, Ref) (ExternalRuleIndex, error) {
	return e.index, nil
}

// The sources are replaced with their indices so that a full compile after
// this doesn't initialize them again.
func (c *Compiler) initExternalSources() ([]externalIndex, bool) {
	var out []externalIndex
	ok := true
	c.externalSources.Iter(func(ref Ref, source ExternalRuleSource) bool {
		index, err := source.Init(context.TODO(), ref)
		if err != nil {
			ok = false
			return true
		}
		out = append(out, externalIndex{ref, index})
		return false
	})
	if ok {
		for _, e := range out {
			c.externalSources.Put(e.ref, e)
		}
	}
	return out, ok
}

func mergeRequired(a, b *Capabilities) *Capabilities {
	out := *a
	out.Builtins = slices.Clone(a.Builtins)
	for _, bi := range b.Builtins {
		if !slices.ContainsFunc(out.Builtins, func(x *Builtin) bool { return x.Name == bi.Name }) {
			out.Builtins = append(out.Builtins, bi)
		}
	}
	slices.SortFunc(out.Builtins, func(x, y *Builtin) int { return strings.Compare(x.Name, y.Name) })
	out.Features = slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(a.Features), b.Features...))))
	out.FutureKeywords = slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(a.FutureKeywords), b.FutureKeywords...))))
	return &out
}

func reuseRuleIndices(prev, cur *TreeNode) {
	if prev.Index != nil && len(cur.Values) > 0 && slices.Equal(prev.Values, cur.Values) &&
		!slices.ContainsFunc(cur.Values, func(r *Rule) bool { return !r.Head.Ref().IsGround() }) {
		cur.Index = prev.Index
	}
	for k, child := range cur.Children {
		if pc := prev.Child(k); pc != nil {
			reuseRuleIndices(pc, child)
		}
	}
}

// Removing rules can narrow the type of a rule set only if it holds a union, and
// rules that type-checked against the union may then fail.
func hasUnion(t types.Type) bool {
	switch t := t.(type) {
	case types.Any:
		return true
	case *types.Set:
		return hasUnion(t.Of())
	case *types.Array:
		if d := t.Dynamic(); d != nil && hasUnion(d) {
			return true
		}
		for i := range t.Len() {
			if hasUnion(t.Select(i)) {
				return true
			}
		}
	case *types.Object:
		if d := t.DynamicProperties(); d != nil && (hasUnion(d.Key) || hasUnion(d.Value)) {
			return true
		}
		for _, p := range t.StaticProperties() {
			if hasUnion(p.Value) {
				return true
			}
		}
	}
	return false
}

// dc did not see the rules of reusable modules, so added rules must not use them.
func (c *Compiler) isCompatibleAddedRule(r *Rule, dc *Compiler) bool {
	prev := c.reuse.prev
	if r.Default || r.Else != nil || len(r.Head.Args) > 0 || !r.Head.Ref().IsGround() {
		return false
	}
	ref := r.Ref()
	existing := prev.GetRulesExact(ref)
	if len(existing) == 0 || existing[0].Head.RuleKind() != r.Head.RuleKind() {
		return false
	}
	if types.Compare(prev.TypeEnv.Get(ref), dc.TypeEnv.Get(ref)) != 0 {
		return false
	}
	compatible := !c.refsReusable(r.Body)
	WalkExprs(r.Body, func(x *Expr) bool {
		if isRegoMetadataChainCall(x) || isRegoMetadataRuleCall(x) {
			compatible = false
		}
		return !compatible
	})
	return compatible
}

// env's checker would otherwise keep the compiler that built it alive.
func (c *Compiler) rebindTypeEnv(env *TypeEnv) *TypeEnv {
	checker := newTypeChecker().
		WithAllowNet(c.capabilities.AllowNet).
		WithSchemaSet(c.schemaSet).
		WithInputType(c.inputType).
		WithBuiltins(c.builtins).
		WithRequiredCapabilities(c.Required).
		WithVarRewriter(rewriteRefErrVars(c.localvargen.subjects, c.RewrittenVars)).
		WithDependentsResolver(c.dependentRuleRefs).
		WithAllowUndefinedFunctionCalls(c.allowUndefinedFuncCalls)
	cpy := *env
	cpy.newChecker = checker.copyForEnv
	return &cpy
}

func isPlaceholder(r *Rule) bool {
	return r.Module != nil && r.Module.reusable
}

func (c *Compiler) reparsePlaceholders(modules map[string]*Module) (map[string]*Module, bool) {
	var names []string
	for name, m := range modules {
		if slices.ContainsFunc(m.Rules, isPlaceholder) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, true
	}
	slices.Sort(names)
	if c.reuse.reparse == nil {
		c.err(NewError(CompileErr, modules[names[0]].Package.Location, "%s: module holds reused rules but cannot be parsed again", names[0]))
		return nil, false
	}

	parsed := make([]*Module, len(names))
	errs := make([]error, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	for i, name := range names {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			parsed[i], errs[i] = c.reuse.reparse(name)
		})
	}
	wg.Wait()

	out := maps.Clone(modules)
	for i, name := range names {
		if errs[i] != nil {
			c.err(NewError(ParseErr, modules[name].Package.Location, "%s: %v", name, errs[i]))
			return nil, false
		}
		out[name] = parsed[i]
	}
	return out, true
}

func (c *Compiler) newDeltaCompiler() *Compiler {
	dc := NewCompiler()
	dc.capabilities = c.capabilities
	dc.customBuiltins = c.customBuiltins
	dc.unsafeBuiltinsMap = c.unsafeBuiltinsMap
	dc.enablePrintStatements = c.enablePrintStatements
	dc.schemaSet = c.schemaSet
	dc.strict = c.strict
	dc.useTypeCheckAnnotations = c.useTypeCheckAnnotations
	dc.allowUndefinedFuncCalls = c.allowUndefinedFuncCalls
	dc.evalMode = c.evalMode
	dc.rewriteTestRulesForTracing = c.rewriteTestRulesForTracing
	dc.defaultRegoVersion = c.defaultRegoVersion
	dc.skipStages = c.skipStages
	dc.injectedVirtual = c.injectedVirtual
	dc.debug = c.debug
	dc.maxErrs = c.maxErrs
	return dc
}

func ruleAnnotations(m *Module) map[*Rule][]*Annotations {
	out := make(map[*Rule][]*Annotations, len(m.Rules))
	for _, a := range m.Annotations {
		if r, ok := a.node.(*Rule); ok {
			out[r] = append(out[r], a)
		}
	}
	return out
}

func (mod *Module) addRuleCopy(r *Rule, ann []*Annotations) {
	cpy := *r
	cpy.Module = mod
	if len(r.Annotations) > 0 {
		cpy.Annotations = make([]*Annotations, len(r.Annotations))
		for i, a := range r.Annotations {
			ac := *a
			ac.node = &cpy
			cpy.Annotations[i] = &ac
		}
	}
	mod.Rules = append(mod.Rules, &cpy)
	for _, a := range ann {
		ac := *a
		ac.node = &cpy
		mod.Annotations = append(mod.Annotations, &ac)
	}
}
