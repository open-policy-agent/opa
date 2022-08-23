#!/bin/bash

set -eo pipefail

# Inspired by https://github.com/aptos-labs/aptos-core/pull/1324
# Docs: https://docs.netlify.com/configure-builds/ignore-builds/
if [ "$PULL_REQUEST" == "true" ]; then
  BASE=main
else
  BASE=$CACHED_COMMIT_REF
fi

# NOTE(sr): we include version because that's what drives releases
# Makefile and netlify.toml capture when the build infrastructure changes
# ast/builtins.go and capabilities.json are driving the builtins_metadata.
git diff --quiet $BASE $COMMIT_REF docs/ Makefile netlify.toml ast/builtins.go capabilities.json version/