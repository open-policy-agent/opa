#!/usr/bin/env bash

GIT_SHA=$(git rev-parse --short HEAD)

if [ -z "$(git status --porcelain 2>/dev/null)" ]; then
    echo $GIT_SHA
else
    echo "$GIT_SHA-dirty"
fi
