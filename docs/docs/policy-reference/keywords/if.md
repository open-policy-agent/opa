---
sidebar_label: if
title: 'Rego Keyword: if'
---

The `if` keyword is used when defining rules in Rego. `if` separates the
rule head from the rule body, making it clear which part of the rule is
is the condition (the part following the `if`).

The keyword is also use to make the policy rules written in Rego easier to
read by being more 'English-like'. For example:

```rego
rule := "some value" if some_condition
```


## Examples

<PlaygroundExample dir={require.context('./_examples/if/boolean')} />

<PlaygroundExample dir={require.context('./_examples/if/multi-value')} />

<PlaygroundExample dir={require.context('./_examples/if/functions')} />

<PlaygroundExample dir={require.context('./_examples/if/when-not')} />


## Further Reading

Below are some links that provide more information about the `if` keyword:

- If you are interested in learning about why `if` was added to Rego, see the
  notes in the
  [OPA v1.0](https://www.openpolicyagent.org/docs/opa-1/#enforce-use-of-if-and-contains-keywords-in-rule-head-declarations)
  documentation.
- Read the release notes from when the `if` keyword was added to Rego in
  [OPA v0.42.0](https://github.com/open-policy-agent/opa/releases/tag/v0.42.0).
- Using `if` is also
  [recommended by Regal](https://docs.styra.com/regal/rules/idiomatic/use-if).
