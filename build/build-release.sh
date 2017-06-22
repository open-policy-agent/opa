#!/usr/bin/env bash
# Script to build OPA releases. Assumes execution environment is golang Docker container.

set -e

usage() {
    echo "build-release.sh --output-dir=<path>"
    echo "                 --source-url=<git-url>"
    echo "                 [--tag=<tag>]"
}

for i in "$@"; do
    case $i in
    --source-url=*)
        SOURCE_URL="${i#*=}"
        shift
        ;;
    --output-dir=*)
        OUTPUT_DIR="${i#*=}"
        shift
        ;;
    --tag=*)
        TAG="${i#*=}"
        shift
        ;;
    *)
        usage
        exit 1
        ;;
    esac
done

if [ -z "$OUTPUT_DIR" ]; then
    usage
    exit 1
elif [ -z "$SOURCE_URL" ]; then
    usage
    exit 1
fi

build_binaries() {
    make deps
    GOOS=darwin GOARCH=amd64 make build
    GOOS=linux GOARCH=amd64 make build
    mv opa_*_* $OUTPUT_DIR
}

build_site() {
    pushd site
    jekyll build . && rm _site/assets/.sprockets-manifest-*.json
    tar czvf $OUTPUT_DIR/site.tar.gz -C _site .
    popd
}

clone_repo() {
    git clone $SOURCE_URL /go/src/github.com/open-policy-agent/opa
    cd /go/src/github.com/open-policy-agent/opa
    if [ -n "$TAG" ]; then
        git checkout $TAG
    fi
}

main() {
    clone_repo
    build_binaries
    build_site
    make test
}

main
