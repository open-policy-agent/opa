#!/bin/bash
# Run goversioninfo to generate the resource.syso to embed version info.
set -eux

NAME="Open Policy Agent (OPA)"
VERSION=$(./build/get-build-version.sh)
FLAGS=()

# If building for arm64, then include the extra flags required.
if [ -n "${1+x}" ] && [ "$1" = "arm64" ]; then
    FLAGS=(-arm -64)
fi

if ! command -v goversioninfo &> /dev/null; then
    go version
    ls -al /go/bin/
    /go/bin/goversioninfo
    # If goversioninfo isn't on the path, print an error message
    echo "Error: goversioninfo command not found" >&2
    exit 1
fi

goversioninfo "${FLAGS[@]}" \
    -product-name "$NAME" \
    -product-version "$VERSION" \
    -skip-versioninfo \
    -icon=logo/logo.ico \
    -64=true \
    -o resource.syso
