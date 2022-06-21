# Compiler engine

This package implements the Compiler engine for WebAssembly *purely written in Go*.
In this README, we describe the background, technical difficulties and some design choices.

## General limitations on pure Go Compiler engines

In Go program, each Goroutine manages its own stack, and each item on Goroutine
stack is managed by Go runtime for garbage collection, etc.

These impose some difficulties on compiler engine purely written in Go because
we *cannot* use native push/pop instructions to save/restore temporary
variables spilling from registers. This results in making it impossible for us
to invoke Go functions from compiled native codes with the native `call`
instruction since it involves stack manipulations.

*TODO: maybe it is possible to hack the runtime to make it possible to achieve
function calls with `call`.*

## How to generate native codes

wazero uses its own assembler, implemented from scratch in the
[`internal/asm`](../../asm/) package. The primary rationale are wazero's zero
dependency policy, and to enable concurrent compilation (a feature the
WebAssembly binary format optimizes for).

Before this, wazero used [`twitchyliquid64/golang-asm`](https://github.com/twitchyliquid64/golang-asm).
However, this was not only a dependency (one of our goals is to have zero
dependencies), but also a large one (several megabytes added to the binary).
Moreover, any copy of golang-asm is not thread-safe, so can't be used for
concurrent compilation (See [#233](https://github.com/tetratelabs/wazero/issues/233)).

The assembled native codes are represented as `[]byte` and the slice region is
marked as executable via mmap system call.

## How to enter native codes

Assuming that we have a native code as `[]byte`, it is straightforward to enter
the native code region via Go assembly code. In this package, we have the
function without body called `nativecall`

```go
func nativecall(codeSegment, engine, memory uintptr)
```

where we pass `codeSegment uintptr` as a first argument. This pointer is to the
first instruction to be executed. The pointer can be easily derived from
`[]byte` via `unsafe.Pointer`:

```go
code := []byte{}
/* ...Compilation ...*/
codeSegment := uintptr(unsafe.Pointer(&code[0]))
nativecall(codeSegment, ...)
```

And `nativecall` is actually implemented in [arch_amd64.s](./arch_amd64.s)
as a convenience layer to comply with the Go's official calling convention.
We delegate the task to jump into the code segment to the Go assembler code.

## How to achieve function calls

Given that we cannot use `call` instruction at all in native code, here's how
we achieve the function calls back and forth among Go and (compiled) Wasm
native functions.

TODO:
