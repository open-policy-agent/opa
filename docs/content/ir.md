---
title: Intermediate Representation (IR)
kind: misc
weight: 1
---

# Overview

OPA can compile policy queries into planned evaluation paths suitable for
further compilation or interpretation. This document explains the structure and
semantics of the intermediate representation (IR) used to represent these
planned evaluation paths. Read this document if you want to write a compiler or
interpreter for Rego.

# Structure

This section explains the structure of policies compiled into the IR.

## Policy

The root object emitted by the compiler is a `Policy` and contains the following
top-level keys:

* `static` is an object containing static data used by the compiled plans and
  functions.
* `plans` is an object containing entrypoints to compiled evaluation paths.
* `funcs` is an object containing functions supporting the compiled evaluation
  paths.

## Static

The `Static` object contains static data required by the plans and functions.
The static object also contains metadata that does not affect the semantics of
the policy. The static object contains the following top-level keys:

* `strings` is an array of string constants referenced by compiled statements in
  the plans and functions.
* `builtin_funcs` is an array of function declarations representing built-in
  functions required by the compiled statements.
* `files` is used for debugging purposes only. It is an array of filenames that
  were used during compilation.

### Strings

The `Strings` array is a collection of string objects referenced by compiled
statements in the policy. Strings are referenced by their index in the
collection. Each string object contains the following fields:

* `value` is the string constant value. The string may be any valid JSON string.

### Built-in Functions

The `Built-in Functions` array is a collection of built-in function
declarations. Each declaration represents a function that must be provided by
the environment where the policy is eventually executed. Each built-in function
contains the following fields:

* `name` is the name of the function that must be provided.
* `decl` is the type definition of the function.

### Files

The `Files` array is a collection of static strings representing names of source
files used during compilation. Filenames are referred to by their index in the
files array.

## Plans

The `Plans` object contains a collection of planned evaluation paths
representing entrypoints to the policy. When users compile policies they supply
the queries to expose as entrypoints. Each plan contains the following fields:

* `name` is the entrypoint identifier, typically set to the path of the policy
  decision (e.g., `authz/allow`).
* `blocks` is a collection of [`Block`](#blocks) objects representing the
  compiled statements that define the entrypoint.

## Functions

The `Functions` object contains a collection of function definitions that
represent functions supporting the plans. Functions can be invoked by name
inside of plans and other functions. Each function contains the following
fields:

* `name` is the function identifier referenced by call statements.
* `path` is the function identifier referenced by dynamic call statements.
* `params` is an ordered list of local variable identifiers representing
  function parameters. The parameters can be referenced inside of the blocks
  that define the function.
* `return` is the local variable containing the return value of the function.
* `blocks` is collection of [`Block`](#blocks) objects representing the compiled
  statements that define the function.

## Blocks

The `Block` object contains a sequence of [Statements](#statements) that must be
executed in order until a statement terminating block execution is encountered
or the end of the block is reached. Each block contains the following fields:

* `stmts` is an array of `Statement` objects.

## Statements

The `Statement` object represents an operation performed by the policy (e.g.,
function invocation, lookup, iteration, comparison, etc.) The structure is
specific to each statement type but every statement contains the following
fields:

* `type` is a string value that identifies the type of the statement.
* `stmt` is an object containing statement-specific fields.
* `file` is the index of source filename where this statement originated.
* `row` is the row in the source file where this statement originated.
* `col` is the column in the source file where this statement originated.

See the [Statement Definitions](#statement-definitions) section for an
explanation of the supported statement types.

# Execution

This section explains the execution model for compiled policies.

## Plan Execution

Compiled policies consist of one or more plans. Any plan can be invoked by name.
If no name is supplied, the first plan in the policy should be executed. Plans
consist of one or more [Blocks](#blocks) that are executed in-order. Statements
inside the blocks of a plan have implicit access to two local variables
representing the `input` and `data` documents (`0` and `1` respectively.) The
final statement in every block inside of a plan is a `ResultSetAddStmt`
statement that adds an object to an implicit result set. The object contains the
key-value bindings representing the values of variables in the original query.
If no `ResultSetAddStmt` statements are executed, the implicit result set is
empty.

## Function Execution

Compiled policies may contain zero or more functions. Any function can be
invoked by name via the `CallStmt` statement or dynamically via the
`CallDynamicStmt` statement. All functions are defined with two or more
positional arguments. The first positional argument is a local variable
representing the `input` document. The second positional argument is a local
variable representing the `data` document. Function execution terminates when a
`ReturnLocalStmt` statement is encountered. All functions include a final block
that includes a `ReturnLocalStmt`.

## Block Execution

Blocks are sequences of statements that are executed in order. Statements can be
executed if all of the input parameters are defined. If any input parameter is
undefined then the statement is undefined. The [Statement
Definitions](#statement-definitions) section below indicates when a statement
may be undefined. When a statement is undefined execution breaks to the end of
the current block and resumes execution at the statement immediately following
the block (which may be the beginning of another block.) When a statement is
defined, all output parameters are defined. Execution halts if a statement
raises an exception.

# Statement Definitions

This section defines the statements that can be contained in plans and functions
and explains the input and output parameters that each statement accepts. The
set of valid parameter types are:

* `local` is a 32-bit integer representing a local variable.
* `int32` is a 32-bit integer.
* `int64` is a 64-bit integer.
* `uint32` is a 32-bit unsigned integer.
* `string` is an arbitrary-length unicode string.
* `array[...]` represents a sequence of `...` values.

In addition, parameters may be of type `operand`. The `operand` type represents
a tagged union that can refer to a local variable, boolean constant, or string
constant index:

```
{
    "type": "local" | "bool" | "string_index"
    "value": number | boolean | number
}
```

Local variables refer to values. The value types are any JSON value (i.e., `null`,
`true`, `false`, `number`, `string`, `array`, and `object`) as well as sets
(which are unordered value collections.)

## `ArrayAppendStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`array` | `input` | `local` | The array to append a value to.
`value` | `input` | `operand` | The value to append to the array.

## `AssignIntStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`value` | `input` | `int64` | The integer value to assign to the target.
`target` | `output` | `local` | The local variable to assign the integer to.

## `AssignVarOnceStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to assign to the target.
`target` | `output` | `local` | The local variable to assign the operand to.

{{< danger >}}
This statement raises an exception if the `target` operand is already assigned.
{{< /danger >}}

## `AssignVarStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to assign to the target.
`target` | `output` | `local` | The local variable to assign the operand to.

## `BlockStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`blocks` | `input` | [Blocks](#blocks) | The nested blocks to execute.

## `BreakStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`index` | `input` | `uint32` | The index of the block to jump out of starting with zero representing the current block and incrementing by one for each outer block.

## `CallDynamicStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`path` | `input` | `array[operand]` | The path of the function to invoke.
`args` | `input` | `array[local]` | The positional arguments to pass to the function.
`result` | `output` | `local` | The local variable to assign the function return value to.

## `CallStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`func` | `input` | `string` | The name of the function to invoke.
`args` | `input` | `array[local]` | The positional arguments to pass to the function.
`result` | `output` | `local` | The local variable to assign the function return value to.

## `DotStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to perform a lookup operation on.
`key` | `input` | `operand` | The key to lookup in the source.
`target` | `output` | `local` | The local variable to assign the result to.

This statement is **undefined** if the `key` does not exist in the `source` value.

## `EqualStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`a` | `input` | `operand` | The first value to compare.
`b` | `input` | `operand` | The second value to compare.

This statement is **undefined** if `a` does not equal `b`.

## `IsArrayStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to check.

This statement is **undefined** if `source` is not an array.

## `IsDefinedStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to check.

This statement is **undefined** if `source` is undefined.

## `IsObjectStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to check.

This statement is **undefined** if `source` is not an object.

## `IsUndefinedStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to check.

This statement is **undefined** if `source` is not undefined.

## `LenStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `operand` | The value to compute the length for.
`target` | `output` | `local` | The local variable to assign the length to.

## `MakeArrayStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`capacity` | `input` | `int32` | The initial size of the array to pre-allocate.
`target` | `output` | `local` | The local variable to assign the array value to.

## `MakeNullStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`target` | `output` | `local` | The local variable to assign the null value to.

## `MakeNumberIntStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`value` | `input` | `int64` | The integer value to initialize the target with.
`target` | `output` | `local` | The local variable to assign the number to.

## `MakeNumberRefStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`index` | `input` | `int32` | The index of the string constant to construct the number with.
`target` | `output` | `local` | The local variable to assign the number to.

## `MakeObjectStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`target` | `output` | `local` | The local variable to assign the object to.

## `MakeSetStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`target` | `output` | `local` | The local variable to assign the set to.

## `NopStmt`

This statement is only used for debugging purposes.

## `NotEqualStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`a` | `input` | `operand` | The first value to compare.
`b` | `input` | `operand` | The second value to compare.

This statement is **undefined** if `a` is equal to `b`.

## `NotStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`block` | `input` | [Block](#blocks) | The negated statement to execute.

This statement is **undefined** if the contained block is not undefined.

## `ObjectInsertOnceStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`key` | `input` | `operand` | The key to insert into the object.
`value` | `input` | `operand` | The value to insert into the object.
`object` | `input` | `local` | The object to insert the key-value pair into.

{{< danger >}}
This statement raises an exception if the `object` contains an existing `key` with a different `value`.
{{< /danger >}}

## `ObjectInsertStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`key` | `input` | `operand` | The key to insert into the object.
`value` | `input` | `operand` | The value to insert into the object.
`object` | `input` | `local` | The object to insert the key-value pair into.

## `ObjectMergeStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`a` | `input` | `local` | The object to merge into.
`b` | `input` | `local` | The object to merge from.
`target` | `output` | `local` | The local variable to assign the merged object to.

## `ResetLocalStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`target` | `output` | `local` | The local variable to reset.

## `ResultSetAddStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`value` | `input` | `local` | The value to add to the result set.

## `ReturnLocalStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `local` | The value to return from the function.

## `ScanStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`source` | `input` | `local` | The value to scan.
`key` | `output` | `local` | The local variable to assign keys to before executing the nested block.
`value` | `output` | `local` | The local variable to assign values to before executing the nested block.
`block` | `input` | [Block](#blocks) | The nested block to execute repeatedly for each element in the collection.

This statement is **undefined** if `source` is a scalar value or empty collection.

## `SetAddStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`value` | `input` | `operand` | The value to insert into the set.
`set` | `input` | `local` | The set to insert the value into.

## `WithStmt`

Parameter | Input/Output | Type | Description
--- | --- | --- | ---
`local` | `input` | `local` | The value to mutate in the context of the nested block.
`path` | `input` | `array[int32]` | The path of the nested document to replace with the `value` represented as an array of string constant indices.
`value` | `input` | `operand` | The value to upsert.
`block` | `input` | [Block](#blocks) | The nested block to execute in the context of the mutation.

# Test Suite

The OPA repository contains a [test
suite](https://github.com/open-policy-agent/opa/tree/main/test/cases/testdata)
that is used internally to validate both the Go interpreter and the Wasm
compiler. If you are implementing your own compiler or interpreter we highly
recommend integrating the test suite into your own development environment so
that your implementation can be verified to conform with OPA's.

The test suite consists of a set of YAML files that each contain a set of test
cases. Each test cases specifies a query, set of modules, data values, and
expected outputs or expected error conditions.

To get started with the test suite, see the [Hello
World](https://github.com/open-policy-agent/opa/blob/main/test/cases/testdata/helloworld/test-helloworld-1.yaml)
example.

The following examples show how the test suite is used internally:

* [`github.com/open-policy-agent/opa/topdown#TestRego`](https://github.com/open-policy-agent/opa/blob/main/topdown/exported_test.go)
* [`github.com/open-policy-agent/opa/internal/wasm/sdk/test/e2e/external_test`](https://github.com/open-policy-agent/opa/blob/main/internal/wasm/sdk/test/e2e/external_test.go)
