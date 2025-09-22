---
sidebar_label: complete rules must not produce multiple outputs
image: /img/opa-errors.png
---

# eval_conflict_error: complete rules must not produce multiple outputs

Complete rules are rules that evaluate to a single value, or possibly, don't complete evaluation at all (where their
value is `undefined`). An "output" in this context could be likened to a **return value**, as should be familiar to
most developers. While Rego rules can be incrementally defined in multiple statements, a complete rule can't produce
multiple outputs, or "return values".

| Stage        | Category              | Message                                            |
| ------------ | --------------------- | -------------------------------------------------- |
| `evaluation` | `eval_conflict_error` | `complete rules must not produce multiple outputs` |

## Examples

The most trivial example of this would be to simply — and unconditionally — assign two different values to a rule:

```rego
package policy

x := 1

x := 2
```

Naturally, `x` can't be both `1` and `2` at the same time! Real-world examples are commonly not this obvious, but most
often involve scenarios where both rules check for conditions that can _potentially_ both be true. Consider the
following example:

```rego
package policy

import future.keywords.if

first_name := input.user.first_name

first_name := split(input.user.full_name, " ")[0] if {
    input.user.full_name != ""
}
```

This might seem like a reasonable way to extract the first name from a full name in case the first name isn't already
provided in the `input`. And in most cases, it **is**! A policy like this could work for a long time before failing,
if ever. Even if the `first_name` is provided for the user, it's likely going to match the first word of the
`full_name` so this rule has a good chance to work for weeks, months, or even years, before multiple outputs are
produced. However, once presented with `input` like the following, the rule will fail:

```json
{
  "user": {
    "first_name": "Johan",
    "full_name": "John Doe"
  }
}
```

Even when a condition like this is deemed "impossible" — perhaps because another system's constraints forbid it, it
should be considered a best practice to account for this type of scenario in your policies.

## How To Fix It

To fix this — simply ensure that the conditions for a rule to be assigned a conflicting value are mutually exclusive.
A common way to do this is to use negation (i.e. `not`) in one of the rule bodies:

```rego
package policy

import future.keywords.if

first_name := input.user.first_name

first_name := split(input.user.full_name, " ")[0] if {
    not input.user.first_name
    input.user.full_name != ""
}
```
