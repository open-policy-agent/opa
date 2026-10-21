# Using the Rego conformance corpora

Two YAML corpora let the maintainers of another Rego parser or compiler check it against OPA:

| | parser | compiler |
| - | - | - |
| cases | `v1/test/parsercases/testdata` | `v1/test/compilecases/testdata` |
| input | a `module`, or a `body` (a query) | `modules`, optionally a `query` |
| asserts | `want_ast` or `want_errors` | `want` (compiled modules) or `want_errors` |
| tooling | [`generate-parser-cases`](generate-parser-cases/README.md) | [`generate-compiler-cases`](generate-compiler-cases/README.md) |

Run the parser corpus first. Compiler cases write their expectations as Rego and presuppose
that your parser reads OPA's canonical form correctly.

## Getting the cases

The YAML is the corpus, and is usable as is. Copy it, or embed it from Go with
`testdata.FS` in either `testdata` package. Fields are listed in the `TestCase` types in
`v1/test/parsercases/cases.go` and `v1/test/compilecases/cases.go`; unknown fields are rejected.

Most implementations also need filtering and derived forms (plans, ASTs, locations) that
OPA computes for you. The Go loaders in the tool packages provide those; see their READMEs.

## Concepts

**Version.** The top-level directory (`v0`, `v1`, `v0-compat-v1`) is the Rego version a case
is parsed as. No case states one. A `note` names the case and is unique within a version
directory, not across the corpus.

**Fixtures are OPA's output.** They were produced by OPA and reviewed, not independently
derived. A mismatch is a question to ask, not necessarily your bug.

**Diagnostics** (`want_errors`: `code`, `row`, `col`, `message`) are matched as a set,
without regard to order. Every expected entry must be reported; extra reported ones are fine
unless the case is `exhaustive`. Only the first diagnostic of a parse is recorded unless the
case says otherwise. Compiler cases may also carry `detail`. `col` and `detail` are asserted
only when present, so you can delete them to relax a case. Nothing obliges you to word
messages as OPA does; relax the cases where you differ.

**Parser cases.**
- `want_ast` is JSON in a YAML string, object keys in lexical order, number literals as
  written (`1e6` stays `1e6`). Compare structurally, not as text.
- `want_equivalent` is a second module that must parse to the same AST.
- A `body` case parses a query, not a module. Wrapper cases come in pairs, a ` (module)` and
  a ` (body)` sibling, and may report different diagnostics.
- A body has nowhere to declare keywords, so `imports` lists the directives in effect
  (`future.keywords.<kw>`, `future.keywords`, `rego.v1`). Read it as the whole activation.
- Positions are only in cases with `locations: true`.

**Future keywords** are activated by an `import` in the module, so a conforming parser
needs no extra option. In v1 only `and`, `or` and `not` still need one.

**Compiler cases.**
- All `modules` compile together, named `test-0.rego`, `test-1.rego`, … Diagnostics name the
  module; `test-0.rego` is the default.
- `want[i].module` is Rego, but the comparison is between ASTs: it is parsed under the
  case's version and compared to your compiled module. How you print is irrelevant. Where the
  compiled form has no Rego spelling, the entry carries `ast` instead.
- Compiler-generated names (`__local0__`) are part of the assertion.
- A directive import the compiler resolves away is recorded in `want[i].imports`, so you can
  parse `module` with the right keywords.
- A `query` case asserts what the query compiles to. `query.imports` lists directives in
  effect for the body as well as refs in scope.
- `want_stages` is additive: intermediate forms keyed by OPA compiler stage. Ignore it if you
  have no such stages.

**Optional behaviour is filterable, not required.**
- `strict`: absent means the case holds either way; `enabled` or `disabled` pins a setting.
- `schemas`: attaching a schema asks for schema-driven type checking.
- `query`, `strict` and `schemas` each have a filter for implementations without them.
- Capabilities and unsafe built-in sets are deliberately not in the corpus.

## Derived forms

These are computed by the loaders, never committed:

| option | adds | for an implementation that |
| - | - | - |
| `WithIR()` | `WantIR`, `EntryPoints`, `IRError` | is Rego → IR only, with no separable AST |
| `WithAST()` (compiler) | `ast` on every compiled entry | has an AST but no printer for OPA's Rego |
| `WithASTLocations()` (parser) | row/col on every `want_ast` node | wants to compare positions |
| `WithLocationText()` (parser) | also source spans | as above, and can match byte offsets |
| `WithDirectiveImports()` (compiler) | puts resolved imports back in `module` | keeps imports through compilation |

`WantIR` is nil where the case does not compile or plan, and `IRError` says why; that is
diagnostic, not an assertion. Plans conform to `v1/ir/plan.schema.json`.

Filters mark a case `Ignore` rather than removing it, so indexes stay stable. Skip ignored
cases entirely: `RegoVersionFilter`, `CapabilitiesFilter` (drops plans using built-ins you
lack), and for the compiler `StrictModeFilter`, `SchemasFilter`, `QueryFilter`.

## Not in the corpus

Cases that depend on OPA internals stay in Go: source text spans (except via
`WithLocationText`), parser recursion limits, capability-gated behaviour, and unsafe
built-in restrictions.
