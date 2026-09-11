# Compiler conformance corpus

A YAML corpus of compiler conformance cases: Rego modules in, diagnostics out.
Every assertion a case makes is committed here, so a case is complete without a
Go counterpart, and a new compiler test can land as YAML only.

## Run order

1. **[`v1/test/parsercases`](../parsercases/README.md) first.** A compiler case says
   nothing about how Rego parses; run against a disagreeing parser it reports parse
   failures as compiler failures.
2. **This corpus.** Diagnostics are observable in any pipeline, whether or not the
   implementation has a separable compiler stage.

## Adding a case

Write `note` and `modules` plus whatever configuration they need, run `make
generate`, and review what comes out:

```yaml
---
cases:
  - note: safety/unsafe var in rule body
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    exhaustive: true
```

A fixture is what OPA's compiler produces, so it is a golden file: it catches
unreviewed change, it does not independently validate OPA.

`want_errors` is filled in **only where a case has none** and never overwritten, so a
changed diagnostic fails the runner instead of being rewritten under it. To re-seed
one after a deliberate change, delete it and regenerate.

A case has to assert something: either diagnostics, or what compiling its modules
produces. An absent `want_errors` is itself an assertion that nothing was reported.

A module must not carry trailing whitespace: a YAML emitter will not write a block
scalar for such a value, so the generator would render the policy as one escaped
line. The loader names the offending line rather than stripping it.

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
| `want` | what compiling produces, one entry per module — see [Transformations](#transformations) |
| `want_stages` | what the modules look like partway through, keyed by compiler stage — see [Stages](#stages) |

Activate a future keyword with an `import` in the module rather than a field on
the case, for the reason the [parser corpus](../parsercases/README.md#future-keywords)
gives: an import is part of the Rego, so any conforming parser already honours
it. `experimental_keywords` has no import form, so it stays a field.

## Transformations

`want` says what compiling the case's modules produces, one entry per entry in
`modules` and in the same order:

```yaml
    modules:
      - |
        package test

        import data.other.thing

        p if {
        	thing == 1
        }
    want:
      - module: |
          package test

          p = true if {
          	data.other.thing = 1
          }
```

The import is resolved and dropped, the implied rule value is made explicit, and
`==` becomes unification — three things the compiler does that an implementation
has to do too.

**The comparison is between ASTs.** `module` is parsed under the case's
`rego_version` and compared to the compiled module; the Rego is only how the
expectation is written down, so a printer that differs from OPA's costs nothing.

An absent `want_errors` also asserts that nothing was reported.

**Generated, not authored**, and regenerated every time — unlike `want_errors`,
`want` is the compiled form, so review of the diff is the gate. Not by `v1/format`,
which normalises as it prints and so produces a different AST than it was given; the
generator breaks the bodies of OPA's own one-line rendering instead.

**Generated variable names are part of the assertion**, since the comparison is an
AST:

```yaml
    want:
      - module: |
          package test

          p = true if {
          	__local0__ = data.test.q
          	[__local0__]
          }
```

That holds an implementation to OPA's naming, which is more than the corpus asks
anywhere else, and there is no way around it: a transformation that introduces a
variable has to name it.

### `imports`: directives the compiler resolved away

The compiler drops a directive import once it has taken effect, so the compiled form
depends on it with nothing left to say so. The entry records what was there:

```yaml
    want:
      - imports: [future.keywords.or]
        module: |
          package test

          p = true {
          	{ __local0__ = 1 } or { __local1__ = 2 }
          }
```

| entry | effect on parsing `module` |
| - | - |
| `future.keywords.<kw>` | activate that keyword |
| `future.keywords` | activate every future keyword the version has |
| `rego.v1` | parse as `v1`, whatever the case's `rego_version` says |

Writing the import back into `module` is not an option: the result would carry an
import the compiled module does not, and so parse to a different AST.

**It sits on the entry, not the case**, because directives do not carry across
modules — one module may import `future.keywords.not` while its neighbour uses `not`
as ordinary negation. `TestCompilerNotImport` alone has 41 such cases.

**Every directive the module imports is listed**, needed or not: working out which are
redundant would mean modelling what the compiler does to each, and under-declaring
leaves a fixture nobody can parse. An import the schema cannot interpret fails
generation rather than being skipped.

If your parser cannot be told any of this, or your compiler keeps its imports, the
loader will put them back:

```go
sets, err := cases.LoadCompilerTestCases(cases.WithDirectiveImports())
```

The result then has imports OPA's compiled module does not, so it no longer parses to
the AST OPA produces — use it when your pipeline keeps its imports, not to compare
against OPA.

### `ast`: where the compiled form has no Rego spelling

`module` is seeded by printing the compiled AST, and the generator parses it back and
compares before committing it. OPA's printer does not always survive that — `else :=`
is emitted as `else =` — and a fixture that did not round-trip would assert something
the compiler never produced.

Where it does not survive, the entry carries **`ast`** instead: the same assertion,
marshalled. Chosen per module, so one module needing it does not drag its neighbours
in, and annotated with what disqualified it:

```yaml
    want:
      # ast rather than module: the compiled rule `p := 1 if { input.x } else = 2
      # if { true }` prints as Rego that parses to a different AST.
      - ast: |
          {
```

## Stages

`want_stages` records what the modules look like when the pipeline stops after a
named stage. It is keyed by stage name, and each value has the same shape as `want`
— one entry per module, `module` or `ast`:

```yaml
    want:
      - module: |
          package test

          p = true if {
          	__local0__ = data.test.q
          	[__local0__]
          }
    want_stages:
      RewriteEquals:
        - module: |
            package test

            p = true if {
            	[data.test.q]
            }
```

**It is additive, and never a conformance requirement.** `want` and `want_errors`
always describe the whole pipeline, so an implementation that is not split into
OPA's stages ignores `want_stages` and loses no coverage. One that is can assert the
tighter intermediate form. Requiring it would impose OPA's internal architecture on
every implementation, which is the thing this corpus exists to avoid — and OPA's own
`StageID` identifiers are explicitly not stable across versions.

A stage is carried **only where its form differs from the full-pipeline one**. An
intermediate assertion equal to the endpoint asserts nothing the endpoint does not,
so the generator drops it; what survives is the set of stages that do something the
endpoint hides. Name a stage with an empty value and run `make generate` to fill it
in — or to have it removed.

The legal names are `compilecases.Stages`, a copy of `ast.AllStages()` — the schema
package does not import `v1/ast`, since the runner is `package ast` and that would
cycle. `TestCorpusStagesMatchCompiler` keeps the two agreeing.

What catches a renamed or reordered stage is the fixtures, not that test. A stage
whose *behaviour* or *position* changes produces a different `want_stages` value, and
`TestGeneratedFixturesDoNotDrift` byte-compares the regenerated corpus. A stage that
is *renamed* leaves a key neither the loader nor the generator recognises — and both
check the name against `ast.AllStages()` before compiling, because
`WithOnlyStagesUpTo` runs the whole pipeline on a name it does not have, which would
otherwise record the endpoint's form and let the difference gate drop the assertion
as redundant.

A case whose diagnostics are raised *before* the stage it names fails generation
rather than recording anything: the field asserts a form, and there is only one of
those if the pipeline got that far cleanly.

## Several modules

A case compiles all of its `modules` together — how conflicts, cross-package
recursion and cross-module ref resolution are expressed. They are named positionally,
and a diagnostic names the module it is reported against:

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

Diagnostics are compared **as a set**: report order is not part of the contract. Each
expected one must pair with a distinct reported one on module, code, row and message.

`col` is asserted only when present. Deleting it relaxes a case to the row, worth
doing where the column is an artifact of OPA's desugaring.

`exhaustive: true` additionally requires that nothing else was reported. Leave it off
where the number of diagnostics is not itself the contract —
`testdata/v1/safety/test-safety-unsafe-var-cascade.yaml` is the worked case.

The error limit is lifted for both generation and running; at
`CompileErrorLimitDefault` the compiler stops after ten and appends its own "too many
errors".

## Layout

```
testdata/v0/<area>/<file>.yaml
testdata/v1/<area>/<file>.yaml
```

The top level is the Rego version, below it the language area — `builtins/`,
`functions/`, `imports/`, `keywords/`, `print/`, `safety/`, `templatestrings/`,
`vars/` — not the outcome, which `want_errors` already records. Grouping by area puts
a rule and its counter-example side by side.

The directory is organisational; `rego_version` drives parsing, and the two should
agree. Where the same source means different things in v0 and v1, write one case per
version rather than translating between them.

`testdata/testdata.go` embeds the corpus so that tools outside this repository
can consume it, as `v1/test/cases/testdata` does for the evaluation cases.

## Strict mode

Strict mode is a boolean in OPA's compiler, but whether a *consumer* can switch it is
not: an implementation may have none, or may apply the checks unconditionally. So a
case says whether its expectations depend on the setting rather than carrying it.

| `strict` | compile with | means |
| - | - | - |
| absent | either; OPA's runner uses off | the case asserts the same thing both ways |
| `enabled` | strict on | the expectations depend on strict being on |
| `disabled` | strict off | the expectations depend on strict being off |

Both directions are enforced rather than trusted: `TestStrictIsImmaterialWhereUnset`
compiles every unset case both ways and fails if the diagnostics differ, and
`TestStrictMattersWhereSet` fails if a case names a setting it does not need.

`disabled` is not hypothetical: `p(x) foobar if { x == 2 }` reports
`var x is unsafe in rule foobar` with strict off and `unused argument x` with it on.

An implementation whose strict mode is switchable runs the whole corpus. One whose is
not filters, keeping the unset cases either way:

```go
sets, err := cases.LoadCompilerTestCasesFiltered(
	[]cases.Filters{cases.StrictModeFilter(compilecases.StrictDisabled)}, // no strict mode
)
```

## Consuming the corpus from Go

The embedded YAML is the corpus, so a consumer that wants only the committed cases can
read `testdata.FS` directly. `build/generate-compiler-cases` adds a loader for
filtering:

```go
sets, err := cases.LoadCompilerTestCasesFiltered(
	[]cases.Filters{cases.RegoVersionFilter(ast.RegoV1)},
)
```

`RegoVersionFilter` marks `Ignore` on every case written for a version you do not
parse. Matching is exact — `v0-compat-v1` is its own mode — and passing no version
filters nothing. `StrictModeFilter` does the same for the strict setting a case pins.
A rejected case is marked, never removed, so the corpus stays addressable by index.

`CapabilitiesFilter` has no counterpart here yet: it filters on the builtins a plan
calls, and this corpus does not generate plans.

## What this corpus does not carry yet

Query compilation. Those cases still live in `v1/ast/compile_test.go`.

The generator lives in `build/generate-compiler-cases`, alongside
`build/generate-parser-cases` and `build/generate-extended-cases`, which do the
same for the parser and evaluation corpora.
