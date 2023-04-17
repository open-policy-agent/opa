#!/usr/bin/env bash

set -o errexit
set -o pipefail
set -o nounset

export GO111MODULE=on
export GOFLAGS=-mod=vendor

function opa::go_packages() {
    for pkg in $(go list ./.../ 2>/dev/null | grep -v vendor); do
        echo $pkg
    done
}

function opa::go_files_in_package() {
    dir=$(go list -f '{{ .Dir }}' $1)
    for file in $(go list -f '{{ join .GoFiles "\n" }}' $1); do
        echo  $dir/$file
    done
    for file in $(go list -f '{{ join .TestGoFiles "\n" }}' $1); do
        echo $dir/$file
    done
}

function opa::all_go_files() {
    FILES=""
    for pkg in $(opa::go_packages); do
        for file in $(opa::go_files_in_package $pkg); do
            FILES+=" ${file}"
        done
    done
    echo "${FILES}"
}
