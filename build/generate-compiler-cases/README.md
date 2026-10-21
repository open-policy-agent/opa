# generate-compiler-cases

Generates the `want`, `want_stages` and `want_errors` fixtures of the compiler corpus in
`v1/test/compilecases`, and loads the corpus with optional derived forms. Concepts:
[conformance-corpora.md](../conformance-corpora.md).

## Generating

From the repository root:

```
make generate
```

or only the corpus: `go run -tags generate build/generate-compiler-cases/gen/main.go v1/test/compilecases/testdata`.
Maintainer workflow: [compilecases README](../../v1/test/compilecases/README.md).

## Loading

```go
import (
	"github.com/open-policy-agent/opa/v1/ast"
	cases "github.com/open-policy-agent/opa/build/generate-compiler-cases"
	"github.com/open-policy-agent/opa/v1/test/compilecases"
)

sets, err := cases.LoadCompilerTestCasesFiltered(
	[]cases.Filters{
		cases.RegoVersionFilter(ast.RegoV1),
		cases.StrictModeFilter(compilecases.StrictDisabled), // the strict modes you support
		cases.SchemasFilter(),
		cases.QueryFilter(),
	},
	cases.WithAST(),
)
for _, set := range sets {
	for _, tc := range set.Cases {
		if tc.Ignore {
			continue
		}
		// tc.Modules, tc.Want, tc.WantErrors, tc.WantIR, tc.EntryPoints
	}
}
```

Options: `WithAST`, `WithIR`, `WithDirectiveImports`. Filters: `RegoVersionFilter`,
`CapabilitiesFilter`, `StrictModeFilter`, `SchemasFilter`, `QueryFilter`. Drop the filters
for features you support.
