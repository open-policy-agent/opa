# unconditional-assignment

**Summary**: Unconditional assignment in rule body

**Category**: Style

**Avoid**
```rego
package policy

full_name := name if {
    name := concat(", ", [input.first_name, input.last_name])
}

divide_by_ten(x) := y if {
    y := x / 10
}

names contains name if {
    name := "Regal"
}
```

**Prefer**
```rego
package policy

full_name := concat(", ", [input.first_name, input.last_name])

divide_by_ten(x) := x / 10

names contains "Regal"
```

## Rationale

Rules that return values unconditionally should place the assignment directly in the rule head, as doing so in the rule
body adds unnecessary noise.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    unconditional-assignment:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Rego Style Guide: [Prefer unconditional assignment in rule head over rule body](https://www.openpolicyagent.org/docs/style-guide#prefer-unconditional-assignment-in-rule-head-over-rule-body)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/unconditional-assignment/unconditional_assignment.rego)
