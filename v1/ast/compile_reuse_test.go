package ast

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"testing"
)

func spRule(id, version, body string) string {
	return fmt.Sprintf("# METADATA\n# labels:\n#   id: %q\n#   version: %q\nallow if { %s }\n\n", id, version, body)
}

func parseReuseModules(t *testing.T, files map[string]string) map[string]*Module {
	t.Helper()
	mods := make(map[string]*Module, len(files))
	for name, src := range files {
		m, err := ParseModuleWithOpts(name, src, ParserOptions{RegoVersion: RegoV1, ProcessAnnotation: true})
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

// ruleStrings returns the compiled rules at ref, sorted, for comparing against
// a full compile.
func ruleStrings(c *Compiler, ref string) []string {
	var out []string
	for _, r := range c.GetRulesExact(MustParseRef(ref)) {
		out = append(out, r.String())
	}
	slices.Sort(out)
	return out
}

const reuseMain = `package main

allow if data.sp.allow
`

func TestCompilerRuleReuse(t *testing.T) {
	sp := func(rules ...string) string { return "package sp\n\n" + strings.Join(rules, "") }
	r1 := spRule("r1", "1", `input.x == "a"`)
	r2 := spRule("r2", "1", `input.x == "b"`)
	r2v2 := spRule("r2", "2", `input.x == "c"`)
	r3 := spRule("r3", "1", `startswith(input.y, "d")`)

	tests := []struct {
		note   string
		next   map[string]string
		reused int
	}{
		{
			note:   "rule added",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2), "b.rego": sp(r3)},
			reused: 3, // main's rule too
		},
		{
			note:   "rule removed",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": sp(r1)},
			reused: 2,
		},
		{
			note:   "rule changed",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2v2)},
			reused: 2,
		},
		{
			note:   "rules moved between files",
			next:   map[string]string{"main.rego": reuseMain, "a.rego": sp(r2), "b.rego": sp(r1)},
			reused: 3,
		},
		{
			note: "ungrouped module changed",
			next: map[string]string{"main.rego": reuseMain + "\nother := 1\n", "a.rego": sp(r1, r2)},
		},
		{
			note: "added rule starts a new rule set",
			next: map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2, strings.Replace(r3, "allow if", "other if", 1))},
		},
		{
			note: "removal empties a rule set",
			next: map[string]string{"main.rego": reuseMain, "a.rego": sp(strings.Replace(spRule("r6", "1", "true"), "allow if", "other if", 1))},
		},
		{
			note: "added rule depends on grouped rules",
			next: map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2, spRule("r4", "1", "data.sp.other == 1"))},
		},
		{
			note: "added rule changes the rule set's type",
			next: map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2, strings.Replace(spRule("r5", "1", "true"), "allow if", `allow := "yes" if`, 1))},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2)}))
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
			for _, ref := range []string{"data.sp.allow", "data.sp.other", "data.main.allow"} {
				if got, exp := ruleStrings(c, ref), ruleStrings(full, ref); !slices.Equal(got, exp) {
					t.Errorf("%s: got\n%v\nwant\n%v", ref, got, exp)
				}
			}
			if got, exp := slices.Sorted(maps.Keys(c.Modules)), slices.Sorted(maps.Keys(full.Modules)); !slices.Equal(got, exp) {
				t.Fatalf("modules: got %v, want %v", got, exp)
			}
			for name, m := range full.Modules {
				if got, exp := c.Modules[name].String(), m.String(); got != exp {
					t.Errorf("module %s: got\n%s\nwant\n%s", name, got, exp)
				}
			}
			assertModuleLinks(t, c)
			for _, r := range c.GetRulesExact(MustParseRef("data.sp.allow")) {
				if c.annotationSet.Chain(r) == nil || len(c.GetAnnotationSet().GetRuleScope(r)) != 1 {
					t.Errorf("rule %v: missing annotations", r.Head)
				}
			}
		})
	}
}

func TestCompilerRuleReusePlaceholders(t *testing.T) {
	r1 := spRule("r1", "1", `input.x == "a"`)
	r2 := spRule("r2", "1", `input.x == "b"`)
	prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": "package sp\n\n" + r1}))
	placeholder := prev.ReusableRule(prev.reuseKeyOf(t, "r1"))
	if placeholder == nil {
		t.Fatal("no reusable rule for r1")
	}

	// A parser that skipped r1 hands over prev's compiled rule instead.
	withPlaceholder := func(main string) map[string]*Module {
		mods := parseReuseModules(t, map[string]string{"main.rego": main, "a.rego": "package sp\n\n" + r2})
		mods["a.rego"].Rules = append(mods["a.rego"].Rules, placeholder)
		return mods
	}

	t.Run("reused", func(t *testing.T) {
		c := compileReusing(t, prev, withPlaceholder(reuseMain))
		if s := c.RuleReuseStats(); !s.Incremental || s.ParseSkipped != 1 {
			t.Fatalf("stats %+v", s)
		}
		if c.RuleReuseStats().Reused != 2 || len(c.GetRulesExact(MustParseRef("data.sp.allow"))) != 2 { // r1 and main's rule
			t.Fatalf("reused %d, rules %v", c.RuleReuseStats().Reused, c.GetRulesExact(MustParseRef("data.sp.allow")))
		}
	})

	t.Run("full compile parses again", func(t *testing.T) {
		var reparsed []string
		c := NewCompiler().WithRuleReuse(prev).
			WithReparser(func(name string) (*Module, error) {
				reparsed = append(reparsed, name)
				return parseReuseModules(t, map[string]string{name: "package sp\n\n" + r1 + r2})[name], nil
			})
		if c.Compile(withPlaceholder(reuseMain + "\nother := 1\n")); c.Failed() {
			t.Fatal(c.Errors)
		}
		if !slices.Equal(reparsed, []string{"a.rego"}) || c.RuleReuseStats().Reused != 0 {
			t.Fatalf("reparsed %v, reused %d", reparsed, c.RuleReuseStats().Reused)
		}
		if len(c.GetRulesExact(MustParseRef("data.sp.allow"))) != 2 {
			t.Fatal("expected two rules")
		}
	})

	t.Run("full compile without reparser fails", func(t *testing.T) {
		c := NewCompiler().WithRuleReuse(prev)
		if c.Compile(withPlaceholder(reuseMain + "\nother := 1\n")); !c.Failed() {
			t.Fatal("expected an error")
		}
	})
}

func (c *Compiler) reuseKeyOf(t *testing.T, id string) string {
	t.Helper()
	for k, r := range c.reuse.rules {
		for _, a := range r.Annotations {
			if a.Labels["id"] == id {
				return k
			}
		}
	}
	return ""
}

func TestCompilerRuleReuseRequiredCapabilities(t *testing.T) {
	sp := func(rules ...string) map[string]string {
		return map[string]string{"main.rego": reuseMain, "a.rego": "package sp\n\n" + strings.Join(rules, "")}
	}
	r1 := spRule("r1", "1", `input.x == "a"`)
	prev := compileReusing(t, nil, parseReuseModules(t, sp(r1)))
	c := compileReusing(t, prev, parseReuseModules(t, sp(r1, spRule("r2", "1", `upper(input.x) == "B"`))))
	if c.RuleReuseStats().Reused != 2 {
		t.Fatalf("reused %d rules", c.RuleReuseStats().Reused)
	}
	if !slices.ContainsFunc(c.Required.Builtins, func(b *Builtin) bool { return b.Name == "upper" }) {
		t.Fatalf("upper missing from required builtins: %v", c.Required.Builtins)
	}
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
	sp := func(main string, rules ...string) map[string]*Module {
		return parseReuseModules(t, map[string]string{"main.rego": main, "a.rego": "package sp\n\n" + strings.Join(rules, "")})
	}
	r1 := spRule("r1", "1", `data.ext.p == input.x`)
	r2 := spRule("r2", "1", `data.ext.q == input.y`)
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
	prev, _ := compile(nil, sp(reuseMain, r1), "data.ext")

	t.Run("reused", func(t *testing.T) {
		c, sources := compile(prev, sp(reuseMain, r1, r2), "data.ext")
		if c.RuleReuseStats().Reused != 2 || sources[0].inits != 1 {
			t.Fatalf("reused %d rules, %d inits", c.RuleReuseStats().Reused, sources[0].inits)
		}
		if n := c.RuleTree.Find(MustParseRef("data.ext")); n == nil || n.External == nil {
			t.Fatal("no external node for data.ext")
		}
		full, _ := compile(nil, sp(reuseMain, r1, r2), "data.ext")
		if got, exp := ruleStrings(c, "data.sp.allow"), ruleStrings(full, "data.sp.allow"); !slices.Equal(got, exp) {
			t.Fatalf("got\n%v\nwant\n%v", got, exp)
		}
	})

	t.Run("fallback initializes once", func(t *testing.T) {
		c, sources := compile(prev, sp(reuseMain, r1, strings.Replace(r2, "allow if", "other if", 1)), "data.ext")
		if c.RuleReuseStats().Reused != 0 || sources[0].inits != 1 {
			t.Fatalf("reused %d rules, %d inits", c.RuleReuseStats().Reused, sources[0].inits)
		}
	})

	t.Run("external refs changed", func(t *testing.T) {
		c, _ := compile(prev, sp(reuseMain, r1, r2), "data.ext", "data.ext2")
		if c.RuleReuseStats().Reused != 0 {
			t.Fatalf("reused %d rules", c.RuleReuseStats().Reused)
		}
	})
}

func TestCompilerRuleReuseContentKey(t *testing.T) {
	compile := func(prev *Compiler, files map[string]string) *Compiler {
		t.Helper()
		c := NewCompiler().WithRuleReuse(prev)
		if c.Compile(parseReuseModules(t, files)); c.Failed() {
			t.Fatal(c.Errors)
		}
		return c
	}
	r1 := spRule("r1", "1", `input.x == "a"`)
	r2 := spRule("r2", "1", `input.x == "b"`)
	prev := compile(nil, map[string]string{"main.rego": reuseMain, "a.rego": "package sp\n\n" + r1 + r2})

	// Same labels, different text: a content key tells them apart.
	next := map[string]string{"main.rego": reuseMain, "b.rego": "package sp\n\n" + r1 + spRule("r2", "1", `input.x == "c"`)}
	c := compile(prev, next)
	if s := c.RuleReuseStats(); !s.Incremental || s.Reused != 2 { // r1 and main's rule
		t.Fatalf("stats %+v", s)
	}
	full := NewCompiler()
	if full.Compile(parseReuseModules(t, next)); full.Failed() {
		t.Fatal(full.Errors)
	}
	if got, exp := ruleStrings(c, "data.sp.allow"), ruleStrings(full, "data.sp.allow"); !slices.Equal(got, exp) {
		t.Fatalf("got\n%v\nwant\n%v", got, exp)
	}
}

// prev keeps serving queries while the next compiler is built from it, so a
// compile with reuse must only read it. Run with -race.
func TestCompilerRuleReuseLeavesPrevUntouched(t *testing.T) {
	sp := func(rules ...string) string { return "package sp\n\n" + strings.Join(rules, "") }
	r1 := spRule("r1", "1", `input.x == "a"`)
	r2 := spRule("r2", "1", `input.x == "b"`)
	prev := compileReusing(t, nil, parseReuseModules(t, map[string]string{"main.rego": reuseMain, "a.rego": sp(r1, r2), "b.rego": "package other\n\n" + spRule("o1", "1", "input.y")}))
	before := ruleStrings(prev, "data.sp.allow")

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
				for _, ref := range []string{"data.sp.allow", "data.other.allow", "data.main.allow"} {
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
			"b.rego":    "package other\n\n" + spRule("o1", "1", "input.y"),
			"a.rego":    sp(r1, spRule(fmt.Sprintf("n%d", i), "1", fmt.Sprintf("input.x == %d", i))),
		}))
		if !c.RuleReuseStats().Incremental {
			t.Fatalf("round %d: not incremental", i)
		}
		assertModuleLinks(t, c)
	}
	close(done)
	wg.Wait()

	if after := ruleStrings(prev, "data.sp.allow"); !slices.Equal(before, after) {
		t.Fatalf("prev changed:\n%v\n%v", before, after)
	}
}

// assertModuleLinks checks that rules and annotations point at the module, and
// rules, holding them.
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
	files := map[string]string{"a.rego": "package sp\n\n" + spRule("r1", "1", `input.x == "a"`)}
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
	c := compileReusing(t, prev, mods(spRule("r2", "1", `input.x == "b"`)))
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

func TestCompilerRuleReuseRecursion(t *testing.T) {
	mods := func(extra string) map[string]string {
		return map[string]string{
			"main.rego": "package main\n\nh if data.sp.allow\n\nk if data.sp.other\n",
			"a.rego":    "package sp\n\n" + spRule("r1", "1", `data.main.k`) + extra,
		}
	}
	prev := compileReusing(t, nil, parseReuseModules(t, mods("")))

	// The cycle other -> main.h -> sp.allow -> main.k -> other runs through a
	// reused rule, which the compile of the added rules doesn't see.
	c := NewCompiler().WithRuleReuse(prev)
	c.Compile(parseReuseModules(t, mods("other if data.main.h\n")))
	if !c.Failed() || !strings.Contains(c.Errors.Error(), "recursive") {
		t.Fatalf("expected a recursion error, got %v", c.Errors)
	}
	if c.RuleReuseStats().Incremental {
		t.Fatal("expected a full compile")
	}
}
