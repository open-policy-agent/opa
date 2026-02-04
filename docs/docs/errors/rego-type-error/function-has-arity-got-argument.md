---
sidebar_label: function name has arity n, got m argument(s)
image: /img/opa-errors.png
---

# rego_type_error: function `{name}` has arity n, got m argument(s)

[Arity](https://en.wikipedia.org/wiki/Arity) is a term used to describe the number of arguments a function takes.
This error happens when a user-defined function is called with an unexpected number of arguments - the wrong _arity_.

:::note
This message is only shown for user defined functions, not for Rego's built-in functions. For errors
containing `arity mismatch` see [this section](./arity-mismatch).
:::

| Stage         | Category          | Message                                  |
| ------------- | ----------------- | ---------------------------------------- |
| `compilation` | `rego_type_error` | `{name}` has arity n, got m argument(s)` |

## Examples

In the following example, the user-defined function `is_authorized` function is called with one argument,
but it takes two:

```rego
package policy

import future.keywords.if
import future.keywords.in

default is_authorized(_, _) := false

is_authorized(user_id, resource) if {
  # implementation details
}

default allow := false

allow := is_authorized(input)
```

When compiled, this will result in the following error:

```txt
1 error occurred: policy.rego:16: rego_type_error: function data.policy.is_authorized has arity 2, got 1 argument
```

## How To Fix It

In order to find the function that's being called with the wrong arity, you first need to find the line number
in the error message - `16` in the example above. On that line, we need to update the function call to pass the
correct number of arguments. The example above might be fixed like so:

```rego
package policy

import future.keywords.if
import future.keywords.in

default is_authorized(_, _) := false

is_authorized(user_id, resource) if {
  # implementation details
}

default allow := false

allow := is_authorized(input.user_id, input.resource) # pass two arguments
```
