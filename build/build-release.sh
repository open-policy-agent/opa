#!/usr/bin/env bash

# This script will build an OPA release given (1) the release version and
# (2) the output directory to write artifacts to. This script assumes it is
# running under Docker.

set -ex

VERSION=$1
RELEASE_DIR=$2

git clone https://github.com/open-policy-agent/opa.git /go/src/github.com/open-policy-agent/opa
cd /go/src/github.com/open-policy-agent/opa
git checkout v$VERSION

build_binaries() {
    make deps
    GOOS=darwin GOARCH=amd64 make build
    GOOS=linux GOARCH=amd64 make build
    mv opa_*_* $RELEASE_DIR
}

build_site() {
    pushd site
    jekyll build . && rm _site/assets/.sprockets-manifest-*.json
    tar czvf $RELEASE_DIR/site.tar.gz -C _site .
    popd
}

main() {
    build_binaries
    build_site
    make test
}

main
