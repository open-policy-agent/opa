---
sidebar_label: arity mismatch
image: /img/opa-errors.png
---

# rego_type_error: `{built-in name}` arity mismatch

[Arity](https://en.wikipedia.org/wiki/Arity) is a term used to describe the number of arguments a function takes.
This error happens when one of Rego's
[built-in functions](https://www.openpolicyagent.org/docs/policy-reference/#built-in-functions)
is called with an unexpected number of arguments - the wrong _arity_.

:::note
This message is only shown for Rego's built-in functions, not for user-defined functions. For
errors containing `has arity n, got m argument(s)` see [this section](./function-has-arity-got-argument).
:::

| Stage         | Category          | Message                          |
| ------------- | ----------------- | -------------------------------- |
| `compilation` | `rego_type_error` | `{built-in name} arity mismatch` |

## Examples

In the following example, the
[`split`](https://www.openpolicyagent.org/docs/policy-reference/#builtin-aggregates-split)
function is called with two arguments, but it only takes one:

```rego
package policy

import future.keywords.if
import future.keywords.in

allow if "admin" in split("admin,member")
```

When compiled, this will result in the following error:

```txt
1 error occurred: policy.rego:6: rego_type_error: split: arity mismatch
	have: (string, ???)
	want: (x: string, delimiter: string)
```

## How To Fix It

In order to find the built-in function that is causing this error, you can find the line number in the error message.
E.g. in the example above, the error message says:

```txt
policy.rego:6: rego_type_error: split: arity mismatch
     ^      ^                     ^
   file  line number       built-in function
```

Here, the line number is `6` and the built-in function is `split`.

Once you have found the built-in function, you can look up its expected arguments and arity in the
['Policy Reference' documentation](https://www.openpolicyagent.org/docs/policy-reference/#built-in-functions).
