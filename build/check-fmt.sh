#!/usr/bin/env bash

OPA_DIR=$(dirname "${BASH_SOURCE}")/..
source ${OPA_DIR}/build/utils.sh

function opa::check_fmt() {
    exec 5>&1
    exit_code=0
    out=$(${OPA_DIR}/build/run-goimports.sh -format-only -d $(opa::all_go_files) | tee >(cat - >&5))
    if [ ! -z "$out" ]; then
        exit_code=1
    fi
    exit $exit_code
}

opa::check_fmt
