#!/usr/bin/env bash

OPA_DIR=$(dirname "${BASH_SOURCE}")/..
source $OPA_DIR/build/utils.sh

function opa::check_fmt() {
    exec 5>&1
    exit_code=0
    for pkg in $(opa::go_packages); do
        for file in $(opa::go_files_in_package $pkg); do
            __diff=$(gofmt -d $file | tee >(cat - >&5))
            if [ ! -z "$__diff" ]; then
                exit_code=1
            fi
        done
    done
    exit $exit_code
}

opa::check_fmt
