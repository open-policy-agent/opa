#!/usr/bin/env bash

set -ex

# Expects to be run in openpolicyagent/fuzzer docker image from the root of the project.

OPA_DIR=$(dirname "${BASH_SOURCE}")/..
cd ${OPA_DIR}
go-fuzz-build -libfuzzer -o ast-fuzzer.a ./ast/
clang -fsanitize=fuzzer ast-fuzzer.a -o ast-fuzzer
fuzzit create job --type "fuzzing" opa/ast ast-fuzzer
