---
sidebar_label: "unexpected identifier token: expected \n or ; or }"
image: /img/opa-errors.png
---

# `rego_parse_error`: unexpected identifier token: expected \n or ; or }

This error is raised when the Rego parser encounters an unexpected identifier
token.

An identifier token represents a name assigned to variables, rules, and
references in the policy. In parsing, the identifier tokens are different from
other tokens like keywords.

This error is typically caused by misplaced whitespace or by poorly formatted
code.

| Stage     | Category           | Message                                              |
| --------- | ------------------ | ---------------------------------------------------- |
| `parsing` | `rego_parse_error` | `unexpected identifier token: expected \n or ; or }` |

## Examples

A simple example of a policy that contains this error follows, note the missing
`.` between `input` and `roles`:

```rego
package policy

import rego.v1

allow if {
// highlight-next-line
    "admin" in input roles
}
```

The code above will raise the following error, helpfully showing the location in
question:

```txt
1 error occurred: policy.rego:6: rego_parse_error: unexpected identifier token: expected \n or ; or }
    "admin" in input roles
                     ^
```

Another case when this error can occur is when a statement has been added to the
end of another on the same line, this happens a lot when copying and pasting. In
the example below, the `endswith` email check is meant to be on the next line:

```rego
package policy

import rego.v1

allow if {
// highlight-next-line
    "admin" in input.roles endswith(input.email, "@example.com")
}
```

This should be written like this, with the statements on separate lines:

```rego
package policy

import rego.v1

allow if {
    "admin" in input.roles
// highlight-next-line
    endswith(input.email, "@example.com")
}
```

## How To Fix It

While the fix for this error depends on other elements on the line of Rego in
question, the error message will always point to the location of the error to
get you started.

Typically, the way to resolve this to find and correct misplaced whitespace
using the location in the error message.

This can be tricky in larger files and often happens when moving code between
files. We recommend migrating functions and rules incrementally to reduce the
risk of this happening.
