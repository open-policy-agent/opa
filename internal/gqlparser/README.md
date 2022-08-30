# gqlparser (Details of library residing in OPA's internal)

## Description

https://github.com/vektah/gqlparser was duplicated into `internal/gqlparser` folder, so that we no longer have to track the external library 1-to-1, and so that OPA library users who want to use newer/older gqlparser versions won't have to match our GraphQL parser's version.

The current version we have forked from is the [`v2.4.8`](https://github.com/vektah/gqlparser/releases/tag/v2.4.8) release. (Commit: `6d97050` on `master`)

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
