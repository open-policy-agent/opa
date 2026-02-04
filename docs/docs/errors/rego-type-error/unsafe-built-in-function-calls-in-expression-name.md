---
sidebar_label: "unsafe built-in function calls in expression: {name}"
image: /img/opa-errors.png
---

# `rego_type_error`: unsafe built-in function calls in expression: `{name}`

| Stage         | Category          | Message                                                                                                     |
| ------------- | ----------------- | ----------------------------------------------------------------------------------------------------------- |
| `compilation` | `rego_type_error` | `unsafe built-in function calls in expression: {name}` (where name is the reference to a built-in function) |

## Description

This error is only surfaced when a
[capabilities](https://www.openpolicyagent.org/docs/deployments/#capabilities) configuration has been provided
to the OPA compiler (via `opa eval` or other commands). This is commonly used to restrict certain built-ins from being
used in environments where it's not deemed safe to allow them to execute. The likely most well-known example is the
[Rego Playground](https://play.openpolicyagent.org/), where the `http.send` function is disabled due to security
concerns. If you're seeing this error elsewhere, it's likely that whoever configured the OPA instance for your system
thought it would be a good idea to put certain restrictions in place.

## How To Fix It

Check the capabilities configuration provided to OPA when executed, and the `--capabilities` flag in particular.
If you're encountering this on the Rego Playground, simply run the policy on your own machine using e.g. `opa eval`
or `opa run` instead.
