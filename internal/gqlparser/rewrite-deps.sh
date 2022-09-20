#!/usr/bin/env bash
set -e

# Relies on perl to do in-place regex search-and-replace for the module strings.
# Relies on find to recursively enumerate all the Go files.

# Rewrite imports to use this module.
for f in $(find . -name "*.go"); do perl -pi -e "s/github.com\/vektah\/gqlparser\/v2/github.com\/open-policy-agent\/opa\/internal\/gqlparser/" $f; done
