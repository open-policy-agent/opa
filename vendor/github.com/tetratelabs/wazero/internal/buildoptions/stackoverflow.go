package buildoptions

// CallStackCeiling is the maximum WebAssembly call stack height. This allows wazero to raise
// wasm.ErrCallStackOverflow instead of overflowing the Go runtime.
//
// The default value should suffice for most use cases. Those wishing to change this can via `go build -ldflags`.
var CallStackCeiling = 2000
