# chained-rule-body

**Summary**: Avoid chaining rule bodies

**Category**: Custom

**Avoid**
```rego
package policy

has_x_or_y {
    input.x
} {
    input.y
}
```

**Prefer**
```rego
package policy

has_x_or_y {
    input.x
}

has_x_or_y {
    input.y
}
```

## Rationale

If the head of the rule is same, it's possible to chain multiple rule bodies together to obtain the same result. This
form was more common in the past, but is no longer recommended as it is arguably less readable, and less likely to be
understood by people new to Rego.

## Exceptions

The `opa fmt` command will automatically "unchain" chained rule bodies, so if you have enabled the [opa-fmt](https://www.openpolicyagent.org/projects/regal/rules/style/opa-fmt)
rule (as it is by default), there's no point in enabling this rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  custom:
    chained-rule-body:
      # note that all rules in the "custom" category are disabled by default
      # (i.e. level "ignore") as some configuration needs to be provided by
      # the user (i.e. you!) in order for them to be useful.
      #
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Incremental Definitions](https://www.openpolicyagent.org/docs/policy-language/#incremental-definitions)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/custom/chained-rule-body/chained_rule_body.rego)
