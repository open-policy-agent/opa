package gojsonschema

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestNoStdlibTextTemplateImport guards the linker dead-code-elimination fix
// (#7903): the error formatter uses a method-less template package, never stdlib
// text/template or html/template. Their evalField reaches
// reflect.Value.MethodByName, whose reachability disables method-level DCE for
// the whole binary of every OPA embedder (golang/go#72895). gojsonschema is
// reached unconditionally from ast.Compiler.Compile, so a stdlib import here
// keeps the trigger live for every compiler embedder. Scans every non-test file
// in the package so the guard holds even if the import moves.
func TestNoStdlibTextTemplateImport(t *testing.T) {
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
