---
sidebar_label: "unexpected } token"
image: /img/opa-errors.png
---

# rego_parse_error: unexpected `}` token

This error is raised when the Rego parser encounters an unexpected `}` token. Like many
language, Rego used `{` and `}` to denote blocks of code. This error is raised when
the parser encounters a `}` token where it was not expecting one, or where it was unable
to find the matching `{` token.

| Stage     | Category           | Message              |
| --------- | ------------------ | -------------------- |
| `parsing` | `rego_parse_error` | `unexpected } token` |

## Examples

A simple example of a policy that contains this error follows, note the extra `}` after `input.roles == {}`:

```rego
package policy

import rego.v1

deny if {
    input.roles == {}}
    input.user != "admin"
}
```

The code above will raise the following error:

```txt
1 error occurred: policy.rego:8: rego_parse_error: unexpected } token
    }
    ^
```

The parser error is pointing to the later `}` token, which in this case is not the issue.

## How To Fix It

While the fix for this error depends on other elements on the line of Rego in question,
the error message will always point to the location of the error to get you started so
you at least know which block of code to look at.

Typically, the fix is to unmatched `{}` characters. Many text editors will highlight matching
brackets to help you find the issue.
