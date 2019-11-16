---
title: WebAssembly
kind: misc
weight: 1
---

# What is Wasm?

As described on [https://webassembly.org/](https://webassembly.org/)

> WebAssembly (abbreviated Wasm) is a binary instruction format for a stack-based virtual machine. Wasm is designed as a portable target for compilation of high-level languages like C/C++/Rust, enabling deployment on the web for client and server applications.

# OPA & Wasm

OPA is able to compile Rego policies into a Wasm binary which can be evaluated with different
inputs and external data. This is *not* running the OPA server in Wasm, nor is this just cross compiled
Golang code. The compiled binary is a planned evaluation path for the source policy and query.

## Current Status

The core language is supported fully but there are a number of builtin functions that are not, and probably won't be
natively supported in Wasm (eg. `http.send`. Future plans are to have callouts to the native runtime to handle these
and custom functions the caller wishes to supply implementations for.

# Compiling Policy

Either use the [Compile REST API](../rest-api/#compile-api) or `opa build` CLI tool.

For example:

```bash
opa build -d example.rego 'data.example.allow = true'
```
Which is compiling the `example.rego` policy file with the query `data.example.allow = true`.

See `opa build --help` for more details.

> Note: The query must be specified at compilation time and cannot be changed without recompiling the binary!

# Using the Compiled Policy
## JavaScript SDK
There is a JavaScript SDK available which simplifies much of the process to load and evaluate the Wasm module. It is
recommended for JavaScript use-cases that the SDK is used.

See [https://github.com/open-policy-agent/npm-opa-wasm](https://github.com/open-policy-agent/npm-opa-wasm) for more details.

There is an example NodeJS application located [here](https://github.com/open-policy-agent/npm-opa-wasm/tree/master/examples/nodejs-app)

## Instantiating the Wasm Module

Once compiled the module needs to be loaded in the Wasm runtime. At a high level there must be a memory buffer,
required import functions, and any data or input loaded into the shared memory buffer.

### Exports

The primary exported functions for interacting with the compiled policy are:

| Function Name | Return | Description|
|---------------|--------|------------|
| `eval(ctxAddr)` |  | Evaluates the loaded policy with the supplied evaluation context address. |
| `opa_eval_ctx_new()` | `ctxAddr` | Returns an address to new evaluation context object. |
| `opa_eval_ctx_set_input(ctxAddr, parsedInputAddr)` | | Set the input address for the next evaluation with the supplied context. |
| `opa_eval_ctx_set_data(ctxAddr, parsedDataAddr)`  | | Set the data address for the next evaluation with the supplied context. |
| `opa_eval_ctx_get_result(ctxAddr)` | `resultAddr` | Get the address to the evaluation result. Only available after calling `eval`. |
| `opa_malloc(size)` | `addr` | Allocates a buffer within the shared memory for the module. |
| `opa_json_parse(stringAddr, length)` | `parsedAddr` | Parse the JSON string at the supplied address. |
| `opa_json_dump(addr)` | `stringAddr` | Dump the object at the supplied address into a null terminated JSON string. |
| `opa_heap_ptr_set(addr)` | | Set the heap pointer for the next evaluation. |
| `opa_heap_ptr_get()` | `addr` | Get the current heap pointer. |
| `opa_heap_top_set(addr)` | | Set the heap top for the next evaluation. |
| `opa_heap_top_get()` | `addr` | Get the current heap top. |

### Imports

The binary will need the following imports:

* `opa_abort(addr)`: The `addr` will be a pointer to a null terminated string within the shared memory buffer. 
* `opa_println(addr)`: The `addr` will be a pointer to a null terminated string within the shared memory buffer. 
* `memory`: A shared memory buffer.

### Memory Buffer

A shared memory buffer must be provided as an import for the policy Wasm module with the `memory` key of the import
object. The buffer must be large enough to accommodate the input, provided data, and result of evaluation.

## Evaluation

Once instantiated the policy module is ready to be evaluated. An evaluation context must be created via the exported
`opa_eval_ctx_new` method. The returned address will be required to load input or external data.

To evaluate, make a call to the exported `eval` function with the eval context address as the only parameter.

### Input

The (optional) `input` document for a policy can be provided by loading a JSON string into the shared memory buffer. Use
 the `opa_malloc` exported function to allocate a buffer the size of the JSON string and copy the contents in at the
returned address. After the raw string is loaded into memory you will need to call the `opa_json_parse` exported method
to get an address to the parsed input document for use in evaluations. Set the address via the `opa_eval_ctx_set_input`
builtin supplying the evaluation context address and parsed input document address.

### External Data

External data can be loaded for use in evaluation. Similar to the `input` this is done by loading a JSON string into
the shared memory buffer. Use `opa_malloc` and `opa_json_parse` followed by `opa_eval_ctx_set_data` to set the address
on the evaluation context.

After loading the external data use the `opa_heap_ptr_get` and `opa_heap_top_get` exported methods to save the current
point in the heap before evaluation. After evaluation these should be reset by calling `opa_heap_ptr_set` and
`opa_heap_top_set` to ensure that evaluation restarts back at the saved data and re-uses heap space. This is
particularly important if re-evaluating many times with the same data.

### Results

After evaluation results can be retrieved via the exported `opa_eval_ctx_get_result` method. Pass in the evaluation
context address. The return value is an address in the shared memory buffer to the structured result. To access the
JSON result use the `opa_json_dump` exported function to retrieve a pointer in shared memory to a null terminated JSON
string.
