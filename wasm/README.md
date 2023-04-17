# OPA-WASM

This directory contains a library that implements various low-level
operations for policies compiled into WebAssembly (Wasm). Specifically, the
library implements most of the built-in functions provided by OPA. See the "Wasm"
tag next to each function listed in the
[built-in function reference](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions)
for the complete list, or the 
[builtin_metadata.json](https://github.com/open-policy-agent/opa/blob/main/builtin_metadata.json) file for programmatic purposes.

This library does not make any backwards compatibility guarantees.

## Documentation

See the [OPA docs](https://www.openpolicyagent.org/docs/latest/wasm/) on Wasm for an overview of the integration,
compilation options, and more.

## Development

You should have Docker installed to build and test changes to the library. We
commit the output of the build (`opa.wasm`) into the repository, so it's
important for the build output to be reproducible.

You can build the library by running `make build`. This will produce WASM
executables under the `_obj` directory.

You can test the library by running `make test`. By default, the test runner
does not print messages when tests pass. If you run `make test VERBOSE=1` it
will log all of the tests that were run.

You can run `make hack` to start a shell inside the builder image. This is
useful if you need to interact with low-level WASM tooling like
`wasm-objdump`, `wasm2wat`, etc. or LLVM itself.

You must manually push the builder image if you make changes to it (run `make
builder` to produce a new Docker image).

### Debug Builds

Set the `DEBUG` environment variable to `1` to enable generating binaries with
debug symbols and a less aggressive optimization level. Eg: `DEBUG=1 make build`.

## Vendoring

If you make changes to the library, run the `make generate` in the parent
directory and commit the results back to the repository. The `generate`
target will:

1. Build the OPA-WASM library
2. Copy the library into the [internal/compiler/wasm/opa](../internal/compiler/wasm/opa) directory.
3. Run the tool to generate the [internal/compiler/wasm/opa/opa.go](../internal/compiler/wasm/opa/opa.go) file.
