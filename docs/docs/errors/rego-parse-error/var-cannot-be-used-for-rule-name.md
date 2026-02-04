---
sidebar_label: var cannot be used for rule name
image: /img/opa-errors.png
---

# `rego_parse_error`: var cannot be used for rule name

This cause for this error isn't always obvious at first glance, but as any parser error, this one is caused by an error
in the syntax of a Rego policy, and one around _rule declarations_ specifically. Look closely at the rule head for
anything that might be missing! The following examples will help demonstrate the most common causes for this error.

| Stage     | Category           | Message                            |
| --------- | ------------------ | ---------------------------------- |
| `parsing` | `rego_parse_error` | `var cannot be used for rule name` |

## Examples

The below example shows the most likely occurrence of this error: having
forgotten to import `if`, either via `import future.keywords.if`, or `import rego.v1` (OPA v0.59.0+):

:::note
Some that in OPA versions `>=1.0.0`, imports for `if` etc. are no longer
required.
:::

```rego
package policy

allow if {
    input.request.method == "GET"
    input.request.path == ["users"]
}
```

Another example could be having forgotten to add an assignment operator between the rule name and the value intended
for assignment:

```rego
package policy

import future.keywords.if

# note the missing `:=` between internal_user and email
internal_user email if {
    endswith(email, "@acmecorp.com")

    email := input.user.email
}
```

## How To Fix It

If caused by a missing import like the `if` keyword, simply add the import at
the top of your package. Other cases are likely caused by having forgotten to
add an assignment operator between the rule name and the value to assign. Once
you have identified the cause, fix the syntax error or adding the missing import
will address the issue.
