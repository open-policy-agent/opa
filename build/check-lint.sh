#!/usr/bin/env bash

OPA_DIR=$(
    dir=$(dirname "${BASH_SOURCE}")/..
    cd "$dir"
    pwd
)
source $OPA_DIR/build/utils.sh


function opa::check_lint() {
    exec 5>&1
    exit_code=0
    __output=$(go run ./vendor/github.com/golangci/golangci-lint/cmd/golangci-lint/main.go run | tee >(cat - >&5))
    if [ ! -z "$__output" ]; then
        exit_code=1
    fi
    exit $exit_code
}

opa::check_lint
