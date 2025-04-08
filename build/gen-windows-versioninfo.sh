#!/bin/bash
# Run goversioninfo to generate the resource.syso to embed version info.
set -eux

NAME="One Policy Agent (OPA)"
VERSION=$(./build/get-build-version.sh)

goversioninfo \
    -product-name "$NAME" \
    -product-version "$VERSION" \
    -skip-versioninfo \
    -icon=logo/logo.ico \
    -o resource.syso