---
sidebar_label: contains
title: 'Rego Keyword Examples: contains'
---

Rego's `contains` keyword is used to incrementally build
[multi-value rules](https://www.openpolicyagent.org/docs/policy-language/#generating-sets)
in a policy. Often, tasks like validation are defined as a series of checks
and these break down nicely into a series of `contains` rules that evaluate
to a larger result. A `contains` rule typically takes the following form:

```rego
my_rule contains value if {
    # logic to check if the value should be set

    # set the value
    # value := ...
}
```

However, there some some different ways to use `contains` in a policy which are covered
in the examples below.

:::note
If you're looking for the built-in function `contains` for substring checking, you can read
about it in the [built-ins section](/docs/policy-reference/builtins/strings#builtin-strings-contains).
:::

## Examples

<PlaygroundExample dir={require.context('./_examples/contains/todo-list')} />

<PlaygroundExample dir={require.context('./_examples/contains/error-codes')} />

<PlaygroundExample dir={require.context('./_examples/contains/aggregated-validation')} />

<PlaygroundExample dir={require.context('./_examples/contains/object-validation')} />
