Loader Example
==============

[main.go](./main.go) loads a bundle either from a file or HTTP server.

## Setup

The example directory includes some Rego source files. The first step is to
compile them into Wasm modules. Run:

```shell
opa build -t wasm -e example/allow ./example.rego
```

## Running the Example

In the directory of the main.go, execute:
```
go run main.go bundle.tar.gz
```
To load the accompanied bundle file. Similarly, execute:
```
go run main.go http://url/to/bundle.tar.gz
```
to test downloading the bundle from a HTTP server.
