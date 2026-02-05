---
sidebar_label: assigned var {name} unused
image: /img/opa-errors.png
---

# `rego_compile_error`: assigned var `{name}` unused

This error is caused by a variable being assigned a value, but never used. This is often the result of a typo or
mis-named variable being assigned.

:::note
This error is only shown when running OPA with the `--strict` (or `-S`) flag.
:::

| Stage     | Category             | Message                                          |
| --------- | -------------------- | ------------------------------------------------ |
| `parsing` | `rego_compile_error` | `rego_compile_error: assigned var {name} unused` |

## Examples

This a simple example of this error is when a variable is assigned a value, but never used:

```rego
package policy

import future.keywords.if

allow if {
    user := input.user
    input.user == "admin"
}
```

When compiled, this will result in the following error because the variable `user` is never used:

```txt
1 error occurred: policy.rego:6: rego_compile_error: assigned var user unused
```

Another common reason for this message is when making a typo or mis-naming a variable. For example:

```rego
package policy

import future.keywords.contains
import future.keywords.if

deny contains message if {
    input.user != "admin"

    msg := "user is not admin"
}
```

Here, we can see the intent, the `message` should be set to `user is not admin` if the user is not an admin. However,
the variable `msg` is assigned instead.

## How To Fix It

Based on the examples above, we can see that there are two main ways to fix this error:

- Remove the assignments of unused variables. In the first example, we can simply remove the line `user := input.user`.
  The other option is of course to use the variable that was assigned. Often orphaned variables like this are the result
  a refactoring and can be safely removed if they aren't making the rule more readable.
- Check for typos and mis-named variables. As well as normal typos, it's also easy to make mistakes and swap variables
  for alternative names. For example, sometimes `msg` instead of `message` or `user` instead of `username`.

While it's also possible to ignore this error by unsetting the `--strict` (`-S`) flag, it's highly recommended to fix
the underlying issue as outlined in the points above.
