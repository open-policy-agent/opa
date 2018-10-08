#!/usr/bin/env bash

set -e

mkdir -p /test
tar xzf _test/testcases.tar.gz -C /test
cd /test

node test.js opa.wasm