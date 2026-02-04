---
sidebar_label: "unexpected { token: expected \n or ; or }"
image: /img/opa-errors.png
---

# rego_parse_error: unexpected `{` token: expected `\n` or ; or `}`

This error is raised when the Rego parser encounters an unexpected `{` token. Like many
language, Rego used `{` and `}` to denote blocks of code. This error is raised when
the parser encounters a `{` token where it was not expecting one, or where it was unable
to find the matching `}` token.

| Stage     | Category           | Message                                     |
| --------- | ------------------ | ------------------------------------------- |
| `parsing` | `rego_parse_error` | `unexpected { token: expected \n or ; or }` |

## Examples

A simple example of a policy that contains this error follows, note the extra `{` after `input.roles == {}`:

```rego
package policy

import rego.v1

deny if {
  input.roles == {}{
  input.user != "admin"
}
```

The code above will raise the following error, helpfully showing the location in question:

```txt
1 error occurred: policy.rego:6: rego_parse_error: unexpected { token: expected \n or ; or }
  input.roles == {}{
                   ^
```

## How To Fix It

While the fix for this error depends on other elements on the line of Rego in question,
the error message will always point to the location of the error to get you started.

Typically, the fix is to unmatched `{}` characters. Many text editors will highlight matching
brackets to help you find the issue.
