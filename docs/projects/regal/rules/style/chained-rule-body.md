# chained-rule-body

**Summary**: Avoid chaining rule bodies

**Category**: Style

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

The `opa fmt` command will automatically "unchain" chained rule bodies, so if you have enabled the [opa-fmt](opa-fmt)
rule, you may safely configure the level of this rule to `ignore`. While we normally don't include style rules covered
by `opa fmt`, this one is peculiar enough that we felt it was worthy of an exception.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    chained-rule-body:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Incremental Definitions](https://www.openpolicyagent.org/docs/policy-language/#incremental-definitions)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/chained-rule-body/chained_rule_body.rego)
