# trailing-default-rule

**Summary**: Default rule should be declared first

**Category**: Style

**Avoid**
```rego
package policy

allow if {
    # some conditions
}

default allow := false
```

**Prefer**
```rego
package policy

default allow := false

allow if {
    # some conditions
}
```

## Rationale

Presenting the default value of a rule (if one is used) before the conditional rule assignments is a common practice,
and it's often easier to to reason about conditional assignments knowing there is a default fallback value in place.
For that reason, it's recommended to follow the convention and place the default rule declaration before rules
conditionally assigning values.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    trailing-default-rule:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/trailing-default-rule/trailing_default_rule.rego)
