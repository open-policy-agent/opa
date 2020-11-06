#!/usr/bin/env bash

OPA_DIR=$(
    dir=$(dirname "${BASH_SOURCE}")/..
    cd "$dir"
    pwd
)
source $OPA_DIR/build/utils.sh

function opa::check_vet() {
    exec 5>&1
    rc=0
    exit_code=0
    for pkg in $(opa::go_packages); do
        go vet -tags=opa_wasmer $pkg || rc=$?
        if [[ $rc != 0 ]]; then
            exit_code=1
        fi
    done
    exit $exit_code
}

opa::check_vet
