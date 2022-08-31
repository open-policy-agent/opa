#!/bin/bash

REPO_DIR=./graphql-js
EXPORTER_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

cd "$EXPORTER_ROOT" || exit

GIT_REF=origin/main

if [[ -f "$EXPORTER_ROOT/graphql-js-commit.log" ]] ; then
  GIT_REF=$(cat "$EXPORTER_ROOT/graphql-js-commit.log")
fi
echo $GIT_REF

if [[ -d "$REPO_DIR" ]] ; then
    echo "fetching graphql-js with ${GIT_REF}"
    cd "$REPO_DIR" || exit
    git fetch origin master
    git checkout "$GIT_REF"
    git reset --hard
else
    echo "cloning graphql-js with ${GIT_REF}"
    git clone --no-tags --single-branch -- https://github.com/graphql/graphql-js $REPO_DIR
    cd "$REPO_DIR" || exit
    git checkout "$GIT_REF"
fi
git rev-parse HEAD > $EXPORTER_ROOT/graphql-js-commit.log

cd "$EXPORTER_ROOT" || exit

echo "installing js dependencies"
npm ci

echo "exporting tests"
npx babel-node -x ".ts,.js" ./export.js
