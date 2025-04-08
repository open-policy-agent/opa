#!/bin/bash
# Run goversioninfo to generate the resource.syso to embed version info.
set -eux

NAME="One Policy Agent (OPA)"
VERSION=$(./build/get-build-version.sh)
FLAGS=()

# If building for arm64, then include the extra flags required.
if [ -n "${1+x}" ] && [ "$1" = "arm64" ]; then
    FLAGS=(-arm -64)
fi

goversioninfo "${FLAGS[@]}" \
    -product-name "$NAME" \
    -product-version "$VERSION" \
    -skip-versioninfo \
    -icon=logo/logo.ico \
    -o resource.syso