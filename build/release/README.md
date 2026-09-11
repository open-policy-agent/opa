# release

Release-prep tooling for OPA.

Nothing here mutates git or writes to GitHub. Output lands in the working tree
for review via `git status` / `git diff`.

## Usage

```sh
make release-prepare VERSION=1.19.0
```

Runs both `changelog` and `artefacts` subcommands in order.

After the release is tagged, reopen development:

```sh
make dev-prepare VERSION=1.19.1     # the *next* release
```

To run each subcommand alone (e.g. to pass in options the make target ignores):

```sh
cd build/release
go run . changelog --version 1.19.0            # render to stdout, write nothing
go run . artefacts --version 1.19.0 --dry-run
go run . dev --version 1.19.1 --dry-run
```

## `changelog`

```
changelog [--version X.Y.Z] [--from ref] [--to ref] [--repo owner/name]
          [--out path | --update path] [--include-local] [--record dir]
```

Renders the CHANGELOG section for a commit range. Without `--update` it writes
nothing to the repo, so that is the dry run. `--version` adds the `## X.Y.Z`
heading and is required with `--update`.

1. Enumerate non-merge commits in `--from..--to` (default `<latest tag>..HEAD`).
2. Per commit, ask GitHub for the author, the associated PRs, and the first PR's
   `closingIssuesReferences` — the PR "Development" panel, which REST does not
   expose. A commit missing from the remote (HTTP 422) falls back to
   `Fixes/Closes/Resolves #N` trailers in the local message and is excluded
   unless `--include-local`.
3. Derive an area prefix: existing `<area>: ` prefix, else the changed-path table.
   Capitalize the subject, flag dependency bumps.
4. Drop release-mechanics commits (`Release vX.Y.Z`, `Prepare vX.Y.Z
   development`, `Integrate X.Y.Z patch release`), bot-authored dependency
   commits, and dependency commits confined to `.github/`, `e2e/` or `docs/`.
5. Diff direct `go.mod` requires across the range. Any module no surviving commit
   names gets a bare bullet; the rest are reported as already covered.
6. Render one `### Miscellaneous` list, sorted by subject, dependency bumps
   nested. Bullets link to the issue over the PR over the commit.
7. `--update` renames a `## Unreleased` heading to `## X.Y.Z` and appends the
   bullets after any hand-written prose in that section; with no such heading, a
   new section goes above the topmost release. An existing `## X.Y.Z` is an
   error.

Every decision in steps 3–5 is logged to stderr, followed by a review checklist
of what needs a human: commits with several PRs, PRs closing several issues,
commits with no PR, and local-only commits.

**Expect to edit the result.** Area prefixes are a guess, and the tool has no way
to infer which entries are worth listing or how to group them topically.

### Auth

`$GITHUB_TOKEN`, else `gh auth token`. Effectively required: GraphQL rejects
unauthenticated requests outright and REST allows 60/hour. `public_repo` is
enough. Transient failures retry three times with backoff; a sustained outage
means starting the range over.

## `artefacts`

```
artefacts --version X.Y.Z [--repo-root path] [--dry-run] [--skip-generate]
```

1. `v1/version/version.go` — set `Version`.
2. Regenerate `capabilities.json`.
3. Copy it to `capabilities/vX.Y.Z.json`.
4. Regenerate `builtin_metadata.json` and `v1/ast/version_index.json`.

The order is forced: `capabilities.json` must be current before it is
snapshotted, and the version index reads the `go:embed`'d `capabilities/`
directory, so it only sees the new snapshot once that file is on disk.

Runs the three generators directly rather than `make generate`, which depends on
`wasm-lib-build` and would rebuild `opa.wasm` through docker into the release
diff. Takes a few seconds; `--skip-generate` exists for iterating on the version
bump alone. Re-running is safe.

## `dev`

```
dev --version X.Y.Z [--repo-root path] [--dry-run] [--allow-existing-unreleased]
```

Reopens development once the release is tagged. `--version` is the *next*
release, not the one just cut.

1. Add a `## Unreleased` heading above the topmost release in `CHANGELOG.md`.
2. `v1/version/version.go` — set `Version` to `X.Y.Z-dev`.

In that order, so a CHANGELOG that already has an `## Unreleased` heading — which
is an error — fails before `version.go` is touched.
`--allow-existing-unreleased` downgrades that to a warning: the heading is left
alone and the version bump still happens.

## Tests

```sh
make check-release-tool     # vet, test, lint — this module is not covered by the root targets
```

`internal/changelog` has a golden test over `testdata/scenarios`, a hand-authored
fixture with one commit per pipeline branch. It is hermetic: no network (the
`gh.Client` is a replay over recorded maps) and no git (commit list, messages and
both `go.mod` snapshots come from disk). The data is fictional — fake repo,
modules, SHAs and authors — and two tests keep it that way.

`--record dir` on a real run captures a fresh fixture. `-update` on the test
rewrites the goldens.
