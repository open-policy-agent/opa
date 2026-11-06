// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"fmt"
	"maps"
	"math/rand"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var localVarRE = regexp.MustCompile(`__local\d+__`)

// Rules compiled in different compiles get different generated names.
func normalizeLocals(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		seen := map[string]string{}
		lines[i] = localVarRE.ReplaceAllStringFunc(l, func(v string) string {
			if _, ok := seen[v]; !ok {
				seen[v] = fmt.Sprintf("__L%d__", len(seen))
			}
			return seen[v]
		})
	}
	return strings.Join(lines, "\n")
}

type fuzzRule struct {
	pkg, text string
}

// withPlaceholders does what a reuse parser does.
func withPlaceholders(prev *Compiler, mods map[string]*Module) int {
	if prev == nil {
		return 0
	}
	n := 0
	for _, m := range mods {
		replaced := map[*Rule]bool{}
		for i, r := range m.Rules {
			if k, ok := reuseKey(r); ok {
				if pr := prev.ReusableRule(k); pr != nil {
					replaced[r] = true
					m.Rules[i] = pr
					n++
				}
			}
		}
		m.Annotations = slices.DeleteFunc(m.Annotations, func(a *Annotations) bool {
			r, ok := a.node.(*Rule)
			return ok && replaced[r]
		})
	}
	return n
}

func genRule(rng *rand.Rand, id int, reusableOnly bool) fuzzRule {
	pkgs := []string{"a", "b", "c"}
	pkg := pkgs[rng.Intn(len(pkgs))]
	other := pkgs[rng.Intn(len(pkgs))]
	var meta string
	switch rng.Intn(6) {
	case 0:
		meta = fmt.Sprintf("# METADATA\n# title: t%d\n", id)
	case 1:
		meta = fmt.Sprintf("# METADATA\n# scope: document\n# title: d%d\n", id)
	}
	var body string
	kind := rng.Intn(15)
	if reusableOnly {
		kind = []int{0, 0, 0, 1, 2, 3, 4, 5, 11, 12, 13, 14}[rng.Intn(12)]
	}
	switch kind {
	case 0:
		body = fmt.Sprintf("allow if input.x == %d", id%7)
	case 1:
		body = fmt.Sprintf("allow if data.%s.v > %d", other, id%5)
	case 2:
		body = fmt.Sprintf("v := %d", id%3)
	case 3:
		body = fmt.Sprintf("v := \"s%d\"", id%2)
	case 4:
		body = fmt.Sprintf("s contains %d if input.y", id%4)
	case 5:
		body = fmt.Sprintf("s contains x if { some x in input.zs; x > %d }", id%3)
	case 6:
		body = fmt.Sprintf("f(x) := x + %d", id%2)
	case 7:
		body = fmt.Sprintf("allow if data.%s.f(1) > %d", other, id%3)
	case 8:
		body = "default allow := false"
	case 9:
		body = fmt.Sprintf("w := 1 if input.q == %d\nelse := 2", id%3)
	case 10:
		body = fmt.Sprintf("m := rego.metadata.chain() if input.m == %d", id%2)
	case 11:
		body = fmt.Sprintf("allow if count(data.%s.s) > %d", other, id%3)
	case 12:
		body = fmt.Sprintf("allow if { x := data.%s.v; x == %d }", other, id%3)
	case 13:
		body = fmt.Sprintf("o.k%d := %d", id%3, id%4)
	case 14:
		body = fmt.Sprintf("g contains n if { some k in input.zs; n := count([z | some z in input.zs; z == k]) + %d }", id%2)
	}
	return fuzzRule{pkg: pkg, text: meta + body}
}

func fuzzModules(rules []fuzzRule, files int, rng *rand.Rand) map[string]string {
	out := map[string]string{}
	for _, r := range rules {
		name := fmt.Sprintf("%s/f%d.rego", r.pkg, rng.Intn(files))
		if out[name] == "" {
			out[name] = "package " + r.pkg + "\n\n"
			if rng.Intn(10) == 0 {
				out[name] += "import data.b as bb\n\n"
			}
		}
		out[name] += r.text + "\n\n"
	}
	return out
}

func fuzzParse(files map[string]string) (map[string]*Module, bool) {
	mods := map[string]*Module{}
	for name, src := range files {
		m, err := ParseModuleWithOpts(name, src, ParserOptions{RegoVersion: RegoV1, ProcessAnnotation: true, DropLocations: true})
		if err != nil {
			return nil, false
		}
		mods[name] = m
	}
	return mods, true
}

// compareCompilers compares c to full, a compile from scratch of the same
// modules, in what eval reads of a compiler.
func compareCompilers(c, full *Compiler) string {
	errs := func(c *Compiler) string {
		var out []string
		for _, e := range c.Errors {
			out = append(out, normalizeLocals(e.Error()))
		}
		slices.Sort(out)
		return strings.Join(out, "\n")
	}
	if got, exp := errs(c), errs(full); got != exp {
		return fmt.Sprintf("errors:\ngot:\n%s\nfull:\n%s", got, exp)
	}
	if c.Failed() {
		return ""
	}
	if a, b := slices.Sorted(maps.Keys(c.Modules)), slices.Sorted(maps.Keys(full.Modules)); !slices.Equal(a, b) {
		return fmt.Sprintf("modules %v vs %v", a, b)
	}
	for name, m := range full.Modules {
		if got, exp := numberLocals(c.Modules[name]).String(), numberLocals(m).String(); got != exp {
			return fmt.Sprintf("module %s:\ngot:\n%s\nfull:\n%s", name, got, exp)
		}
		if got, exp := numberRewrittenVars(c.Modules[name], c.RewrittenVars), numberRewrittenVars(m, full.RewrittenVars); !slices.Equal(got, exp) {
			return fmt.Sprintf("module %s, rewritten vars:\ngot:  %v\nfull: %v", name, got, exp)
		}
		for i, r := range m.Rules {
			if got, exp := comprehensionIndicesOf(c, c.Modules[name].Rules[i]), comprehensionIndicesOf(full, r); !slices.Equal(got, exp) {
				return fmt.Sprintf("module %s, rule %d, comprehension indices:\ngot:  %v\nfull: %v", name, i, got, exp)
			}
		}
	}
	// Entries for rules that are gone would keep them alive.
	if a, b := len(c.RewrittenVars), len(full.RewrittenVars); a != b {
		return fmt.Sprintf("%d rewritten vars, full has %d", a, b)
	}
	if a, b := len(c.comprehensionIndices), len(full.comprehensionIndices); a != b {
		return fmt.Sprintf("%d comprehension indices, full has %d", a, b)
	}
	// Incremental compiles keep what removed rules required, which only decides
	// whether the compile checks for deprecated built-ins.
	required := func(c *Compiler) []string {
		var out []string
		for _, b := range c.Required.Builtins {
			out = append(out, "builtin "+b.Name)
		}
		for _, f := range c.Required.Features {
			out = append(out, "feature "+f)
		}
		for _, k := range c.Required.FutureKeywords {
			out = append(out, "keyword "+k)
		}
		return out
	}
	if got, exp := required(c), required(full); slices.ContainsFunc(exp, func(r string) bool { return !slices.Contains(got, r) }) {
		return fmt.Sprintf("required: got %v, full %v", got, exp)
	}
	annotations := func(c *Compiler) string {
		var out []string
		for _, e := range c.GetAnnotationSet().Flatten() {
			out = append(out, e.Path.String()+" "+e.Annotations.Title+" "+fmt.Sprint(e.Annotations.Labels))
		}
		slices.Sort(out)
		return strings.Join(out, "\n")
	}
	if got, exp := annotations(c), annotations(full); got != exp {
		return fmt.Sprintf("annotations:\ngot:\n%s\nfull:\n%s", got, exp)
	}
	var diff string
	full.RuleTree.DepthFirst(func(n *TreeNode) bool {
		if len(n.Values) == 0 || diff != "" {
			return false
		}
		ref := n.Values[0].Ref().GroundPrefix()
		if a, b := c.TypeEnv.Get(ref), full.TypeEnv.Get(ref); !typesEqual(a, b) {
			diff = fmt.Sprintf("type of %v: got %v, full %v", ref, a, b)
			return true
		}
		for _, input := range oracleInputs {
			if got, exp := indexLookup(c, ref, input), indexLookup(full, ref, input); got != exp {
				diff = fmt.Sprintf("index of %v, input %v:\ngot:  %s\nfull: %s", ref, input, got, exp)
				return true
			}
		}
		return false
	})
	return diff
}

// comprehensionIndicesOf lists the keys of the comprehension indices c holds
// for r, in the names numberLocals gives r's locals.
func comprehensionIndicesOf(c *Compiler, r *Rule) []string {
	names, _ := localNames(r)
	var out []string
	WalkTerms(r, func(t *Term) bool {
		switch t.Value.(type) {
		case *ArrayComprehension, *SetComprehension, *ObjectComprehension:
			ci := c.ComprehensionIndex(t)
			if ci == nil {
				out = append(out, "none")
				return false
			}
			keys := make([]string, len(ci.Keys))
			for i, k := range ci.Keys {
				keys[i] = k.String()
				if v, ok := k.Value.(Var); ok && names[v] != "" {
					keys[i] = string(names[v])
				}
			}
			out = append(out, strings.Join(keys, ", "))
		}
		return false
	})
	return out
}

var oracleInputs = func() []*Term {
	var out []*Term
	for i := range 7 {
		out = append(out, MustParseTerm(fmt.Sprintf(`{"x": %d, "y": %v, "zs": [%d, 1], "q": %d, "m": %d}`, i, i%2 == 0, i%3, i%3, i%2)))
	}
	return out
}()

type oracleResolver struct{ input *Term }

func (r oracleResolver) Resolve(ref Ref) (Value, error) {
	if !ref.HasPrefix(InputRootRef) {
		return nil, UnknownValueErr{}
	}
	v, err := r.input.Value.Find(ref[1:])
	if err != nil {
		return nil, nil
	}
	return v, nil
}

func indexLookup(c *Compiler, ref Ref, input *Term) string {
	idx := c.RuleIndex(ref)
	if idx == nil {
		return "no index"
	}
	res, err := idx.Lookup(oracleResolver{input})
	if err != nil {
		return "error: " + err.Error()
	}
	var b strings.Builder
	for _, r := range res.Rules {
		b.WriteString(r.String())
		for _, e := range res.Else[r] {
			b.WriteString(" else " + e.String())
		}
		b.WriteString("; ")
	}
	if res.Default != nil {
		b.WriteString("default " + res.Default.String())
	}
	fmt.Fprintf(&b, " early exit: %v", res.EarlyExit)
	return normalizeLocals(b.String())
}

// An incrementalCompile compiles mods, parsed from files, with what it kept of
// prev, the compile of the revision before (nil for the first). It reports
// whether it compiled incrementally.
type incrementalCompile func(prev *Compiler, files map[string]string, mods map[string]*Module) (*Compiler, bool)

func ruleReuseCompile(placeholders bool) incrementalCompile {
	return func(prev *Compiler, files map[string]string, mods map[string]*Module) (*Compiler, bool) {
		c := NewCompiler().WithRuleReuse(prev).WithReparser(func(name string) (*Module, error) {
			m, _ := fuzzParse(map[string]string{name: files[name]})
			return m[name], nil
		})
		if placeholders {
			withPlaceholders(prev, mods)
		}
		c.Compile(mods)
		return c, c.RuleReuseStats().Incremental
	}
}

// FuzzCompilerRuleReuse compiles a sequence of revisions derived from seed,
// each with reuse from the one before and from scratch, and compares them.
func FuzzCompilerRuleReuse(f *testing.F) {
	for seed := range int64(100) {
		for _, reusableOnly := range []bool{false, true} {
			for _, placeholders := range []bool{false, true} {
				f.Add(seed, reusableOnly, placeholders)
			}
		}
	}
	f.Add(int64(428), true, false)  // added rule's local named like a reused rule's
	f.Add(int64(1488), false, true) // one of two identical rules removed
	f.Fuzz(func(t *testing.T, seed int64, reusableOnly, placeholders bool) {
		checkIncrementalCompile(t, seed, reusableOnly, ruleReuseCompile(placeholders))
	})
}

func TestCompilerRuleReuseFuzzSeedsCompileIncrementally(t *testing.T) {
	var n int
	for seed := range int64(100) {
		n += checkIncrementalCompile(t, seed, true, ruleReuseCompile(true))
	}
	if n == 0 {
		t.Fatal("no incremental compiles")
	}
}

// checkIncrementalCompile compiles a sequence of revisions derived from seed
// with compile and from scratch, compares them, and returns how many of the
// compiles were incremental.
func checkIncrementalCompile(t *testing.T, seed int64, reusableOnly bool, compile incrementalCompile) int {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	var rules []fuzzRule
	next := 0
	for range 3 + rng.Intn(8) {
		rules = append(rules, genRule(rng, next, reusableOnly))
		next++
	}
	incremental := 0
	var prev *Compiler
	for rev := range 6 {
		if rev > 0 {
			switch rng.Intn(4) {
			case 0:
				rules = append(rules, genRule(rng, next, reusableOnly))
				next++
			case 1:
				if len(rules) > 1 {
					i := rng.Intn(len(rules))
					rules = slices.Delete(rules, i, i+1)
				}
			case 2:
				rules[rng.Intn(len(rules))] = genRule(rng, next, reusableOnly)
				next++
			case 3:
				rng.Shuffle(len(rules), func(i, j int) { rules[i], rules[j] = rules[j], rules[i] })
			}
		}
		files := fuzzModules(rules, 1+rng.Intn(3), rng)
		mods, ok := fuzzParse(files)
		if !ok {
			prev = nil
			continue
		}
		c, inc := compile(prev, files, mods)
		fullMods, _ := fuzzParse(files)
		full := NewCompiler()
		full.Compile(fullMods)
		// Lets the seed corpus be checked to exercise the incremental path at all.
		if inc {
			incremental++
		}
		if d := compareCompilers(c, full); d != "" {
			var srcs []string
			for _, n := range slices.Sorted(maps.Keys(files)) {
				srcs = append(srcs, "--- "+n+"\n"+files[n])
			}
			t.Fatalf("seed %d rev %d (incremental=%v): %s\nfiles:\n%s", seed, rev, inc, d, strings.Join(srcs, ""))
		}
		if c.Failed() {
			prev = nil
		} else {
			prev = c
		}
	}
	return incremental
}
