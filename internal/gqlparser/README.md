# gqlparser (Details of library residing in OPA's internal)

## Description

https://github.com/vektah/gqlparser was duplicated into `internal/gqlparser` folder, so that we no longer have to track the external library 1-to-1, and so that OPA library users who want to use newer/older gqlparser versions won't have to match our GraphQL parser's version.

The current version we have forked from is commit [`b3be96f` on branch `master`](https://github.com/vektah/gqlparser/commit/b3be96ff69fa97682c43570dcb6f75d08fdf8586), which is 2 commits past the [`v2.5.1`](https://github.com/vektah/gqlparser/releases/tag/v2.5.1) release tag.
We picked this specific commit because the two commits after the 2.5.1 release dramatically improve the linter state of the library to be as strict or stricter than OPA's linting, allowing for nearly drop-in integration.

We currently modify `gqlparser/gqlerror/error.go` to provide the line and column of the error location, so as to keep our `graphql` builtin error messages consistent.
This requires either modifying all the library tests, or removing them.
For now, at least until upstream adds columns to error messages, we will just remove tests from the imported library with the `remove-tests.sh` script.

## Rewriter script

The `rewrite-deps.sh` can be run from this directory, and it will do the grunt work of rewriting all import path prefixes for `gqlparser` sub-packages, so that they import this package.
It also will add some linter ignore annotations on the validator rules, since those are tedious to do by hand.

The script thus should alleviate around 40-60% of the linter-fixup work required during a version bump.

## Original README

This is a parser for graphql, written to mirror the graphql-js reference implementation as closely while remaining idiomatic and easy to use.

spec target: June 2018 (Schema definition language, block strings as descriptions, error paths & extension)

This parser is used by [gqlgen](https://github.com/99designs/gqlgen), and it should be reasonably stable.

Guiding principles:

 - maintainability: It should be easy to stay up to date with the spec
 - well tested: It shouldn't need a graphql server to validate itself. Changes to this repo should be self contained.
 - server agnostic: It should be usable by any of the graphql server implementations, and any graphql client tooling.
 - idiomatic & stable api: It should follow go best practices, especially around forwards compatibility.
 - fast: Where it doesn't impact on the above it should be fast. Avoid unnecessary allocs in hot paths.
 - close to reference: Where it doesn't impact on the above, it should stay close to the [graphql/graphql-js](https://github.com/graphql/graphql-js) reference implementation.
