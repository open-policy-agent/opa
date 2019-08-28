#!/bin/bash
# Build fuzz targets
go get -u github.com/dvyukov/go-fuzz/go-fuzz github.com/dvyukov/go-fuzz/go-fuzz-build
go-fuzz-build -libfuzzer -o ast-fuzzer.a ./ast/
clang -fsanitize=fuzzer ast-fuzzer.a -o ast-fuzzer

# Run regression or upload fuzzers to fuzzit
wget -q -O fuzzit https://github.com/fuzzitdev/fuzzit/releases/download/v2.4.29/fuzzit_Linux_x86_64
chmod a+x fuzzit
./fuzzit create job --type "${1}" opa/ast ast-fuzzer