# generate-parser-cases

Generates the `want_ast` and `want_errors` fixtures of the parser corpus in
`v1/test/parsercases`, and loads the corpus with optional derived forms. Concepts:
[conformance-corpora.md](../conformance-corpora.md).

## Generating

From the repository root:

```
make generate
```

or only the corpus: `go run -tags generate build/generate-parser-cases/gen/main.go v1/test/parsercases/testdata`.
Maintainer workflow: [parsercases README](../../v1/test/parsercases/README.md).

## Loading

```go
import (
	"github.com/open-policy-agent/opa/v1/ast"
	cases "github.com/open-policy-agent/opa/build/generate-parser-cases"
)

sets, err := cases.LoadParserTestCasesFiltered(
	[]cases.Filters{cases.RegoVersionFilter(ast.RegoV1)},
	cases.WithIR(),
)
for _, set := range sets {
	for _, tc := range set.Cases {
		if tc.Ignore {
			continue
		}
		// tc.Module, tc.WantAST, tc.WantErrors, tc.WantIR, tc.EntryPoints
	}
}
```

Options: `WithIR`, `WithASTLocations`, `WithLocationText`. Filters: `RegoVersionFilter`,
`CapabilitiesFilter`.

The loader reads `exceptions.yaml` from the working directory: run from this directory, or
pass `-parser-exceptions <file>`.
