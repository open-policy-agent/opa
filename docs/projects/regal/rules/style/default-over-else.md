# default-over-else

**Summary**: Prefer default assignment over fallback `else`

**Category**: Style

**Avoid**
```rego
package policy

permissions := ["read", "write"] if {
    input.user == "admin"
} else := ["read"]
```

**Prefer**
```rego
package policy

default permissions := ["read"]

permissions := ["read", "write"] if {
    input.user == "admin"
}
```

## Rationale

The `else` keyword has a single purpose in Rego — to allow a policy author to control the order of evaluation. Whether
several `else`-clauses are chained or not, it's common to use a last "fallback" `else` to cover all cases not covered by
the conditions in the preceding `else`-bodies. A kind of "catch all", or "default" condition. This is useful, but Rego
arguably provides a more idiomatic construct for default assignment: the
[default keyword](https://www.openpolicyagent.org/docs/policy-language/#default-keyword).

While the end result is the same, default assignment has the benefit of more clearly — and **before** the conditional
assignments — communicating what the *safe* option is. This is particularly important for
[entrypoint](https://openpolicyagent.org/projects/regal/rules/idiomatic/no-defined-entrypoint) rules, where the
default value of a rule is a part of the rule's contract.

## Exceptions

OPA [v0.55.0](https://github.com/open-policy-agent/opa/releases/tag/v0.55.0) introduced support for the default keyword
for custom functions. This means that `else` fallbacks in functions may now be rewritten to use default assignment too:

```rego
package policy

first_name(full_name) := split(full_name, " ")[0] if {
    full_name != ""
} else := "Unknown"
```

Could now be written as:

```rego
package policy

default first_name(_) := "Unknown"

first_name(full_name) := split(full_name, " ")[0] if {
    full_name != ""
}
```

Default value assignment for functions however come with a big caveat — the default case will only be triggered if all
arguments passed to the function evaluate to a *defined value*. Thus, calling the `first_name` function from our above
example is **not** guaranteed to return a value of `"Unknown"`:

```rego
# undefined if `input.name` is undefined
fname := first_name(input.name)
```

Whether deemed acceptable or not, this differs enough from default assignment of rules to make this preference opt-in
rather than opt-out. Use the `prefer-default-functions` configuration option to control whether `default` assignment
should be preferred over `else` fallbacks also for custom functions. The default value (no pun intended!) of this config
option is `false`.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    default-over-else:
      # one of "error", "warning", "ignore"
      level: error
      # whether to prefer default assignment over
      # `else` fallbacks for custom functions
      prefer-default-functions: false
```

## Related Resources

- OPA Docs: [Default Keyword](https://www.openpolicyagent.org/docs/policy-language/#default-keyword)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/default-over-else/default_over_else.rego)
