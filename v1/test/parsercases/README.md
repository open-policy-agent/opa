# Parser conformance corpus

A YAML corpus of parser conformance cases: a Rego module in, an AST or
diagnostics out. Every assertion a case makes is committed here, so a case is
complete without a Go counterpart, and a new parser test can land as YAML only.

## Run order

1. **This corpus.** Diagnostics are observable in any pipeline, and `want_ast`
   pins the parse result of an implementation with a separate parser stage.
2. **`v1/test/compilecases`**, once this one passes. Compiler cases presuppose
   that the implementation parses OPA's canonical form correctly.
3. **IR**, for an implementation that is not split into parser / compiler /
   planner and so cannot assert an AST. See [`want_ir`](#want_ir) below.

## Adding a case

Write `module` plus whatever configuration it needs, run `make generate`, and
review the `want_ast` that comes out. A fixture is what OPA's parser produces,
so it is a golden file: it does not independently validate OPA, it catches
unreviewed change.

A case is either a **failure case**, asserting `want_errors`, or a **success
case**, asserting `want_ast` and optionally `want_equivalent`. The loader
rejects anything else.

| field | |
| - | - |
| `note` | globally unique identifier, and the subtest name |
| `module` | the policy to parse, named `test-0.rego` |
| `rego_version` | `v0`, `v1` (default), or `v0-compat-v1` |
| `future_keywords`, `all_future_keywords` | keywords available without importing them |
| `experimental_keywords` | opt-in to experimental future keywords |
| `annotations` | parse metadata comments into annotations |
| `locations` | include the row and col of every node in `want_ast` |
| `want_ast` | the AST the parse must produce, as JSON; generated, not authored |
| `want_equivalent` | a second module that must parse to the same AST |
| `want_errors` | diagnostics the parse must produce, as `code`/`row`/`col`/`message` |
| `exhaustive` | require `want_errors` to be the complete set, not a subset |
| `entrypoints` | override the entrypoints derived from the module when generating IR, see [Overriding the entrypoints](#overriding-the-entrypoints) |

`want_ast` is JSON carried in a YAML string rather than nested YAML: a YAML
scalar cannot hold a Rego number literal faithfully, since `1e6` is a string to
a YAML parser and a float to a JSON one.

## Locations

Off by default, opt in per case with `locations: true`. Positions are already
pinned by the diagnostics in this corpus and in `compilecases`, by the eval
corpus `want_error` strings, and by the IR plan. Putting them in every
`want_ast` would add little on top of that while forcing positions onto AST
nodes — an implementation using a side-table keyed by node identity would emit
byte-identical errors and IR and still fail.

`locations` and `want_equivalent` are mutually exclusive: the two modules are
structurally identical but positionally different, so they cannot share a
fixture.

Source text spans are never in a fixture. `Location.Text` is a byte offset into
source and the least portable thing OPA exposes; the cases that assert it stay
in Go, along with `TestMaxParsingRecursionDepth`, which is an implementation
limit rather than a language rule.

## `want_ir`

A generator option, not a committed field:

```go
sets, err := cases.LoadParserTestCases(cases.WithIR())
```

`WithIR` runs the full downstream pipeline for every success case — parse,
compile to completion, plan — and populates `WantIR` where it succeeds, along
with the entrypoints it planned for. A case that fails to compile or plan yields
no IR, and says why on `IRError`; failure cases never yield one.

Keeping plans out of the repository is what makes them free: no IR fixtures to
review, no planner change rewriting thousands of committed files, and no size
cost. Nothing here asserts *compiler* behaviour — `want_ir` is a downstream
artifact of a successful parse.

### Cases that produce no plan

Roughly half the success cases do not plan. Where that happens, `WantIR` is nil
and `IRError` says why:

```
terms/var  1 error occurred: test-0.rego:3: rego_unsafe_var_error: var foo is unsafe
```

`IRError` is diagnostic, not an assertion, and the example above is why: that
case exists to pin how the token `foo` parses as a var term, and its module is
`p := foo` because that is the smallest wrapper that puts a bare term in a
module. The unsafe-var diagnostic is an artifact of the wrapper, not something
the case set out to say. Most of these failures are of that kind, so harvesting
them into expected-error fixtures would fill the corpus with assertions nobody
authored and no implementation should be held to.

Compiler diagnostics that *are* worth asserting belong in
`v1/test/compilecases`, where they are authored and reviewed one at a time.

What `IRError` is good for: telling "OPA cannot plan this either" apart from
"generation broke", and spotting cases where a wrapper chosen for AST fidelity
is costing IR coverage that could be recovered by rewording the module.

### Example

Print every case that plans, with the entrypoints it was planned for:

```go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	cases "github.com/open-policy-agent/opa/build/generate-parser-cases"
)

func main() {
	sets, err := cases.LoadParserTestCases(cases.WithIR())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	for _, set := range sets {
		for _, tc := range set.Cases {
			if tc.WantIR == nil {
				// The module did not compile, or did not plan; tc.IRError says which.
				continue
			}

			plan, err := json.MarshalIndent(tc.WantIR, "", "  ")
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}

			fmt.Printf("%s %v\n%s\n", tc.Note, tc.EntryPoints, plan)
		}
	}
}
```

```console
$ go run ./main.go
rules/partial-set [test/p]
{
  "static": {
    "strings": [
...
```

`LoadParserTestCases` reads `exceptions.yaml` from the working directory, so run
it from `build/generate-parser-cases` or point `-parser-exceptions` at your own
file. `EntryPoints` is populated on every case that plans, whether the case
authored an override or the set was derived from its module, so a consumer never
has to derive it — there is one plan per entry, in the same order.

### Overriding the entrypoints

By default a case is planned for every ground rule ref in its module. Author
`entrypoints` where that set is wrong for the case — where the interesting plan
needs a particular shape, and planning the whole package would bury it.
`testdata/v1/rules/test-entrypoints.yaml` is the worked example:

```yaml
- note: rules/authored-entrypoints
  module: |
    package test

    allow if {
    	helper
    }

    helper if {
    	input.x == 1
    }
  entrypoints:
    - test/helper
```

Each entry is a slash-separated document path under `data`, so `test/helper`
plans `data.test.helper`. Without the override this case would be planned for
`[test/allow, test/helper]`; with it, `WithIR` produces the single plan
`test/helper`.

To drop the cases whose plans reference operators you do not implement, pair
`WithIR` with `CapabilitiesFilter`. A rejected case is marked `Ignore` and has
its plan dropped, but is not removed: the corpus stays addressable by index, and
the case's `want_ast` is still yours to run — it is the plan you cannot execute,
not the parse.

```go
capabilities, err := ast.LoadCapabilitiesFile("capabilities.json")
if err != nil {
	// ...
}

sets, err := cases.LoadParserTestCasesFiltered(
	[]cases.Filters{cases.CapabilitiesFilter(capabilities)},
	cases.WithIR(),
)
```

The generator lives in `build/generate-parser-cases`, alongside
`build/generate-extended-cases`, which does the same for the evaluation corpus.
`WithASTLocations`, `WithLocationText` and `Filters` are documented there.
