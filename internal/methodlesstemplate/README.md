# methodlesstemplate

A vendored copy of the Go standard library's `text/template`, with **one** behavioral change: the
method-calls-on-data branch in `exec.go`'s `evalField` (`reflect.Value.MethodByName`) is removed.

A reachable, non-constant `reflect.Value.MethodByName` makes the Go linker disable method-level
dead-code elimination for the **entire binary** (golang/go#72895). OPA reaches `text/template` from
its compiler (schema errors) and the `strings.render_template` builtin, so that one edge retains the
full reflected method surface of every embedder — a large binary-size regression (#7903). Rego
values and gojsonschema `ErrorDetails` decode to `map[string]any` / `[]any` / scalars, which have no
methods, so eliding the data-method lookup is a behavioral no-op while restoring DCE.

Only `exec.go` differs from the upstream stdlib. `doc.go`, `funcs.go`, `option.go`, `template.go`,
and `internal/fmtsort/sort.go` are byte-identical to their Go release; `text/template/parse` is
reused via its normal import. `helper.go` (ParseFiles/ParseGlob/ParseFS) is intentionally not
vendored. Go's BSD `LICENSE` and per-file copyright headers are preserved.

## Regenerating

Do not hand-edit these files. To re-sync to a new Go release, run under the target toolchain:

```
GOTOOLCHAIN=go1.26.0 build/regen-methodless-template.sh
```

The script copies the stdlib files verbatim and re-applies the single method-elision edit. If the
edit no longer applies (the stdlib changed that region), the script fails and the elision must be
re-derived and the patch in the script updated.
