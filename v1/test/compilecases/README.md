# Compiler conformance corpus

YAML cases for the Rego compiler: modules (and optionally a query) in, the compiled modules or
diagnostics out. This package holds the schema and the loader; the cases are in
`testdata/<version>/<area>/*.yaml`.

Using the corpus from another compiler: [`build/conformance-corpora.md`](../../../build/conformance-corpora.md).
The [parser corpus](../parsercases/README.md) is a precondition.

## Adding a case

Add it to the file for its area under `testdata/v1/` (`testdata/v0/` for v0-only behaviour),
with a `note` unique within the version directory and one or more `modules`:

```yaml
  - note: safety/unsafe var in rule body
    modules:
      - |
        package test

        p if {
        	x == 2
        }
    exhaustive: true
```

Then run `make generate` and review the diff.

- `want` (the compiled modules) and `want_stages` are regenerated on every run; reviewing the
  diff is the gate. Name a stage with an empty value to have it filled in.
- `want_errors` is filled in only where a case has none. To re-seed one after a deliberate
  change, delete it and regenerate. An absent `want_errors` asserts nothing was reported.
- `exhaustive: true` requires `want_errors` to be the complete set.
- Set `strict: enabled|disabled` only where the result depends on it; this is checked.
- Attaching `schemas` enables schema-driven type checking.
- A `query` asserts what the query compiles to; the modules are its environment.
- Activate future keywords with an `import` in the module.
- No trailing whitespace in a module.

Fixtures are golden files of what OPA produces: they catch unreviewed change, they do not
validate OPA.

## Running

```
make generate                                     # regenerate fixtures
go test ./v1/ast -run TestCompileCases            # run the corpus against OPA's compiler
go test ./v1/test/compilecases                    # schema and loader
go test ./build/generate-compiler-cases           # generator, including fixture drift
```
