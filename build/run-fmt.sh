#!/usr/bin/env bash

OPA_DIR=$(dirname "${BASH_SOURCE}")/..
source ${OPA_DIR}/build/utils.sh

${OPA_DIR}/build/run-goimports.sh -format-only -w $(opa::all_go_files)
