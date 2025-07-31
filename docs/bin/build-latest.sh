#!/usr/bin/env bash

set -euo pipefail

if ! git diff --quiet HEAD -- || ! git diff --cached --quiet; then
    echo "Latest release build must be done without working changes"
    git status
    exit 1
fi

git fetch --tags origin

CURRENT_REF=$(git rev-parse HEAD)

LATEST_TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)

if [ -z "$LATEST_TAG" ]; then
    echo "No valid release tag found (expected format: v1.2.3)"
    exit 1
fi

git checkout "$LATEST_TAG"

BUILD_VERSION="$LATEST_TAG" npx docusaurus build

git checkout "$CURRENT_REF"
