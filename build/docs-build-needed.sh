#!/bin/bash

set -exo pipefail

if [ "$PULL_REQUEST" == "true" ]; then
  BASE=main
else
  BASE=$CACHED_COMMIT_REF
fi

# NOTE(sr): we include version because that's what drives releases
# Makefile and netlify.toml capture when the build infrastructure changes
# ast/builtins.go and capabilities.json are driving the builtins_metadata.
git diff --exit-code $BASE $COMMIT_REF docs/ Makefile build/ netlify.toml ast/builtins.go capabilities.json version/
