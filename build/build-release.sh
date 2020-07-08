#!/usr/bin/env bash
# Script to build OPA releases. Assumes execution environment is golang Docker container.

set -e

OPA_DIR=/go/src/github.com/open-policy-agent/opa
BUILD_DIR=$OPA_DIR/build

usage() {
    echo "build-release.sh --output-dir=<path>"
    echo "                 --source-url=<git-url>"
    echo "                 [--version=<mj.mn.pt>]"
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
    --version=*)
        VERSION="${i#*=}"
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

build_release() {
    make build-all-platforms RELEASE_DIR="${OUTPUT_DIR}"
}

clone_repo() {
    git clone $SOURCE_URL /go/src/github.com/open-policy-agent/opa
    cd /go/src/github.com/open-policy-agent/opa
    if [ -n "$VERSION" ]; then
        git checkout v${VERSION}
    fi
}

main() {
    clone_repo
    build_release
}

main
