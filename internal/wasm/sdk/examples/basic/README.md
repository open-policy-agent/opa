Basic Wasm Module Evaluation Example
====================================

The [main.go](./main.go) example demonstrates the loading and executing of OPA
produced wasm policy binary.

## Setup

The example directory includes some Rego source files. The first step is to
compile them into Wasm modules. Run:

```shell
opa build -t wasm -e example/allow ./example-1.rego
```

This will generate a `bundle.tar.gz` in your current directory which has the Wasm module included. Extract the module with:

```shell
tar -zxvf bundle.tar.gz /policy.wasm
mv policy.wasm example-1.wasm
```

Repeat the process for the second example Wasm binary:
```shell
opa build -t wasm -e example/allow ./example-2.rego
tar -zxvf bundle.tar.gz /policy.wasm
mv policy.wasm example-2.wasm
```

The final result should be two Wasm modules:

```
example-1.wasm
example-2.wasm
```

## Running the Example

After the Wasm binaries are available run the example with
```shell
go run main.go .
```
> This should be run from the same directory as `main.go`, alternatively provide
  a path to the directory which contains the Wasm binaries.