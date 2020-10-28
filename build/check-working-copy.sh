#!/usr/bin/env bash

EXCEPTIONS=(
  "internal/compiler/wasm/opa/opa.go"
  "internal/compiler/wasm/opa/opa.wasm"
)

STATUS=$(git status --porcelain)

HAS_CHANGES=0

if [[ -z "${STATUS}" ]]; then
  exit 0
else
  for file in $(echo -E "${STATUS}" | awk '{print $2}'); do
    if [[ "${EXCEPTIONS[@]}" =~ "${file}" ]]; then
      echo "Ignoring changed file: ${file}"
    else
      HAS_CHANGES=1
    fi
  done
fi

if [[ "${HAS_CHANGES}" == "1" ]]; then
  echo ""
  echo "git status"
  git status
  exit 1
fi
