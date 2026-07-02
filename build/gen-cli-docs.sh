#!/usr/bin/env bash

SCRIPT_DIR=$(cd $(dirname "${BASH_SOURCE[0]}") && pwd)

GOOS="" GOARCH="" go run "$SCRIPT_DIR"/generate-cli-docs/generate.go "$@"
