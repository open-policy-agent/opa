---
title: Adding Built-in Functions
kind: contrib
weight: 5
---

[Built-in Functions](../policy-reference/#built-in-functions)
can be added inside the `topdown` package.

Built-in functions may be upstreamed if they are generally useful and provide functionality that would be
impractical to implement natively in Rego (e.g., CIDR arithmetic). Implementations should avoid thirdparty
dependencies. If absolutely necessary, consider importing the code manually into the `internal` package.

{{< info >}}
Read more about extending OPA with custom built-in functions in go [here](../extensions#custom-built-in-functions-in-go).
{{< /info >}}

Adding a new built-in function involves the following steps:

1. [Declare and register](#declare-and-register) the function
2. [Implement](#implement) the function
3. [Test](#test) the function
4. [Document](#document) the function
5. [Add a capability](#add-a-capability) for the function

## Example

The following example adds a simple built-in function, `repeat(string, int)`, that returns a given string repeated a given number of times.

### Declare and Register

In `ast/builtins.go`, we declare the structure of our built-in function with a `Builtin` struct instance:

```go
var Repeat = &Builtin{
	Name:        "repeat", // The name of the function
	Description: "Returns, as a string, the given string repeated the given number of times.",
	Decl: types.NewFunction(
		types.Args( // The built-in takes two arguments, where ..
			types.Named("str", types.S).Description("string to repeat"),            // named string argument
			types.Named("count", types.N).Description("how often to repeat `str`"), // named number argument
		),
		types.Named("output", types.S).Description("the repetitions"), // The return type is a string.
	),
  Categories: category("strings"), // the category the built-in belongs to
}
```

To register the new built-in function, we locate the `DefaultBuiltins` array in `ast/builtins.go`, and add the `Builtin` instance to it:

```go
var DefaultBuiltins = [...]*Builtin{
    ...
    Repeat,
    ...
}
```

### Implement

In the `topdown` package, we locate a suitable source file for our new built-in function, or add a new file, as appropriate.

In this example, we introduce a new source file, `topdown/repeat.go`:

```go
package topdown

import (
    "fmt"
    "strings"

    "github.com/open-policy-agent/opa/ast"
    "github.com/open-policy-agent/opa/topdown/builtins"
)

// implements topdown.BuiltinFunc
func builtinRepeat(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
    // Get the first argument as a string, returning an error if it's not the correct type.
    str, err := builtins.StringOperand(operands[0].Value, 1)
    if err != nil {
        return err
    }

    // Get the second argument as an int, returning an error if it's not the correct type or not a positive value.
    count, err := builtins.IntOperand(operands[1].Value, 2)
    if err != nil {
        return err
    } else if count < 0 {
        // Defensive check, strings.Repeat(...) will panic for count<0
        return fmt.Errorf("count must be a positive integer")
    }

    // Return a string by invoking the given iterator function
    return iter(ast.StringTerm(strings.Repeat(string(str), count)))
}

func init() {
    RegisterBuiltinFunc(ast.Repeat.Name, builtinRepeat)
}
```

In the above code, `builtinRepeat` implements the `topdown.BuiltinFunc` function type.
The call to `RegisterBuiltinFunc(...)` in `init()` adds the built-in function to the evaluation engine; binding the implementation to `ast.Repeat` that was registered in [an earlier step](#declare-and-register).

### Test

All built-in function implementations must include a test suite.
Test cases for built-in functions are written in YAML and located under `test/cases/testdata`.

We create two new test cases (one positive, expecting a string output; and one negative, expecting an error) for our built-in function:

```yaml
cases:
  - note: repeat/positive
    query: data.test.p = x
    modules:
      - |
        package test

        p := repeated {
          repeated := repeat(input.str, input.count)
        }
    input: {"str": "Foo", "count": 3}
    want_result:
      - x: FooFooFoo
  - note: repeat/negative
    query: data.test.p = x
    modules:
      - |
        package test

        p := repeated {
          repeated := repeat(input.str, input.count)
        }
    input: { "str": "Foo", "count": -3 }
    strict_error: true
    want_error_code: eval_builtin_error
    want_error: 'repeat: count must be a positive integer'
```

The above test cases can be run separate from all other tests through: `go test ./topdown -v -run 'TestRego/repeat'`

See [test/cases/testdata/helloworld](https://github.com/open-policy-agent/opa/blob/main/test/cases/testdata/helloworld)
for a more detailed example of how to implement tests for your built-in functions.

{{< info >}}
Note: We can manually test our new built-in function by [building](../contrib-development#getting-started)
and running the `eval` command. E.g.: `$./opa_<OS>_<ARCH> eval 'repeat("Foo", 3)'`
{{< /info >}}

### Document

All built-in functions will automatically be documented in `docs/content/policy-reference.md` under an appropriate subsection.

For this example, we'll get an entry for our new function under the `Strings` section.

### Add a capability

{{< info >}}
Read more about extending the default capabilities list for built-ins [here](../deployments/#built-ins).
{{< /info >}}

One of the security features of OPA is [capabilities](../deployments/#capabilities) checks on policies, allowing users to restrict which built-in functions will be available to policies at runtime.
To ensure that our new `repeat` function will be available to callers, we'll need to add it to the `capabilities.json` file at the root of the repo.
We can have this entry auto-generated for us by running `make generate`.

After running `make generate` we should see a new JSON object entry in the list under the `"builtins"` key:

```json
...
{
  "name": "repeat",
  "decl": {
    "args": [
      {
        "type": "string"
        "type": "number"
      }
    ],
    "result": {
      "type": "string"
    },
    "type": "function"
  }
},
...
```
