// Copyright 2026 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package ast

import (
	"context"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func labeledRule(id, version, body string) string {
	return fmt.Sprintf("# METADATA\n# labels:\n#   id: %q\n#   version: %q\nallow if { %s }\n\n", id, version, body)
}

func parseReuseModules(t *testing.T, files map[string]string) map[string]*Module {
	t.Helper()
	mods := make(map[string]*Module, len(files))
	for name, src := range files {
		m, err := ParseModuleWithOpts(name, src, ParserOptions{RegoVersion: RegoV1, ProcessAnnotation: true, DropLocations: true})
		if err != nil {
			t.Fatal(err)
		}
		mods[name] = m
	}
	return mods
}

func compileReusing(t *testing.T, prev *Compiler, mods map[string]*Module) *Compiler {
	t.Helper()
	c := NewCompiler().WithRuleReuse(prev)
	if c.Compile(mods); c.Failed() {
		t.Fatal(c.Errors)
	}
	return c
}

func ruleStrings(c *Compiler, ref string) []string {
	var out []string
	for _, r := range c.GetRulesExact(MustParseRef(ref)) {
		out = append(out, numberLocals(r).String())
	}
	slices.Sort(out)
	return out
}

func pkg(rules ...string) string { return "package gen\n\n" + strings.Join(rules, "") }

const reuseMain = `package main

allow if data.gen.allow
`

func TestCompilerRuleReuse(t *testing.T) {
	r1 := labeledRule("r1", "1", `input.x == "a"`)
	r2 := labeledRule("r2", "1", `input.x == "b"`)
	r2v2 := labeledRule("r2", "2", `input.x == "c"`)
	r3 := labeledRule("r3", "1", `startswith(input.y, "d")`)
	rx := labeledRule("rx", "1", `x := input.a; x == 1`)

	tests := []struct {
		note   string
		prev   map[string]string // defaults to main and r1, r2
		next   map[string]string
		reused int
	}{
		{
			note:   "same labels, different text",
			next:   map[string]string{"main.rego": reuseMain, "b.rego": pkg(r1, labeledRule("r2", "1", `input.x == "c"`))},
			reused: 2,
		},
		{
			note:   "added rule needs a builtin",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2, labeledRule("r4", "1", `upper(input.x) == "B"`))},
			reused: 3,
		},
		{
			note:   "added rule with an indexed comprehension",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2, labeledRule("rc", "1", `x := input.x; ys := [y | some y in input.zs; y == x]; count(ys) > 0`))},
			reused: 3,
		},
		{
			note:   "added rule with a local next to a reused one",
			prev:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(rx)},
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(rx, labeledRule("ry", "1", `y := input.b; y == 2`))},
			reused: 2,
		},
		{
			note:   "rule added",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2), "b.rego": pkg(r3)},
			reused: 3, // main's rule too
		},
		{
			note:   "rule removed",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1)},
			reused: 2,
		},
		{
			note:   "rule changed",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2v2)},
			reused: 2,
		},
		{
			note:   "rules moved between files",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": pkg(r2), "b.rego": pkg(r1)},
			reused: 3,
		},
		{
			note: "ungrouped module changed",
			next: map[string]string{"main.rego": reuseMain + "\nother := 1\n", "a.rego": pkg(r1, r2)},
		},
		{
			note: "added rule starts a new rule set",
			next: map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2, strings.Replace(r3, "allow if", "other if", 1))},
		},
		{
			note: "removal empties a rule set",
			next: map[string]string{"main.rego": reuseMain, "a.rego": pkg(strings.Replace(labeledRule("r6", "1", "true"), "allow if", "other if", 1))},
		},
		{
			note: "added rule depends on grouped rules",
			next: map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2, labeledRule("r4", "1", "data.gen.other == 1"))},
		},
		{
			note: "added rule changes the rule set's type",
			next: map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2, strings.Replace(labeledRule("r5", "1", "true"), "allow if", `allow := "yes" if`, 1))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			if tc.prev == nil {
				tc.prev = map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2)}
			}
			prev := compileReusing(t, nil, parseReuseModules(t, tc.prev))
			if prev.RuleReuseStats().Reused != 0 {
				t.Fatalf("first compile reused %d rules", prev.RuleReuseStats().Reused)
			}

			c := compileReusing(t, prev, parseReuseModules(t, tc.next))
			if c.RuleReuseStats().Reused != tc.reused {
				t.Errorf("reused %d rules, want %d", c.RuleReuseStats().Reused, tc.reused)
			}

			full := NewCompiler()
			if full.Compile(parseReuseModules(t, tc.next)); full.Failed() {
				t.Fatal(full.Errors)
			}
			for _, ref := range []string{"data.gen.allow", "data.gen.other", "data.main.allow"} {
				if got, exp := ruleStrings(c, ref), ruleStrings(full, ref); !slices.Equal(got, exp) {
					t.Errorf("%s: got\n%v\nwant\n%v", ref, got, exp)
				}
			}
			if got, exp := slices.Sorted(maps.Keys(c.Modules)), slices.Sorted(maps.Keys(full.Modules)); !slices.Equal(got, exp) {
				t.Fatalf("modules: got %v, want %v", got, exp)
			}
			for name, m := range full.Modules {
				if got, exp := numberLocals(c.Modules[name]).String(), numberLocals(m).String(); got != exp {
					t.Errorf("module %s: got\n%s\nwant\n%s", name, got, exp)
				}
				if got, exp := numberRewrittenVars(c.Modules[name], c.RewrittenVars), numberRewrittenVars(m, full.RewrittenVars); !slices.Equal(got, exp) {
					t.Errorf("module %s: rewritten vars: got %v, want %v", name, got, exp)
				}
			}
			indexed := func(c *Compiler) int {
				n := 0
				for _, m := range c.Modules {
					WalkTerms(m, func(t *Term) bool {
						if c.ComprehensionIndex(t) != nil {
							n++
						}
						return false
					})
				}
				return n
			}
			if got, exp := indexed(c), indexed(full); got != exp {
				t.Errorf("indexed comprehensions: got %d, want %d", got, exp)
			}
			for _, b := range full.Required.Builtins {
				if !slices.ContainsFunc(c.Required.Builtins, func(x *Builtin) bool { return x.Name == b.Name }) {
					t.Errorf("required builtins: %s missing", b.Name)
				}
			}
			if a, b := len(c.RewrittenVars), len(full.RewrittenVars); a != b {
				t.Errorf("%d rewritten vars, want %d", a, b)
			}
			assertModuleLinks(t, c)
			for _, r := range c.GetRulesExact(MustParseRef("data.gen.allow")) {
				if c.annotationSet.Chain(r) == nil || len(c.GetAnnotationSet().GetRuleScope(r)) != 1 {
					t.Errorf("rule %v: missing annotations", r.Head)
				}
			}
		})
	}
}

func TestCompilerRuleReusePlaceholders(t *testing.T) {
	r1 := labeledRule("r1", "1", `input.x == "a"`)
	r2 := labeledRule("r2", "1", `input.x == "b"`)
	prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": "package gen\n\n" + r1}))
	placeholder := prev.ReusableRule(prev.reuseKeyOf(t, "r1"))
	if placeholder == nil {
		t.Fatal("no reusable rule for r1")
	}

	// A parser that skipped r1 hands over prev's compiled rule instead.
	withPlaceholder := func(main string) map[string]*Module {
		mods := parseReuseModules(t, map[string]string{"main.rego": main, "a.rego": "package gen\n\n" + r2})
		mods["a.rego"].Rules = append(mods["a.rego"].Rules, placeholder)
		return mods
	}

	t.Run("reused", func(t *testing.T) {
		c := compileReusing(t, prev, withPlaceholder(reuseMain))
		if s := c.RuleReuseStats(); !s.Incremental || s.ParseSkipped != 1 {
			t.Fatalf("stats %+v", s)
		}
		if c.RuleReuseStats().Reused != 2 || len(c.GetRulesExact(MustParseRef("data.gen.allow"))) != 2 { // r1 and main's rule
			t.Fatalf("reused %d, rules %v", c.RuleReuseStats().Reused, c.GetRulesExact(MustParseRef("data.gen.allow")))
		}
	})

	t.Run("full compile parses again", func(t *testing.T) {
		var reparsed []string
		c := NewCompiler().WithRuleReuse(prev).
			WithReparser(func(name string) (*Module, error) {
				reparsed = append(reparsed, name)
				return parseReuseModules(t, map[string]string{name: "package gen\n\n" + r1 + r2})[name], nil
			})
		if c.Compile(withPlaceholder(reuseMain + "\nother := 1\n")); c.Failed() {
			t.Fatal(c.Errors)
		}
		if !slices.Equal(reparsed, []string{"a.rego"}) || c.RuleReuseStats().Reused != 0 {
			t.Fatalf("reparsed %v, reused %d", reparsed, c.RuleReuseStats().Reused)
		}
		if len(c.GetRulesExact(MustParseRef("data.gen.allow"))) != 2 {
			t.Fatal("expected two rules")
		}
	})

	t.Run("full compile without reparser fails", func(t *testing.T) {
		c := NewCompiler().WithRuleReuse(prev)
		if c.Compile(withPlaceholder(reuseMain + "\nother := 1\n")); !c.Failed() {
			t.Fatal("expected an error")
		}
	})

	// The key a module got when parsed doesn't cover the rules a parser appended.
	t.Run("in a module whose source is unchanged", func(t *testing.T) {
		lib := "package gen\n\nf(x) if x\n"
		prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2), "lib.rego": lib}))

		// r2 moved into lib.rego, and a parser skipped it there.
		mods := parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1), "lib.rego": lib})
		mods["lib.rego"].Rules = append(mods["lib.rego"].Rules, prev.ReusableRule(prev.reuseKeyOf(t, "r2")))
		c := NewCompiler().WithRuleReuse(prev).WithReparser(func(name string) (*Module, error) {
			return parseReuseModules(t, map[string]string{name: lib + "\n" + r2})[name], nil
		})
		if c.Compile(mods); c.Failed() {
			t.Fatal(c.Errors)
		}
		if rules := c.GetRulesExact(MustParseRef("data.gen.allow")); len(rules) != 2 {
			t.Fatalf("expected r1 and r2, got %v", rules)
		}
	})
}

func (c *Compiler) reuseKeyOf(t *testing.T, id string) RuleKey {
	t.Helper()
	for k, r := range c.reuse.rules {
		for _, a := range r.Annotations {
			if a.Labels["id"] == id {
				return k
			}
		}
	}
	return RuleKey{}
}

type countingSource struct {
	ref   Ref
	inits int
}

func (s *countingSource) Refs() []Ref { return []Ref{s.ref} }

func (s *countingSource) Init(context.Context, Ref) (ExternalRuleIndex, error) {
	s.inits++
	return &mockExternalIndex{callCount: new(int32)}, nil
}

func TestCompilerRuleReuseExternalSources(t *testing.T) {
	pkg := func(main string, rules ...string) map[string]*Module {
		return parseReuseModules(t, map[string]string{"main.rego": main, "a.rego": "package gen\n\n" + strings.Join(rules, "")})
	}
	r1 := labeledRule("r1", "1", `data.ext.p == input.x`)
	r2 := labeledRule("r2", "1", `data.ext.q == input.y`)
	compile := func(prev *Compiler, mods map[string]*Module, refs ...string) (*Compiler, []*countingSource) {
		t.Helper()
		c := NewCompiler().WithRuleReuse(prev)
		var sources []*countingSource
		for _, r := range refs {
			s := &countingSource{ref: MustParseRef(r)}
			sources = append(sources, s)
			c.WithExternalSource(s.ref, s)
		}
		if c.Compile(mods); c.Failed() {
			t.Fatal(c.Errors)
		}
		return c, sources
	}
	prev, _ := compile(nil, pkg(reuseMain, r1), "data.ext")

	t.Run("reused", func(t *testing.T) {
		c, sources := compile(prev, pkg(reuseMain, r1, r2), "data.ext")
		if c.RuleReuseStats().Reused != 2 || sources[0].inits != 1 {
			t.Fatalf("reused %d rules, %d inits", c.RuleReuseStats().Reused, sources[0].inits)
		}
		if n := c.RuleTree.Find(MustParseRef("data.ext")); n == nil || n.External == nil {
			t.Fatal("no external node for data.ext")
		}
		full, _ := compile(nil, pkg(reuseMain, r1, r2), "data.ext")
		if got, exp := ruleStrings(c, "data.gen.allow"), ruleStrings(full, "data.gen.allow"); !slices.Equal(got, exp) {
			t.Fatalf("got\n%v\nwant\n%v", got, exp)
		}
	})

	t.Run("fallback initializes once", func(t *testing.T) {
		c, sources := compile(prev, pkg(reuseMain, r1, strings.Replace(r2, "allow if", "other if", 1)), "data.ext")
		if c.RuleReuseStats().Reused != 0 || sources[0].inits != 1 {
			t.Fatalf("reused %d rules, %d inits", c.RuleReuseStats().Reused, sources[0].inits)
		}
	})

	t.Run("external refs changed", func(t *testing.T) {
		c, _ := compile(prev, pkg(reuseMain, r1, r2), "data.ext", "data.ext2")
		if c.RuleReuseStats().Reused != 0 {
			t.Fatalf("reused %d rules", c.RuleReuseStats().Reused)
		}
	})
}

// prev keeps serving queries while the next compiler is built from it, so a
// compile with reuse must only read it. Run with -race.
func TestCompilerRuleReuseLeavesPrevUntouched(t *testing.T) {
	r1 := labeledRule("r1", "1", `input.x == "a"`)
	r2 := labeledRule("r2", "1", `input.x == "b"`)
	prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, r2), "b.rego": "package other\n\n" + labeledRule("o1", "1", "input.y")}))
	before := ruleStrings(prev, "data.gen.allow")

	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				for _, ref := range []string{"data.gen.allow", "data.other.allow", "data.main.allow"} {
					for _, r := range prev.GetRulesExact(MustParseRef(ref)) {
						_ = r.String()
						_ = r.Ref()
					}
					_ = prev.TypeEnv.Get(MustParseRef(ref))
				}
				for _, m := range prev.Modules {
					WalkRules(m, func(r *Rule) bool { _ = r.Module.Package.Path.String(); return false })
				}
			}
		})
	}

	for i := range 20 {
		c := compileReusing(t, prev, parseReuseModules(t, map[string]string{
			"main.rego": reuseMain,
			"b.rego":    "package other\n\n" + labeledRule("o1", "1", "input.y"),
			"a.rego":    pkg(r1, labeledRule(fmt.Sprintf("n%d", i), "1", fmt.Sprintf("input.x == %d", i))),
		}))
		if !c.RuleReuseStats().Incremental {
			t.Fatalf("round %d: not incremental", i)
		}
		assertModuleLinks(t, c)
	}
	close(done)
	wg.Wait()

	if after := ruleStrings(prev, "data.gen.allow"); !slices.Equal(before, after) {
		t.Fatalf("prev changed:\n%v\n%v", before, after)
	}
}

func TestCompilerRuleReuseReleasesPrev(t *testing.T) {
	r1 := labeledRule("r1", "1", `input.x == "a"`)
	other := "package other\n\n" + labeledRule("o1", "1", "input.y")
	prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1), "b.rego": other}))
	var released atomic.Bool
	runtime.AddCleanup(prev, func(*atomic.Bool) { released.Store(true) }, &released)

	// data.other.allow and data.main.allow are unchanged, so their indices are carried over.
	c := compileReusing(t, prev, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": pkg(r1, labeledRule("r2", "1", `input.x == "b"`)), "b.rego": other}))
	if !c.RuleReuseStats().Incremental {
		t.Fatal("not incremental")
	}
	for range 50 {
		if runtime.GC(); released.Load() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !released.Load() {
		t.Fatal("the compiler reused from is still reachable from the new one")
	}
	runtime.KeepAlive(c)
}

func assertModuleLinks(t *testing.T, c *Compiler) {
	t.Helper()
	for name, m := range c.Modules {
		for _, r := range m.Rules {
			if r.Module != m {
				t.Fatalf("%s: rule %v points at another module", name, r.Head.Ref())
			}
			for _, a := range r.Annotations {
				if a.node != r {
					t.Fatalf("%s: annotation of %v points at another rule", name, r.Head.Ref())
				}
			}
		}
		for _, a := range m.Annotations {
			if r, ok := a.node.(*Rule); ok && !slices.Contains(m.Rules, r) {
				t.Fatalf("%s: annotation points at a rule of another module", name)
			}
		}
	}
}

func TestCompilerRuleReuseModulesWithoutSource(t *testing.T) {
	// A module built in Go: no source text anywhere.
	built := func() *Module {
		return &Module{
			Package: &Package{Path: MustParseRef("data.built")},
			Rules:   []*Rule{{Head: NewHead(Var("x"), nil, IntNumberTerm(1)), Body: NewBody(NewExpr(BooleanTerm(true)))}},
		}
	}
	files := map[string]string{"a.rego": "package gen\n\n" + labeledRule("r1", "1", `input.x == "a"`)}
	mods := func(extra ...string) map[string]*Module {
		m := parseReuseModules(t, files)
		m["built.rego"] = built()
		m["built.rego"].Rules[0].Module = m["built.rego"]
		for _, e := range extra {
			m["a.rego"] = parseReuseModules(t, map[string]string{"a.rego": files["a.rego"] + e})["a.rego"]
		}
		return m
	}
	prev := compileReusing(t, nil, mods())
	c := compileReusing(t, prev, mods(labeledRule("r2", "1", `input.x == "b"`)))
	if !c.RuleReuseStats().Incremental {
		t.Fatal("not incremental")
	}
}

func TestCompilerRuleReuseMatchesFullCompile(t *testing.T) {
	tests := []struct {
		note       string
		prev, next map[string]string
		opts       func(*Compiler) *Compiler
		fails      bool
	}{
		{
			note: "added rule closes a cycle through a function",
			prev: map[string]string{
				"a.rego": "package a\n\np if r\n\nr if input.x\n",
				"b.rego": "package b\n\nf(x) := data.a.p == x\n",
			},
			next: map[string]string{
				"a.rego": "package a\n\np if r\n\nr if input.x\n\nr if data.b.f(1)\n",
				"b.rego": "package b\n\nf(x) := data.a.p == x\n",
			},
			fails: true,
		},
		{
			note: "added rule's type error is hidden by a function",
			prev: map[string]string{
				"a.rego": "package a\n\np contains 1\n\nr := input.x\n",
				"b.rego": "package b\n\nf(x) := data.a.p\n",
			},
			next: map[string]string{
				"a.rego": "package a\n\np contains 1\n\nr := input.x\n\nr := object.get(data.b.f(1), \"k\", 1)\n",
				"b.rego": "package b\n\nf(x) := data.a.p\n",
			},
			fails: true,
		},
		{
			note:  "removed rule leaves an import unused in strict mode",
			opts:  func(c *Compiler) *Compiler { return c.WithStrict(true) },
			prev:  map[string]string{"a.rego": "package a\n\nimport data.z\n\np if z.x\n\np if input.y\n"},
			next:  map[string]string{"a.rego": "package a\n\nimport data.z\n\np if input.y\n"},
			fails: true,
		},
		{
			note: "added rule closes a cycle through a reused rule",
			prev: map[string]string{
				"main.rego": "package main\n\nh if data.gen.allow\n\nk if data.gen.other\n",
				"a.rego":    "package gen\n\n" + labeledRule("r1", "1", `data.main.k`),
			},
			next: map[string]string{
				"main.rego": "package main\n\nh if data.gen.allow\n\nk if data.gen.other\n",
				"a.rego":    "package gen\n\n" + labeledRule("r1", "1", `data.main.k`) + "other if data.main.h\n",
			},
			fails: true,
		},
		{
			note: "document annotation with identical rules in two modules",
			prev: map[string]string{
				"f0.rego": "package b\n\nallow if input.x\n",
				"f1.rego": "package b\n\n# METADATA\n# scope: document\n# title: d\nv := 1\n\nv := 1\n",
			},
			next: map[string]string{
				"f0.rego": "package b\n\n# METADATA\n# scope: document\n# title: d\nv := 1\n\nallow if input.x\n",
				"f1.rego": "package b\n\nv := 1\n",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			opts := tc.opts
			if opts == nil {
				opts = func(c *Compiler) *Compiler { return c }
			}
			prev := opts(NewCompiler().WithRuleReuse(nil))
			if prev.Compile(parseReuseModules(t, tc.prev)); prev.Failed() {
				t.Fatal(prev.Errors)
			}
			c := opts(NewCompiler().WithRuleReuse(prev))
			c.Compile(parseReuseModules(t, tc.next))
			full := opts(NewCompiler())
			full.Compile(parseReuseModules(t, tc.next))
			if full.Failed() != tc.fails {
				t.Fatalf("full compile: failed %v", full.Failed())
			}
			if c.Failed() != full.Failed() {
				t.Fatalf("reuse failed: %v, full failed: %v\n%v", c.Failed(), full.Failed(), c.Errors)
			}
			if !c.Failed() {
				for _, name := range slices.Sorted(maps.Keys(full.Modules)) {
					for i, r := range full.Modules[name].Rules {
						if got, exp := len(c.GetAnnotationSet().Chain(c.Modules[name].Rules[i])), len(full.GetAnnotationSet().Chain(r)); got != exp {
							t.Errorf("%s: %v: annotation chain has %d entries, want %d", name, r.Head, got, exp)
						}
					}
				}
			}
		})
	}
}

func TestCompilerRuleReuseRewrittenVars(t *testing.T) {
	rx := labeledRule("rx", "1", `x := input.a; x == 1`)
	ry := labeledRule("ry", "1", `y := input.b; y == 2`)
	rt := labeledRule("rt", "1", `count(input.zs) > 0`) // its local isn't a rewritten var
	rc := labeledRule("rc", "1", `x := input.x; ys := [y | some y in input.zs; y == x]; count(ys) > 0`)
	r1 := labeledRule("r1", "1", `input.x == "a"`)

	tests := []struct {
		note        string
		prev, next  string
		incremental bool
	}{
		{note: "rule with locals removed", prev: pkg(rx, r1), next: pkg(r1), incremental: true},
		{note: "rule with an indexed comprehension removed", prev: pkg(rc, r1), next: pkg(r1), incremental: true},
		{note: "added rule's locals next to a reused rule's", prev: pkg(rt), next: pkg(rt, ry), incremental: true},
		// Only one of them was reusable, so the other's removal would go unnoticed.
		{note: "one of two identical rules removed", prev: pkg(rx, rx, r1), next: pkg(rx, r1)},
	}
	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"a.rego": tc.prev}))
			c := compileReusing(t, prev, parseReuseModules(t, map[string]string{"a.rego": tc.next}))
			if c.RuleReuseStats().Incremental != tc.incremental {
				t.Fatalf("incremental: %v", c.RuleReuseStats().Incremental)
			}
			full := NewCompiler()
			if full.Compile(parseReuseModules(t, map[string]string{"a.rego": tc.next})); full.Failed() {
				t.Fatal(full.Errors)
			}
			if a, b := len(c.RewrittenVars), len(full.RewrittenVars); a != b {
				t.Errorf("%d rewritten vars, want %d: %v", a, b, c.RewrittenVars)
			}
			if a, b := len(c.comprehensionIndices), len(full.comprehensionIndices); a != b {
				t.Errorf("%d comprehension indices, want %d", a, b)
			}
			got, exp := c.Modules["a.rego"], full.Modules["a.rego"]
			if !numberLocals(got).Equal(numberLocals(exp)) {
				t.Errorf("module: got\n%v\nwant\n%v", got, exp)
			}
			if got, exp := numberRewrittenVars(got, c.RewrittenVars), numberRewrittenVars(exp, full.RewrittenVars); !slices.Equal(got, exp) {
				t.Errorf("rewritten vars: got %v, want %v", got, exp)
			}
		})
	}
}

func TestCompilerRuleReusePathConflicts(t *testing.T) {
	exists := false
	opts := func(c *Compiler) *Compiler {
		return c.WithPathConflictsCheck(func(p []string) (bool, error) {
			return exists && len(p) == 2 && p[0] == "a" && p[1] == "p", nil
		})
	}
	mods := map[string]string{"a.rego": "package a\n\np if input.x\n\np if input.y\n"}
	prev := opts(NewCompiler().WithRuleReuse(nil))
	if prev.Compile(parseReuseModules(t, mods)); prev.Failed() {
		t.Fatal(prev.Errors)
	}
	exists = true
	c := opts(NewCompiler().WithRuleReuse(prev))
	if c.Compile(parseReuseModules(t, mods)); !c.Failed() {
		t.Fatal("expected a path conflict")
	}
}

func TestReuseKeyBoundaries(t *testing.T) {
	keys := map[RuleKey]string{}
	for note, k := range map[string]RuleKey{
		"package ab, c := 10":       ReuseKey("data.ab", []byte("c := 10")),
		"package a, bc := 10":       ReuseKey("data.a", []byte("bc := 10")),
		"METADATA ab, rule c":       ReuseKey("data.a", []byte("ab"), []byte("c")),
		"METADATA a, rule bc":       ReuseKey("data.a", []byte("a"), []byte("bc")),
		"no METADATA, rule abc":     ReuseKey("data.a", []byte("abc")),
		"empty METADATA, rule abc":  ReuseKey("data.a", []byte(""), []byte("abc")),
		"package a.b, rule c":       ReuseKey("data.a.b", []byte("c")),
		"package a, rule b.c (sic)": ReuseKey("data.a", []byte(".b"), []byte("c")),
	} {
		if other, ok := keys[k]; ok {
			t.Fatalf("%s and %s share a key", note, other)
		}
		keys[k] = note
	}
}
