#!/usr/bin/env bash
set -e

# Relies on perl to do in-place regex search-and-replace for the module strings.
# Relies on find to recursively enumerate all the Go files.

# Rewrite imports to use this module.
for f in $(find . -name "*.go"); do perl -pi -e "s/github.com\/vektah\/gqlparser\/v2/github.com\/open-policy-agent\/opa\/internal\/gqlparser/" $f; done

# Insert the following linter-ignore comment into the validator rules files.
#   //nolint:ignore ST1001 Validator rules each use dot imports for convenience.
for f in $(find validator/rules -name "*.go"); do perl -pi -e 's/\t. "github.com\/open-policy-agent\/opa\/internal\/gqlparser\/validator"/\n\t\/\/nolint:revive \/\/ Validator rules each use dot imports for convenience.\n\t. "github.com\/open-policy-agent\/opa\/internal\/gqlparser\/validator"/' $f; done
