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
	kind := rng.Intn(14)
	if reusableOnly {
		kind = []int{0, 0, 0, 1, 2, 3, 4, 5, 11, 12, 13}[rng.Intn(11)]
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
		m, err := ParseModuleWithOpts(name, src, ParserOptions{RegoVersion: RegoV1, ProcessAnnotation: true})
		if err != nil {
			return nil, false
		}
		mods[name] = m
	}
	return mods, true
}

func compareCompilers(c, full *Compiler) string {
	if c.Failed() != full.Failed() {
		return fmt.Sprintf("failed: reuse=%v full=%v\nreuse: %v\nfull: %v", c.Failed(), full.Failed(), c.Errors, full.Errors)
	}
	if c.Failed() {
		return ""
	}
	if a, b := slices.Sorted(maps.Keys(c.Modules)), slices.Sorted(maps.Keys(full.Modules)); !slices.Equal(a, b) {
		return fmt.Sprintf("modules %v vs %v", a, b)
	}
	for name, m := range full.Modules {
		if got, exp := normalizeLocals(c.Modules[name].String()), normalizeLocals(m.String()); got != exp {
			return fmt.Sprintf("module %s:\nreuse:\n%s\nfull:\n%s", name, got, exp)
		}
	}
	var diff string
	full.RuleTree.DepthFirst(func(n *TreeNode) bool {
		if len(n.Values) == 0 || diff != "" {
			return false
		}
		ref := n.Values[0].Ref().GroundPrefix()
		if a, b := c.TypeEnv.Get(ref), full.TypeEnv.Get(ref); !typesEqual(a, b) {
			diff = fmt.Sprintf("type of %v: reuse=%v full=%v", ref, a, b)
		}
		return false
	})
	return diff
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
	f.Fuzz(func(t *testing.T, seed int64, reusableOnly, placeholders bool) {
		checkRuleReuse(t, seed, reusableOnly, placeholders)
	})
}

func TestCompilerRuleReuseFuzzSeedsCompileIncrementally(t *testing.T) {
	var n int
	for seed := range int64(100) {
		n += checkRuleReuse(t, seed, true, true)
	}
	if n == 0 {
		t.Fatal("no incremental compiles")
	}
}

// checkRuleReuse returns how many of the compiles were incremental.
func checkRuleReuse(t *testing.T, seed int64, reusableOnly, placeholders bool) int {
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
		c := NewCompiler().WithRuleReuse(prev).WithReparser(func(name string) (*Module, error) {
			m, _ := fuzzParse(map[string]string{name: files[name]})
			return m[name], nil
		})
		if placeholders {
			withPlaceholders(prev, mods)
		}
		c.Compile(mods)
		fullMods, _ := fuzzParse(files)
		full := NewCompiler()
		full.Compile(fullMods)
		// Lets the seed corpus be checked to exercise the incremental path at all.
		if c.RuleReuseStats().Incremental {
			incremental++
		}
		if d := compareCompilers(c, full); d != "" {
			var srcs []string
			for _, n := range slices.Sorted(maps.Keys(files)) {
				srcs = append(srcs, "--- "+n+"\n"+files[n])
			}
			t.Fatalf("seed %d rev %d (incremental=%v): %s\nfiles:\n%s", seed, rev, c.RuleReuseStats().Incremental, d, strings.Join(srcs, ""))
		}
		if c.Failed() {
			prev = nil
		} else {
			prev = c
		}
	}
	return incremental
}
