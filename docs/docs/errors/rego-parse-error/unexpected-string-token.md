---
sidebar_label: "unexpected string token"
image: /img/opa-errors.png
---

# rego_parse_error: unexpected string token

This parser error is raised when the Rego parser encounters an unexpected string token. This is typically
caused by a missing keyword or operator.

| Stage     | Category           | Message                   |
| --------- | ------------------ | ------------------------- |
| `parsing` | `rego_parse_error` | `unexpected string token` |

## Examples

A simple example of a policy that contains this error follows, note the missing `==` after `input.roles`:

```rego
package policy

import rego.v1

allow if {
  input.role "admin"
}
```

The code above will raise the following error:

```txt
1 error occurred: policy.rego:6: rego_parse_error: unexpected string token: expected \n or ; or }
  input.roles "admin"
              ^
```

## How To Fix It

While the fix for this error depends on other elements on the line of Rego in question,
the error message will always point to the location of the error to get you started.

Typically, the fix it to determine the intended functionality of the line in question and
add a missing keyword or operator. Often `==`/`!=`, or `in`.
