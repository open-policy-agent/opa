# Parser conformance corpus

A YAML corpus of parser conformance cases: a Rego module in, an AST or
diagnostics out. Every assertion a case makes is committed here, so a case is
complete without a Go counterpart, and a new parser test can land as YAML only.

## Run order

1. **This corpus.** Diagnostics are observable in any pipeline, and `want_ast`
   pins the parse result of an implementation with a separate parser stage.
2. **[`v1/test/compilecases`](../compilecases/README.md)**, once this one passes.
   Compiler cases presuppose that the implementation parses OPA's canonical form
   correctly.
3. **IR**, for an implementation that is not split into parser / compiler /
   planner and so cannot assert an AST. See [`want_ir`](#want_ir) below.

## Adding a case

Write `module` — or `body`, see [Entry points](#entry-points) — plus whatever
configuration it needs, run `make generate`, and review what comes out. Whether
the case is a success or a failure case is decided by the parser, not by you: if
the policy parses, the generator fills in `want_ast`; if it does not, it fills in
`want_errors`.

A fixture is what OPA's parser produces, so it is a golden file: it does not
independently validate OPA, it catches unreviewed change.

The two are maintained differently. `want_ast` is regenerated every time, and
review of the diff is the gate. `want_errors` is filled in **only where a case
has none** and is never overwritten, so a diagnostic that changes fails the
runner instead of being quietly rewritten — the messages are user-facing
contract, and the corpus is the place that says so.

Where a parse produces several diagnostics, a fixture records **the first**: the ones
that follow are often a cascade of the same mistake, and matching is a subset, so every
entry a case records is one an implementation has to report. Holding it to OPA's cascade
is not a language rule.

A case that is about a *later* diagnostic says so, by naming the message and leaving the
rest to the generator:

```yaml
  - note: templatestrings/template-string-error/empty template expression (module)
    module: |
      package test

      p if {
      	$"{}"
      }
    want_errors:
      - message: invalid template-string expression
```

`make generate` fills in that entry's `code` and position by finding the message among
the reported diagnostics — `$"{}"` reports `unexpected } token` first — and fails if no
parse reports it at all. Authoring a position alongside the message turns the entry back
into one the generator never touches.

A case that is about **all** of them sets `exhaustive: true`, and the generator fills in
every diagnostic the parse reported, sorted by position:

```yaml
  - note: terms/non-terminated-array
    module: |
      package test

      p := [1, 2
    exhaustive: true
```

That is the field doing what it says: `exhaustive` asserts the recorded set is the whole
one, so recording only the first would leave the runner rejecting the rest. Where such a
case names some messages itself, the generator completes them and fails if the parse
reports one the case does not name.

**Without `exhaustive`, naming several messages records exactly those** — a subset is a
complete assertion, and the diagnostics the case does not name are neither recorded nor
required. A message the parse reports twice can be named twice, and each copy takes a
position of its own; naming it more often than it is reported, or naming one the parse does
not report at all, fails generation.

Use it where the count is the point — two operands each rejected, two bad imports — and
leave it off where the extra diagnostics are the parser resynchronising after one mistake.

A case is either a **failure case**, asserting `want_errors`, or a **success
case**, asserting `want_ast` and optionally `want_equivalent`. The loader
rejects anything else.

A `module`, `body` or `want_equivalent` must not carry trailing whitespace on a line. A
YAML emitter will not write a block scalar for such a value, so the generator —
which rewrites the whole file whenever a fixture changes — would render the
policy as a single escaped line. The loader rejects it by naming the line rather
than stripping it, so the module stays exactly as authored. Nothing this corpus
can express depends on that whitespace; the cases that do stay in Go, along with
the rest of `Location.Text`.

| field | |
| - | - |
| `note` | identifies the case, and names the subtest; unique within its version directory, not across the corpus — its suffix says which entry point, see [Entry points](#entry-points) |
| `module` | the policy to parse, named `test-0.rego` |
| `body` | a query to parse instead: one or more expressions, exclusive with `module` |
| `imports` | directives in effect for `body`, which has nowhere to declare them; `body` cases only |
| `future_keywords`, `all_future_keywords` | activate future keywords by parser option — a last resort, see [Future keywords](#future-keywords) |
| `experimental_keywords` | opt-in to experimental future keywords, which have no import |
| `annotations` | parse metadata comments into annotations |
| `locations` | include the row and col of every node in `want_ast` |
| `want_ast` | the AST the parse must produce, as JSON; generated, not authored |
| `want_equivalent` | a second module that must parse to the same AST |
| `want_errors` | diagnostics the parse must produce, as `code`/`row`/`col`/`message` |
| `exhaustive` | require `want_errors` to be the complete set, not a subset |
| `entrypoints` | override the entrypoints derived from the module when generating IR, see [Overriding the entrypoints](#overriding-the-entrypoints); `module` cases only |

`want_ast` is JSON carried in a YAML string rather than nested YAML: a YAML
scalar cannot hold a Rego number literal faithfully, since `1e6` is a string to
a YAML parser and a float to a JSON one.

**The members of every object are in lexical order**, which is not the order OPA
emits them in. Key order is an implementation detail no conforming parser should
have to reproduce, and OPA's own is not even stable: the `go1.27` marshallers in
`v1/ast` write a rule as head-then-body, the pre-1.27 ones as body-then-head, so
a fixture recording either would depend on the toolchain it was generated with.
`FormatAST` orders them, and the runner compares through the same function, so
the corpus is the same text on any Go version. Scalars are copied through as the
marshaller wrote them, which is what keeps `1e6` from becoming `1000000`.

It is indented rather than compact, which costs roughly three times the bytes —
about 1.6 KB per case, and 8 KB for one pinning locations. That is deliberate.
Every fixture here is read by a person at least three times: when the diff is
reviewed as the case lands, when a consumer debugs a mismatch against it, and
when a change to the parser regenerates it. A one-line fixture serves none of
those, and the size it saves buys nothing — the corpus is never on a hot path.

## Entry points

A case parses a whole module or a bare body — one or more expressions, which is
what a query is. Which one it is is decided by the field it carries, `module` or
`body`, and stated again in the note:

| note ends with | the case parses | why |
| - | - | - |
| nothing | `module`, exactly as the Go test parsed it, or `body` where that is what the Go test parsed | the input was already a whole module, or already a query |
| ` (module)` | `module`, built around a term or expression | see the wrapper table below |
| ` (body)` | `body`, the same Rego unwrapped | the compiler corpus hands queries to `QueryCompiler`, so this is the baseline those cases stand on |

The module and body forms of one input are siblings, adjacent in the same file:
1,080 of the corpus's cases are a wrapper around a term or an expression, and each
has a `(body)` sibling. Both are needed. A wrapper is chosen so the case
reproduces what its Go test asserted, and the wrapper carries part of the
assertion: of the 237 sibling pairs that assert a diagnostic, 19 report a
different one on each side. `every x in xs` fails with
`unexpected } token: missing body` in a rule and
`unexpected eof token: missing body` on its own, because the brace the parser hit
was the wrapper's. Neither reading is the wrong one, and every fixture on both
sides is generated rather than copied across.

A body case takes its keyword activation from `imports`, since a body has nowhere
to declare one. Only directives are accepted there — `future.keywords.<kw>`,
`future.keywords`, `rego.v1` — because parsing a body resolves no references. It
is read with `SkipRules`, as `rego.New` does for a query: without it a body that
reads as a rule fails with `expected body but got *ast.Rule`, a Go type name with
no position, where the parser has a positioned `rego_parse_error` to report.

**A body case states its activation even where the query parses without it.** This
is the one place the corpus does not import the narrowest set that works, and the
reason is that "works" does not mean "means the same thing". `x and y` parses as
one `and` expression with the keyword active and as three juxtaposed expressions
without it — cleanly, both times, because a body may hold several expressions on a
line where a rule body may not. A case recording the shorter form would assert the
reading it was not about. So `imports` says what the query was parsed with, and a
consumer reads it as the whole activation rather than a hint.

`annotations` and `entrypoints` do not apply to a body: `ParseBody` rejects a
metadata block, and there is no module to plan. `want_ast` for a body is a JSON
array of expressions where a module's is an object.

### Wrapping

Wrap in the innermost context where the Rego is legal:

| the case is about | wrap it as |
| - | - |
| a term | `p := <term>` |
| an expression | `p if { <expr> }` |
| a rule, `package`, or `import` | leave it at the module root |

An expression or a term left at the module root is a trap: `a and (b or c)` and
`{x: y | x := 1}` are both valid Rego, but as a module they fail with
`expression cannot be used for rule head` and
`objectcomprehension cannot be used for rule name`. The parser reads the term or
expression first and only then rejects it as a rule head, so an *error* case
still reproduces its diagnostic that way — which makes the mistake easy to miss.
No *success* case can be written for one, though, so the wrapper is what decides
whether a construct is testable at all.

The wrapper can also quietly change what a case is about. `or (a, b)` at the
module root is a rule head, not an attempted call, and it reports the same
`non-terminated expression` as `foo (a, b)` — so a case asserting that the
`or(x, y)` call form is inactive when the keyword is not, written that way, would
assert nothing of the kind. In a rule body it asserts exactly that, which is
where `testdata/v1/logical/test-call-form.yaml` puts it.

That case is also the shape to copy for anything subtle: the two cases that
*parse* are what give the two that fail their meaning. A case that only records a
diagnostic proves an error happened, not that the right thing was rejected.

Because the wrapper decides what a case can say, it is checked rather than
trusted. A wrapper is accepted only where the term or body it introduces equals
what the input parses to on its own — an expression beginning with `{` parses
inside a rule body, but as a comprehension, because the body's own brace is
absorbed, and silently.

## Future keywords

Activate a future keyword with an import in the module, not with a field on the
case:

```yaml
    module: |
      package test

      import future.keywords.or

      p if {
      	x := or({1}, {2})
      }
```

An import is part of the Rego, so any conforming parser already honours it.
`future_keywords` and `all_future_keywords` are out-of-band parser options, which
a consumer would have to expose before it could run those cases at all — the same
portability cost the module-only entry point exists to avoid. No case in the
corpus needs them today.

Import the narrowest set that works. `import future.keywords` (wildcard) is
allowed and activates everything, but naming the keyword records which one the
case depends on. In v1 only `and`, `or` and `not` still need activating; `if`,
`contains`, `in` and `every` are standard there and need importing only under `v0/`.

`experimental_keywords` has no import form, so it stays a field.

## Layout

```
testdata/<version>/<area>/<file>.yaml
```

**The top-level directory is the Rego version, and it is the only place a case says
which one it is parsed as.** There is no `rego_version` field: a case stating one is
rejected as an unknown field, so the path and the parse cannot disagree. `v0` and `v1`
exist today; `v0-compat-v1` is a legal directory name for when a case needs it.

Below the version comes the language area — `terms/`, `exprs/`, `rules/`, `imports/`,
`packages/`, `logical/`, `templatestrings/`, `annotations/`, `locations/`, `modules/`,
`negation/` — not the outcome, which `want_errors` already records.

A note has to be unique within one version directory, not across the corpus. Cases are
expected to overlap: the same construct parsed as v0 and as v1 is two cases with one note,
and the directory tells them apart. Load the corpus root and the loader stamps each case
with the version it was found under.

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
no IR, and says why on `IRError`; failure cases never yield one, and neither do
body cases — there is no module to compile.

Keeping plans out of the repository is what makes them free: no IR fixtures to
review, no planner change rewriting thousands of committed files, and no size
cost. Nothing here asserts *compiler* behaviour — `want_ir` is a downstream
artifact of a successful parse.

### Cases that produce no plan

Roughly half the success cases do not plan. Where that happens, `WantIR` is nil
and `IRError` says why:

```
terms/var-terms/var (module)  1 error occurred: test-0.rego:3: rego_unsafe_var_error: var foo is unsafe
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

### Filtering by Rego version

`RegoVersionFilter` marks `Ignore` on every case written for a version you do not
parse:

```go
sets, err := cases.LoadParserTestCasesFiltered(
	[]cases.Filters{cases.RegoVersionFilter(ast.RegoV1)},
)
```

Matching is exact. `v0-compat-v1` is its own parsing mode, so supporting `v0` or
`v1` does not imply it, and passing no version filters nothing rather than
rejecting the whole corpus.

Unlike `CapabilitiesFilter`, this one rejects the case *outright* — the module is
written in a dialect you do not parse, so neither its AST nor its diagnostics
apply. Both filters set the same `Ignore` flag, so if you pass both, skipping an
ignored case entirely is the safe reading.

The generator lives in `build/generate-parser-cases`, alongside
`build/generate-extended-cases`, which does the same for the evaluation corpus.
`WithASTLocations`, `WithLocationText` and `Filters` are documented there.
