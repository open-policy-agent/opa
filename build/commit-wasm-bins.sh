#!/usr/bin/env bash

OPA_DIR=$(dirname "${BASH_SOURCE}")/..

cd ${OPA_DIR}

WASMFILES=(
  "internal/compiler/wasm/opa/opa.go"
  "internal/compiler/wasm/opa/opa.wasm"
  "internal/compiler/wasm/opa/callgraph.csv"
)

for file in "${WASMFILES[@]}"; do
  git add ${file}
done

if [[ -z "$(git diff --name-only --cached)" ]]; then
  echo "No Wasm changes to commit!"
  exit 1
fi

git commit -m "wasm: Update generated binaries"

echo ""
echo "Committed changes for files:"
git diff-tree --no-commit-id --name-only -r HEAD
echo ""
