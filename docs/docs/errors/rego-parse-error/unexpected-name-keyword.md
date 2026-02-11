---
sidebar_label: "unexpected {name} keyword"
image: /img/opa-errors.png
---

# `rego_parse_error`: unexpected `{name}` keyword

This parser error is raised when an unexpected keyword is encountered. Keywords are usually misplaced
due to a missing operator or a misconception about how the keyword is used in Rego.

| Stage     | Category           | Message                     |
| --------- | ------------------ | --------------------------- |
| `parsing` | `rego_parse_error` | `unexpected {name} keyword` |

## Examples

A simple example of a policy that contains this error follows, note the missing `==` after `input.admin`:

```rego
package policy

import rego.v1

allow if {
    input.admin true
}
```

The code above will raise the following error showing the location of the unexpected keyword:

```txt
1 error occurred: policy.rego:6: rego_parse_error: unexpected true keyword: expected \n or ; or }
    input.admin true
                ^
```

Another example of the same error can be seen in this code, where a policy has been written
with a misunderstanding of how the `else` keyword is used:

```rego
package policy

import rego.v1

allow if {
    input.admin else "admin" in input.roles
}
```

## How To Fix It

The first step is usually identify if the issue is a syntax issue that can be easily fixed (like in the first example),
or if the issue is a misunderstanding. The former is usually fixed by inspecting for invalid syntax at the location
identified by the error message. The latter is more tricky as the code will need to be rewritten to match the intended
functionality.

A common cause for such misunderstandings is expressing or in Rego, if you think this case might apply, there's a great
[blog post](https://www.styra.com/blog/how-to-express-or-in-rego/) on this topic here you might want to check out.

Failing that, the best place to start is to review the OPA documentation on Rego, see this page for an
[explanation of Rego's keywords](https://www.openpolicyagent.org/docs/policy-language).
