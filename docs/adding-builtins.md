# Adding Built-in Functions

Rego includes a number of built-in functions ("built-ins") for performing
standard operations like string manipulation, regular expression matching, and
computing aggregates.

This document describes the process for adding a new built-in to Rego and OPA.

## Steps

### Add built-in description to `github.com/open-policy-agent/opa/ast` package.

The `ast` package contains a registry (i.e., an array of structs) that
enumerates the built-ins included in Rego. When adding a new built-in, you must
update the registry to include your built-in. Otherwise, the compiler will
complain when it encounters your built-in in policy modules.

1. Define the built-in struct. See the `ToNumber` definition in
   [ast/builtins.go](../ast/builtins.go) for an example.

1. Add the struct to the `DefaultBuiltins` array in
   [ast/builtins.go](../ast/builtins.go).

Once both changes have been made, you should be able to compile policy modules
that contain your built-in.

### Add built-in implementation to `github.com/open-policy-agent/opa/topdown` package.

The `topdown` package contains the built-in function implementations that are
called during query evaluation.

1. Add a function that implements the `BuiltinFunc` type to the `topdown`
   package . See `evalToNumber` in [topdown/casts.go](../topdown/casts.go) for
   an example.

1. Add the new function to the `defaultBuiltinFuncs` map in
[topdown/builtins.go](../topdown/builtins.go) map.

1. Add tests for the new built-in. See
[topdown/topdown_test.go](../topdown/topdown_test.go) for examples of other
built-in tests.

### Add an entry to the language reference document.

Update the [Language Reference section on built-in
functions](../site/documentation/references/language/index.md#built-in-functions).
This is the authoritative specification for your built-in.

### Add a REPL example to the language introduction document.

Update the [How Do I Write Policies? section on built-in
functions](../site/documentation/how-do-i-write-policies/index.md#-built-in-functions)
document as well.