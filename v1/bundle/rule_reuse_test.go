package bundle

import (
	"bytes"
	"slices"
	"testing"

	"github.com/open-policy-agent/opa/internal/file/archive"
	"github.com/open-policy-agent/opa/v1/ast"
)

func TestReuseModuleParser(t *testing.T) {
	popts := ast.ParserOptions{RegoVersion: ast.RegoV1, ProcessAnnotation: true}
	rule := func(id, body string) string {
		return "# METADATA\n# labels:\n#   id: \"" + id + "\"\nallow if { " + body + " }\n\n"
	}
	header := "package sp\n\nimport rego.v1\n\n"
	r1 := rule("r1", `input.x == "a"`)
	r2 := rule("r2", `input.x == "b"`)
	multi := "allow if {\n\t# inside\n\tinput.y == `{\n}`\n}\n"
	prevSrc := header + r1 + r2 + multi + "allow if input.w\nallow if input.v\n"

	prevMod, err := ast.ParseModuleWithOpts("a.rego", prevSrc, popts)
	if err != nil {
		t.Fatal(err)
	}
	prev := ast.NewCompiler().WithRuleReuse(nil)
	if prev.Compile(map[string]*ast.Module{"a.rego": prevMod}); prev.Failed() {
		t.Fatal(prev.Errors)
	}
	parse := reuseModuleParser(func() *ast.Compiler { return prev })

	tests := []struct {
		note   string
		src    string
		reused int
		parsed []int // rows of the rules that were parsed
	}{
		{
			note:   "same source",
			src:    prevSrc,
			reused: 5,
			parsed: []int{},
		},
		{
			note:   "rule added",
			src:    header + r2 + rule("r3", `input.x == "c"`) + r1,
			reused: 2,
			parsed: []int{13},
		},
		{
			note:   "rule changed",
			src:    header + rule("r1", `input.x == "z"`) + r2,
			reused: 1,
			parsed: []int{8},
		},
		{
			note:   "comment after the rule",
			src:    header + r1[:len(r1)-1] + "# trailing\n\n" + r2,
			reused: 2,
			parsed: []int{},
		},
		{
			note:   "comment on the rule's line",
			src:    header + "allow if input.w # c\n",
			parsed: []int{5},
		},
	}

	for _, tc := range tests {
		t.Run(tc.note, func(t *testing.T) {
			mod, err := parse("a.rego", []byte(tc.src), popts)
			if err != nil {
				t.Fatal(err)
			}
			var reused int
			rows := []int{}
			for _, r := range mod.Rules {
				if slices.Contains(prev.GetRulesExact(r.Ref()), r) {
					reused++
				} else {
					rows = append(rows, r.Location.Row)
				}
			}
			if reused != tc.reused || !slices.Equal(rows, tc.parsed) {
				t.Fatalf("reused %d, parsed rows %v; want %d, %v", reused, rows, tc.reused, tc.parsed)
			}
		})
	}
}

func TestStatements(t *testing.T) {
	src := "package p\n\n# plain\n# METADATA\n# title: x\na if {\n\t`{`\n\t# c\n}\n# trailing\nb if \"}\"\n" +
		"c if {\n} else := 2 if {\n}\nelse := 3\n"
	got := statements(bytes.SplitAfter([]byte(src), []byte("\n")))
	exp := []statement{{0, 0, 1}, {3, 5, 9}, {10, 10, 11}, {11, 11, 15}}
	if !slices.Equal(got, exp) {
		t.Fatalf("got %v, want %v", got, exp)
	}
}

func TestReadWithRuleReuse(t *testing.T) {
	popts := ast.ParserOptions{RegoVersion: ast.RegoV1, ProcessAnnotation: true}
	src := "package a\n\np := 1\n\nq := 2\n"
	prevMod, err := ast.ParseModuleWithOpts("/a.rego", src, popts)
	if err != nil {
		t.Fatal(err)
	}
	prev := ast.NewCompiler().WithRuleReuse(nil)
	if prev.Compile(map[string]*ast.Module{"/a.rego": prevMod}); prev.Failed() {
		t.Fatal(prev.Errors)
	}

	b, err := NewReader(archive.MustWriteTarGz([][2]string{{"/a.rego", src + "r := 3\n"}})).
		WithRegoVersion(ast.RegoV1).
		WithRuleReuse(func() *ast.Compiler { return prev }).
		Read()
	if err != nil {
		t.Fatal(err)
	}
	mod := b.Modules[0].Parsed
	var reused int
	for _, r := range mod.Rules {
		if slices.Contains(prev.GetRulesExact(r.Ref()), r) {
			reused++
		}
	}
	if len(mod.Rules) != 3 || reused != 2 {
		t.Fatalf("got %d rules, %d reused", len(mod.Rules), reused)
	}
	if string(b.Modules[0].Raw) != src+"r := 3\n" {
		t.Fatalf("got raw %q", b.Modules[0].Raw)
	}
}
