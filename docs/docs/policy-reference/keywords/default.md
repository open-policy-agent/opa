---
sidebar_label: default
title: 'Rego Keyword Examples: default'
---

The `default` keyword is used to provide a default value for rules and
functions. If in other cases, a rule or function is not defined, the default
value will be used.

It is often helpful to have know that a value will _always_ be defined so that
policy or callers do not also need to handle undefined values.

## Examples

<PlaygroundExample dir={require.context('./_examples/default/deny')} />

<PlaygroundExample dir={require.context('./_examples/default/overrides')} />
