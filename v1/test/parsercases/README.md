# Parser conformance corpus

YAML cases for the Rego parser: a module or query in, an AST or diagnostics out. This package
holds the schema and the loader; the cases are in `testdata/<version>/<area>/*.yaml`.

Using the corpus from another parser or compiler: [`build/conformance-corpora.md`](../../../build/conformance-corpora.md).

## Adding a case

Add it to the file for its area under `testdata/v1/` (`testdata/v0/` for v0-only syntax),
with a `note` unique within the version directory and a `module` or `body`:

```yaml
  - note: terms/non-terminated-array
    module: |
      package test

      p := [1, 2
    exhaustive: true
```

Then run `make generate` and review the diff. The parser decides whether a case is a success
or a failure case: `want_ast` or `want_errors` is filled in accordingly.

- `want_ast` is regenerated on every run; reviewing the diff is the gate.
- `want_errors` is filled in only where a case has none. To re-seed one after a deliberate
  change, delete it and regenerate.
- To pin a later diagnostic, author only its `message`. `exhaustive: true` records all of them.
- Wrap a term as `p := <term>` and an expression as `p if { <expr> }`. A wrapped case has the
  note suffix ` (module)` and a ` (body)` sibling with the Rego unwrapped.
- Activate future keywords with an `import` in the module; a `body` uses the `imports` field.
- No trailing whitespace in `module`, `body` or `want_equivalent`.

Fixtures are golden files of what OPA produces: they catch unreviewed change, they do not
validate OPA.

## Running

```
make generate                                    # regenerate fixtures
go test ./v1/ast -run TestParseCases             # run the corpus against OPA's parser
go test ./v1/test/parsercases                    # schema and loader
go test ./build/generate-parser-cases            # generator
```

`build/generate-parser-cases/exceptions.yaml` excludes cases by note or glob when loading.
