package gojsonschema

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestFormatErrorDescription pins the expander semantics. For the only form that
// occurs in this package — a bare {{.key}} over ErrorDetails (map[string]any) —
// behavior is identical to the former text/template path: map index, missing key
// -> "<no value>", values via %v, substituted values not re-expanded.
//
// The trailing cases pin the *intentional* divergences from text/template for
// forms that never appear in locales.go (verified: all locale strings are bare
// {{.identifier}}). They exist only so a future edit that reintroduces such a
// form fails loudly rather than silently changing output: a present-but-nil
// value renders "<nil>" (%v) not "<no value>"; "{{.}}", spaced/trim forms, and
// multi-segment paths are not interpreted.
func TestFormatErrorDescription(t *testing.T) {
	cases := []struct {
		name    string
		format  string
		details ErrorDetails
		want    string
	}{
		{"single placeholder", "{{.field}} is required", ErrorDetails{"field": "name"}, "name is required"},
		{"multiple placeholders", "{{.a}}/{{.b}}", ErrorDetails{"a": "x", "b": "y"}, "x/y"},
		{"adjacent placeholders", "{{.a}}{{.b}}", ErrorDetails{"a": "1", "b": "2"}, "12"},
		{"non-string values via %v", "{{.n}} {{.ok}}", ErrorDetails{"n": 3, "ok": true}, "3 true"},
		{"missing key", "{{.missing}}", ErrorDetails{}, "<no value>"},
		{"one key missing one present", "{{.a}}-{{.b}}", ErrorDetails{"a": "x"}, "x-<no value>"},
		{"no placeholders", "plain text", ErrorDetails{}, "plain text"},
		{"unterminated placeholder", "oops {{.x", ErrorDetails{"x": "v"}, "oops {{.x"},
		{"value is not re-expanded", "{{.a}}", ErrorDetails{"a": "{{.b}}", "b": "nope"}, "{{.b}}"},
		{"substituted value containing braces", "{{.a}}", ErrorDetails{"a": "z}}z"}, "z}}z"},
		{"empty format", "", ErrorDetails{}, ""},
		// Intentional divergences from text/template (never reached by locales.go):
		{"present-but-nil renders <nil>", "{{.x}}", ErrorDetails{"x": nil}, "<nil>"},
		{"bare dot not interpreted", "{{.}}", ErrorDetails{"a": 1}, "<no value>"},
		{"spaced form not interpreted", "{{ .x }}", ErrorDetails{"x": "v"}, "{{ .x }}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatErrorDescription(tc.format, tc.details); got != tc.want {
				t.Errorf("formatErrorDescription(%q, %v) = %q, want %q", tc.format, tc.details, got, tc.want)
			}
		})
	}
}

// TestNoTemplateImport guards the linker dead-code-elimination fix: no non-test
// source file in this package may import text/template or html/template. Their
// template evaluators reach reflect.Value.MethodByName, which disables
// method-level DCE for the whole binary of every OPA embedder (golang/go#72895,
// #7903). Reintroducing the import silently regresses that.
func TestNoTemplateImport(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		scanned++
		for _, imp := range f.Imports {
			switch strings.Trim(imp.Path.Value, `"`) {
			case "text/template", "html/template":
				t.Errorf("%s imports %s — reintroduces the linker DCE-defeat trigger", name, imp.Path.Value)
			}
		}
	}
	if scanned == 0 {
		t.Fatal("scanned no package files; test ran from the wrong directory")
	}
}
