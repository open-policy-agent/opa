#!/bin/bash

set -exo pipefail

# NOTE(sr): we include version because that's what drives releases
# Makefile and netlify.toml capture when the build infrastructure changes
# ast/builtins.go and capabilities.json are driving the builtins_metadata.
git diff --name-only --exit-code $CACHED_COMMIT_REF $COMMIT_REF docs/ Makefile build/ netlify.toml ast/builtins.go capabilities.json version/ CHANGELOG.md
