# Compiler conformance corpus

A YAML corpus of compiler conformance cases: Rego modules in, diagnostics out.
Every assertion a case makes is committed here, so a case is complete without a
Go counterpart, and a new compiler test can land as YAML only.

## Run order

1. **[`v1/test/parsercases`](../parsercases/README.md) first.** A compiler case
   is written in Rego and says nothing about how Rego parses; running it against
   an implementation whose parser disagrees reports compiler failures that are
   really parse failures.
2. **This corpus.** Diagnostics are observable in any pipeline, whether or not
   the implementation has a separable compiler stage.

## Adding a case

Write `note` and `modules` plus whatever configuration they need, run `make
generate`, and review what comes out:

```yaml
---
cases:
  - note: safety/unsafe-var-in-rule-body
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    exhaustive: true
```

A fixture is what OPA's compiler produces, so it is a golden file: it does not
independently validate OPA, it catches unreviewed change.

`want_errors` is filled in **only where a case has none** and is never
overwritten, so a diagnostic that changes fails the runner instead of being
quietly rewritten — the messages are user-facing contract, and the corpus is the
place that says so. To re-seed a case after a deliberate change, delete its
`want_errors` and regenerate.

A case has to assert something, so one that neither reports diagnostics nor states
what compiling its modules produces is rejected.

A module must not carry trailing whitespace on a line. A YAML emitter will not
write a block scalar for such a value, so the generator — which rewrites the
whole file whenever a fixture changes — would render the policy as a single
escaped line. The loader rejects it by naming the line rather than stripping it,
so the module stays exactly as authored.

| field | |
| - | - |
| `note` | globally unique identifier, and the subtest name |
| `modules` | the policies to compile, named `test-0.rego`, `test-1.rego`, … |
| `rego_version` | `v0`, `v1` (default), or `v0-compat-v1` |
| `strict` | `enabled`, `disabled`, or absent — see [Strict mode](#strict-mode) |
| `experimental_keywords` | opt-in to experimental future keywords, which have no import |
| `print_statements` | keep `print()` calls instead of erasing them, as required to reach diagnostics about their operands |
| `want_errors` | diagnostics the compilation must produce, as `module`/`code`/`row`/`col`/`message` |
| `exhaustive` | require `want_errors` to be the complete set, not a subset |
| `want_modules` | the Rego each module compiles to, one per module — see [Transformations](#transformations) |

Activate a future keyword with an `import` in the module rather than a field on
the case, for the reason the [parser corpus](../parsercases/README.md#future-keywords)
gives: an import is part of the Rego, so any conforming parser already honours
it. `experimental_keywords` has no import form, so it stays a field.

## Transformations

`want_modules` says what compiling the case's modules produces, one entry per
entry in `modules` and in the same order:

```yaml
    modules:
      - |
        package test

        import data.other.thing

        p if {
        	thing == 1
        }
    want_modules:
      - |
        package test

        p = true if { data.other.thing = 1 }
```

The import is resolved and dropped, the implied rule value is made explicit, and
`==` becomes unification — three things the compiler does that an implementation
has to do too.

**The comparison is between ASTs, not between text.** `want_modules` is parsed
under the case's `rego_version` and compared to the compiled module; the Rego is
only how the expectation is written down. An implementation that never prints Rego
can compare however it likes, and one whose printer differs from OPA's is not
penalised for it.

A case with `want_modules` and no `want_errors` also asserts that nothing was
reported: an absent `want_errors` means no diagnostics.

**Generated, not authored.** Write `modules` plus the configuration, run
`make generate`, and review what comes out. Unlike `want_errors`, `want_modules`
is regenerated every time — it is the compiled form, so review of the diff is the
gate, and CI fails if someone changes the compiler without regenerating.

The layout is one expression per line, which is how the Rego was written before
the compiler got to it. That is not `v1/format` — the formatter normalises as it
prints, so its output parses to a *different* AST than the one it was given, and
only two thirds of modules survive it. The generator instead breaks the bodies of
OPA's own one-line rendering, so every token still comes from OPA's printer and
nothing about v0 or v1 syntax is reimplemented.

The seed comes from printing the compiled AST, and the generator checks the
round-trip rather than assuming it: the text is parsed back and compared to the AST
it came from, and the case is rejected if they differ. OPA's printer does not
always survive that — `else :=` is emitted as `else =`, for one — and a fixture
that did not round-trip would assert something the compiler never produced.

**Generated variable names are part of the assertion.** Hoisting and local
rewriting introduce variables, and the AST comparison includes their names:

```yaml
    want_modules:
      - |
        package test

        p = true if { __local0__ = data.test.q; [__local0__] }
```

That does hold an implementation to OPA's naming, which is more than the corpus
asks anywhere else. There is no way around it while the assertion is an AST: a
transformation that introduces a variable has to name it. Worth knowing before
running these cases rather than discovering it in a diff.

## Several modules

A case compiles all of its `modules` together, which is how conflicts, recursion
across packages, and cross-module ref resolution are expressed. They are named
positionally, and a diagnostic names the module it is reported against:

```yaml
    modules:
      - |
        package a

        p if {
        	x == 2
        }
      - |
        package b

        q if {
        	y == 3
        }
    want_errors:
      - code: rego_unsafe_var_error
        row: 4
        col: 2
        message: var x is unsafe
      - module: test-1.rego
        code: rego_unsafe_var_error
        row: 4
        col: 2
        message: var y is unsafe
```

`module` is omitted for `test-0.rego`, so a single-module case carries no
attribution it does not need.

## How diagnostics are matched

Diagnostics are compared **as a set**: the order an implementation reports them
in is not part of the contract. Each expected diagnostic must pair with a
distinct reported one on module, code, row and message.

`col` is asserted only when present. The generator writes one wherever the
compiler reports a position, but deleting it from a case relaxes that case to the
row — worth doing where a column is an artifact of OPA's own desugaring rather
than something an implementation should be held to.

`exhaustive: true` additionally requires that nothing else was reported. Leave it
off where the number of diagnostics is not itself the contract — the cascade of
one mistake is the usual example, and
`testdata/v1/safety/test-safety-unsafe-var-cascade.yaml` is the worked case: the
root cause must be reported, the rest may be.

The corpus is generated and run with the compiler's error limit lifted. At the
default of `CompileErrorLimitDefault` the compiler stops after ten diagnostics
and appends a "too many errors" one of its own, which would silently truncate any
case that reports more.

## Layout

```
testdata/v0/<area>/<file>.yaml
testdata/v1/<area>/<file>.yaml
```

The top level is the Rego version and below it is the language area — `builtins/`,
`functions/`, `imports/`, `keywords/`, `print/`, `safety/`, `templatestrings/`,
`vars/` — not the outcome. A directory that meant "these fail" would say nothing
that `want_errors` does not, while costing the grouping that puts a rule and its
counter-example side by side.

The directory is organisational; `rego_version` on the case is what drives
parsing, and the two are expected to agree. A case is filed under the version it
is *about*: where the same source means different things in v0 and v1, write one
case per version rather than translating between them.

`testdata/testdata.go` embeds the corpus so that tools outside this repository
can consume it, as `v1/test/cases/testdata` does for the evaluation cases.

## Strict mode

Strict mode is a boolean in OPA's compiler, but whether a *consumer* can switch it
is not: an implementation may have no strict mode at all, or may apply the checks
unconditionally. So a case does not carry the setting as a boolean — it says
whether its expectations depend on it.

| `strict` | compile with | means |
| - | - | - |
| absent | either; OPA's runner uses off | the case asserts the same thing both ways |
| `enabled` | strict on | the expectations depend on strict being on |
| `disabled` | strict off | the expectations depend on strict being off |

Both halves of that are load-bearing, and both are enforced rather than trusted.
`TestStrictIsImmaterialWhereUnset` compiles every unset case both ways and fails
if the diagnostics differ; `TestStrictMattersWhereSet` fails if a case names a
setting it does not need, since that excludes consumers for nothing. Every
diagnostic is compared, not only those the case asserts — a check strict mode adds
under a different code still makes the setting matter.

`disabled` is not hypothetical. `p(x) foobar if { x == 2 }` reports
`var x is unsafe in rule foobar` with strict off and `unused argument x` with it
on: two diagnostics about different things, and only the first is what the case is
about.

An implementation whose strict mode is switchable runs the whole corpus. One whose
is not filters, and the absent cases are the ones it keeps either way:

```go
sets, err := cases.LoadCompilerTestCasesFiltered(
	[]cases.Filters{cases.StrictModeFilter(compilecases.StrictDisabled)}, // no strict mode
)
```

## Consuming the corpus from Go

The embedded YAML is the corpus, so a consumer that wants only the committed
cases can read `testdata.FS` directly. `build/generate-compiler-cases` offers a
loader on top of it for consumers that want to filter:

```go
sets, err := cases.LoadCompilerTestCasesFiltered(
	[]cases.Filters{cases.RegoVersionFilter(ast.RegoV1)},
)
```

`RegoVersionFilter` marks `Ignore` on every case written for a version you do not
parse. Matching is exact: `v0-compat-v1` is its own parsing mode, so supporting
`v0` or `v1` does not imply it, and passing no version filters nothing rather
than rejecting the whole corpus. `StrictModeFilter` does the same for the strict
setting a case pins — see [Strict mode](#strict-mode). A rejected case is marked,
never removed: the corpus stays addressable by index, and what you do with an
ignored case is your own business.

`CapabilitiesFilter`, which the parser corpus pairs with `WithIR`, has no
counterpart here yet: it filters on the builtins a plan calls, and this corpus
does not generate plans.

## What this corpus does not carry yet

Diagnostics only, today. Compiler *transformations* — asserting that compiling a
module yields a particular compiled form — and query compilation are not
expressible here yet; those tests still live in `v1/ast/compile_test.go`.

The generator lives in `build/generate-compiler-cases`, alongside
`build/generate-parser-cases` and `build/generate-extended-cases`, which do the
same for the parser and evaluation corpora.
