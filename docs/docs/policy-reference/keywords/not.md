---
sidebar_label: not
title: 'Rego Keyword Examples: not'
---

The `not` keyword is the primary means of expressing
[negation](../../policy-language#negation) in Rego. Similar to other keywords in
Rego, it can also make your policies more 'English-like' and thus easier to
read.

```rego
allow if {
    not input.user.external
}
```

## Examples

<PlaygroundExample dir={require.context('./_examples/not/undefined/')} />

<PlaygroundExample dir={require.context('./_examples/not/negation/')} />
