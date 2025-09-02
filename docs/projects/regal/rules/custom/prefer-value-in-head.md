# prefer-value-in-head

**Summary**: Prefer value in rule head

**Category**: Custom

**Avoid**
```rego
package policy

pin_as_number := val if {
    is_number(input.pin_code)
    val := to_number(input.pin_code)
}

deny contains message if {
    not input.user
    message := "user attribute missing from input"
}
```

**Prefer**
```rego
package policy

pin_as_number := to_number(input.pin_code) if is_number(input.pin_code)

deny contains "user attribute missing from input" if not input.user
```

## Rationale

Rules that return the value assigned in the last expression of the rule body may have the value, or the function
returning the value, moved directly to the rule head. This creates more succinct rules, and often allows for rules to be
expressed as "one-liners". This is not a general recommendation, but a style preference that a team or organization
might want to standardize on. As such, it is placed in the custom category, and must be explicitly enabled in
configuration.

The `only-scalars` configuration option may be used to only suggest moving scalar values (strings, numbers, booleans,
null) to the head, and not expressions or functions returning a value. With this option set to `true`, the following
example would be flagged:

```rego
deny contains message if {
    not input.user
    # value is a scalar
    message := "user attribute missing from input"
}
```

But not:

```rego
deny contains message if {
    not input.user
    # value returned from a function call, not suggested if `only-scalars` is set to `true`
    message := sprintf("user attribute missing from input: %v", [input])
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  custom:
    prefer-value-in-head:
      # note that all rules in the "custom" category are disabled by default
      # (i.e. level "ignore")
      #
      # one of "error", "warning", "ignore"
      level: error
      # whether to only suggest moving scalar values (strings, numbers, booleans, null)
      # to the head, and not expressions or functions
      only-scalars: false
      # variable names to exempt from the rule (by default, none)
      except-var-names:
      - report
      - violation
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/custom/prefer-value-in-head/prefer_value_in_head.rego)
