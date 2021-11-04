---
title: WebAssembly
kind: misc
weight: 1
---

# What is WebAssembly (Wasm)?

As described on [https://webassembly.org/](https://webassembly.org/)

> WebAssembly (abbreviated Wasm) is a binary instruction format for a
> stack-based virtual machine. Wasm is designed as a portable target for
> compilation of high-level languages like C/C++/Rust, enabling deployment on
> the web for client and server applications.

## Overview

OPA is able to compile Rego policies into executable Wasm modules that can be
evaluated with different inputs and external data. This is *not* running the OPA
server in Wasm, nor is this just cross-compiled Golang code. The compiled Wasm
module is a planned evaluation path for the source policy and query.

## Current Status

The core language is supported fully but there are a number of built-in
functions that are not, and probably won't be natively supported in Wasm (e.g.,
`http.send`). Built-in functions that are not natively supported can be
implemented in the host environment (e.g., JavaScript).

## Compiling Policies

You can compile Rego policies into Wasm modules using the `opa build` subcommand.

For example, the `opa build` command below compiles the `example.rego` file into a
Wasm module and packages it into an OPA bundle. The `wasm` target requires at least
one entrypoint rule (specified by `-e`).

```bash
opa build -t wasm -e example/allow example.rego
```

The output of a Wasm module built this way contain the `result` of evaluating the
entrypoint rule. For example:
```json
[
  {
    "result": <value of data.example.allow>
  }
]
```

The output of policy evaluation is a set of variable assignments. The variable
assignments specify values that satisfy the expressions in the policy query
(i.e., if the variables in the query are replaced with the values from the
assignments, all of the expressions in the query would be defined and not
false.)

When policies are compiled into Wasm, the user provides the path of the policy
decision that should be exposed by the Wasm module. The policy decision is
assigned to a variable named `result`. The policy decision can be ANY JSON value
(boolean, string, object, etc.) but there will be at-most-one assignment. This
means that callers should first check if the set of variable assignments is
empty (indicating an undefined policy decision) otherwise they should select the
`"result"` key out of the variable assignment set.

> For more information on `opa build` run `opa build --help`.

### Advanced Compiling Options

You can also compile Rego policies into Wasm modules from Go using the lower-level
[rego](https://pkg.go.dev/github.com/open-policy-agent/opa/rego#Rego.Compile) API
that produces raw Wasm executables and the higher-level
[compile](https://pkg.go.dev/github.com/open-policy-agent/opa/compile#Compiler.Build)
API that produces OPA bundle files. The compile API is recommended.

## Using Compiled Policies

### JavaScript SDK

There is a JavaScript SDK available that simplifies the process of loading and
evaluating compiled policies. If you want to evaluate Rego policies inside
JavaScript we recommend you use the JavaScript SDK.

See
[https://github.com/open-policy-agent/npm-opa-wasm](https://github.com/open-policy-agent/npm-opa-wasm)
for more details.

There is an example NodeJS application located
[here](https://github.com/open-policy-agent/npm-opa-wasm/tree/master/examples/nodejs-app).

### From Scratch

If you want to integrate Wasm compiled policies into a language or runtime that
does not have SDK support, read this section.

#### Instantiating the Wasm Module

Before you can evaluate Wasm compiled policies you need to instantiate the Wasm
module produced by the compilation process described earlier on this page.

To load the compiled Wasm module refer the documentation for the Wasm runtime
that you are using. At a high-level you must provide a memory buffer and a set
of import functions. The memory buffer is a contiguous, mutable byte-array that
allows you to pass data to the policy and receive output from the policy. The
import functions are dependencies of the compiled policies.

#### ABI Versions

Wasm modules built using OPA 0.27.0 onwards contain a global variable named
`opa_wasm_abi_version` that has a constant i32 value indicating the ABI version
this module requires. Described below you find ABI versions `1.x`.

There's another i32 constant exported, `opa_wasm_abi_minor_version`, used
to track backwards-compatible changes.

Using tools like `wasm-objdump` (`wasm-objdump -x policy.wasm`), the ABI
version can be found here:

```
Global[3]:
 - global[0] i32 mutable=1 - init i32=121904
 - global[1] i32 mutable=0 <opa_wasm_abi_version> - init i32=1
 - global[2] i32 mutable=0 <opa_wasm_abi_minor_version> - init i32=0
Export[19]:
[...]
 - global[1] -> "opa_wasm_abi_version"
 - global[2] -> "opa_wasm_abi_minor_version"
```

Note the `i32=1` of `global[1]`, exported by the name of `opa_wasm_abi_version`.

##### Version notes

ABI | Notes
--- | ---
1.0 | Start of ABI versioning.
1.1 | Adds export `memory`.
1.2 | Adds exported function `opa_eval`.

#### Exports

The primary exported functions for interacting with policy modules are listed below.
In the ABI column, you can find the ABI version with which the export was introduced.

| Function Signature | Description | ABI
| --- | --- | --- |
| <span class="opa-keep-it-together">`int32 eval(ctx_addr)`</span> | Evaluates the loaded policy with the provided evaluation context. The return value is reserved for future use. | 1.0 |
| <span class="opa-keep-it-together">`value_addr builtins(void)`</span> | Returns the address of a mapping of built-in function names to numeric identifiers that are required by the policy. | 1.0 |
| <span class="opa-keep-it-together">`value_addr entrypoints(void)`</span> | Returns the address of a mapping of entrypoints to numeric identifiers that can be selected when evaluating the policy. | 1.0 |
| <span class="opa-keep-it-together">`ctx_addr opa_eval_ctx_new(void)`</span> | Returns the address of a newly allocated evaluation context. | 1.0 |
| <span class="opa-keep-it-together">`void opa_eval_ctx_set_input(ctx_addr, value_addr)`</span> | Set the input value to use during evaluation. This must be called before each `eval()` call. If the input value is not set before evaluation, references to the `input` document result produce no results (i.e., they are undefined.) | 1.0 |
| <span class="opa-keep-it-together">`void opa_eval_ctx_set_data(ctx_addr, value_addr)`</span>  | Set the data value to use during evalutaion. This should be called before each `eval()` call. If the data value is not set before evalutaion, references to base `data` documents produce no results (i.e., they are undefined.) | 1.0 |
| <span class="opa-keep-it-together">`void opa_eval_ctx_set_entrypoint(ctx_addr, entrypoint_id)`</span>  | Set the entrypoint to evaluate. By default, entrypoint with id `0` is evaluated. | 1.0 |
| <span class="opa-keep-it-together">`value_addr opa_eval_ctx_get_result(ctx_addr)`</span> | Get the result set produced by the evaluation process. | 1.0 |
| <span class="opa-keep-it-together">`addr opa_malloc(int32 size)`</span> | Allocates size bytes in the shared memory and returns the starting address. | 1.0 |
| <span class="opa-keep-it-together">`void opa_free(addr)`</span> | Free a pointer. Calls `opa_abort` on error. | 1.0 |
| <span class="opa-keep-it-together">`value_addr opa_json_parse(str_addr, size)`</span> | Parses the JSON serialized value starting at str_addr of size bytes and returns the address of the parsed value. The parsed value may refer to a null, boolean, number, string, array, or object value. | 1.0 |
| <span class="opa-keep-it-together">`value_addr opa_value_parse(str_addr, size)`</span> | The same as `opa_json_parse` except Rego set literals are supported. | 1.0 |
| <span class="opa-keep-it-together">`str_addr opa_json_dump(value_addr)`</span> | Dumps the value referred to by `value_addr` to a null-terminated JSON serialized string and returns the address of the start of the string. Rego sets are serialized as JSON arrays. Non-string Rego object keys are serialized as strings. | 1.0 |
| <span class="opa-keep-it-together">`str_addr opa_value_dump(value_addr)`</span> | The same as `opa_json_dump` except Rego sets are serialized using the literal syntax and non-string Rego object keys are not serialized as strings. | 1.0 |
| <span class="opa-keep-it-together">`void opa_heap_ptr_set(addr)`</span> | Set the heap pointer for the next evaluation. | 1.0 |
| <span class="opa-keep-it-together">`addr opa_heap_ptr_get(void)`</span> | Get the current heap pointer. | 1.0 |
| <span class="opa-keep-it-together">`int32 opa_value_add_path(base_value_addr, path_value_addr, value_addr)`</span> | Add the value at the `value_addr` into the object referenced by `base_value_addr` at the given path. The `path_value_addr` must point to an array value with string keys (eg: `["a", "b", "c"]`). Existing values will be updated. On success the value at `value_addr` is no longer owned by the caller, it will be freed with the base value. The path must be freed by the caller after use (see `opa_free`). If an error occurs the base value will remain unchanged. Example: base object `{"a": {"b": 123}}`, path `["a", "x", "y"]`, and value `{"foo": "bar"}` will yield `{"a": {"b": 123, "x": {"y": {"foo": "bar"}}}}`. Returns an error code (see below). | 1.0 |
| <span class="opa-keep-it-together">`int32 opa_value_remove_path(base_value_addr, path_value_addr)`</span> | Remove the value from the object referenced by `base_value_addr` at the given path. Values removed will be freed. The path must be freed by the caller after use (see `opa_free`). The `path_value_addr` must point to an array value with string keys (eg: `["a", "b", "c"]`). Returns an error code (see below). | 1.0 |
| <span class="opa-keep-it-together">`str_addr opa_eval(_ addr, entrypoint_id int32, data value_addr, input str_addr, input_len int32, heap_ptr addr, format int32)`</span> | One-off policy evaluation method. Its arguments are everything needed to evaluate: entrypoint, address of data in memory, address and length of input JSON string in memory, heap address to use, and the output format (`0` is JSON, `1` is "value", i.e. serialized Rego values). The first argument is reserved for future use and must be `0`. Returns the address to the serialised result value. | 1.2 |

The addresses passed and returned by the policy modules are 32-bit integer
offsets into the shared memory region. The `value_addr` parameters and return
values refer to OPA value data structures: `null`, `boolean`, `number`,
`string`, `array`, `object`, and `set`.

__Error codes:__

OPA Wasm Error codes are int32 values defined as:

| Value | Name | Description |
|-------|------|-------------|
| 0 | OPA_ERR_OK | No error. |
| 1 | OPA_ERR_INTERNAL | Unrecoverable internal error. |
| 2 | OPA_ERR_INVALID_TYPE | Invalid value type was encountered. |
| 3 | OPA_ERR_INVALID_PATH | Invalid object path reference. |

#### Imports

Policy modules require the following function imports at instantiation-time:

| Namespace | Name | Params | Result | Description |
| --- | --- | --- | --- | --- |
| `env` | `opa_abort` | `(addr)` | `void` | Called if an internal error occurs. The `addr` refers to a null-terminated string in the shared memory buffer. |
| `env` | `opa_println` | `(addr)` | `void` | Called to emit a message from the policy evaluation. The `addr` refers to a null-terminated string in the shared memory buffer. |
| `env` | `opa_builtin0` | <span class="opa-keep-it-together">`(builtin_id, ctx)`</span> | `addr` | Called to dispatch the built-in function identified by the `builtin_id`. The `ctx` parameter reserved for future use. The result `addr` must refer to a value in the shared-memory buffer. The function accepts 0 arguments. |
| `env` | `opa_builtin1` | <span class="opa-keep-it-together">`(builtin_id, ctx, _1)`</span> | `addr` | Same as previous except the function accepts 1 argument. |
| `env` | `opa_builtin2` | <span class="opa-keep-it-together">`(builtin_id, ctx, _1, _2)`</span> | `addr` | Same as previous except the function accepts 2 arguments. |
| `env` | `opa_builtin3` | <span class="opa-keep-it-together">`(builtin_id, ctx, _1, _2, _3)`</span> | `addr` | Same as previous except the function accepts 3 arguments. |
| `env` | `opa_builtin4` | <span class="opa-keep-it-together">`(builtin_id, ctx, _1, _2, _3, _4)`</span> | `addr` | Same as previous except the function accepts 4 arguments. |

The policy module also requires a shared memory buffer named `env.memory`.

#### Memory Buffer

A shared memory buffer must be provided as an import for the policy module with
the name `env.memory`. The buffer must be large enough to accommodate the input,
provided data, and result of evaluation.

#### Built-in Functions

After instantiating the policy module, call the exported `builtins` function to
receive a mapping of built-in functions required during evaluation. The result
maps required built-in function names to the identifiers supplied to the
built-in function callbacks (e.g., `opa_builtin0`, `opa_builtin1`, etc.)

For example:

```javascript
const memory = new WebAssembly.Memory({ initial: 5 });
const policy_module = await WebAssembly.instantiate(byte_buffer, /* import object */);
const addr = policy_module.instance.exports.builtins();
const str_addr =  policy_module.instance.exports.opa_json_dump(addr);
const builtin_map = deserialize_null_terminated_JSON_string(memory, str_addr);
```

The built-in function mapping will contain all of the built-in functions that
may be required during evaluation. For example, the following query refers to
the `http.send` built-in function which is not included in the policy module:

```live:builtin:module:read_only
result := http.send({"method": "get", "url": "https://example.com/api/lookup/12345"})
```

If this query was compiled to Wasm the built-in map would contain a single
element:

```json
{
    "http.send": 0
}
```

When the evaluation runs, the `opa_builtin1` callback would invoked with
`builtin_id` set to `0`.

#### Evaluation

Once instantiated, the policy module is ready to be evaluated. Use the
`opa_eval_ctx_new` exported function to create an evaluation context. Use the
`opa_eval_ctx_set_input` and `opa_eval_ctx_set_data` exported functions to specify
the values of the `input` and base `data` documents to use during evaluation.

To evaluate, call to the exported `eval` function with the eval context address
as the only parameter.

#### Input

The (optional) `input` document for a policy can be provided by loading a JSON
 string into the shared memory buffer. Use the `opa_malloc` exported function to
 allocate a buffer the size of the JSON string and copy the contents in at the
 returned address. After the raw string is loaded into memory you will need to
 call the `opa_json_parse` exported method to get an address to the parsed input
 document for use in evaluations. Set the address via the
 `opa_eval_ctx_set_input` exported functoin supplying the evaluation context
 address and parsed input document address.

#### External Data

External data can be loaded for use in evaluation. Similar to the `input` this
is done by loading a JSON string into the shared memory buffer. Use `opa_malloc`
and `opa_json_parse` followed by `opa_eval_ctx_set_data` to set the address on
the evaluation context.

Data can be updated by using the `opa_value_add_path` and `opa_value_remove_path`
and providing the same value address as the base. Similarly, use `opa_malloc` and
`opa_json_parse` for the updated value and creating the path.

After loading the external data use the `opa_heap_ptr_get` exported method to save
the current point in the heap before evaluation. After evaluation this should be
reset by calling `opa_heap_ptr_set` to ensure that evaluation restarts back at the
saved data and re-uses heap space. This is particularly important if re-evaluating many
times with the same data.

#### Entrypoints

The compiled policy may have one or more entrypoints. If no entrypoint is set
on the evaluation context the default entrypoint (`0`) will be evaluated. SDKs
can call `entrypoints()` after instantiating the module to retrieve the
entrypoint name to entrypoint identifier mapping. SDKs can set the entrypoint to
evaluate by calling `opa_eval_ctx_set_entrypoint` on the evaluation context. If
an invalid entrypoint identifier is passed, the `eval` function will invoke `opa_abort`.

#### Results

After evaluation results can be retrieved via the exported
`opa_eval_ctx_get_result` function. Pass in the evaluation context address. The
return value is an address in the shared memory buffer to the structured result.
To access the JSON result use the `opa_json_dump` exported function to retrieve
a pointer in shared memory to a null terminated JSON string.

The result of evaluation is the set variable bindings that satisfy the
expressions in the query. For example, the query `x = 1; y = 2; y > x` would
produce the following result set:

```json
[
    {
        "x": 1,
        "y": 2
    }
]
```

Sets are represented as JSON arrays.
