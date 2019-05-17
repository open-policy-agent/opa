#!/usr/bin/env bash

set -e

mkdir -p _test
go run test/wasm/cmd/testgen.go --input-dir test/wasm/assets --output _test/testcases.tar.gz $@
docker run -it --rm -e VERBOSE=$VERBOSE -v $PWD:/src -w /src node:8 bash -c 'mkdir -p /test; tar xzf _test/testcases.tar.gz -C /test; cd /test; node test.js opa.wasm'
