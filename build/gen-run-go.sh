#!/bin/sh

set -e

GOOS= GOARCH= CC= go run -tags generate $@
