# rule-assigns-default

**Summary**: Rule assigned its default value

**Category**: Bugs

**Avoid**
```rego
package policy

default allow := false

# this rule assigns the same value as the default
# and the policy would work the same without it
allow := false if {
    not "admin" in input.user.roles
}
```

**Prefer**
```rego
package policy

default allow := false

# or just `allow if {` as `true` is implicit
allow := true if {
    "admin" in input.user.roles
}
```

## Rationale

When a default value is used for a rule, assigning the same value anywhere else to that rule is pointless, as the rule
would evaluate to the same value with or without the assignment.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    rule-assigns-default:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/rule-assigns-default/rule_assigns_default.rego)
