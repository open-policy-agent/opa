# impossible-not

**Summary**: Impossible `not` condition

**Category**: Bugs

**Type**: Aggregate - runs both on single files as well as when more than one file is provided for linting

**Avoid**
```rego
package policy

report contains violation if {
    # ... some conditions
}
```

```rego
package policy_test

import data.policy

test_report_is_empty {
    # evaluation will stop here, as even an empty set is "true"
    not policy.report
}
```

**Prefer**
```rego
package policy

report contains violation if {
    # ... some conditions
}
```

```rego
package policy_test

import data.policy

test_report_is_empty {
    count(policy.report) == 0
}
```

## Rationale

The `not` keyword negates the expression that follows it. A common mistake, especially in tests, is to use `not`
to test the result of evaluating a partial (i.e. multi-value) rule. However, as even an empty set is considered
"truthy", the `not` will in that case always evaluate to `false`. There are more cases where `not` is impossible,
or a [constant condition](https://openpolicyagent.org/projects/regal/rules/bugs/constant-condition), but references to partial
rules are by far the most common. For tests where you want to assert the set is empty or has a specific number of
items, use the built-in `count` function instead.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    impossible-not:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [constant-condition](https://openpolicyagent.org/projects/regal/rules/bugs/constant-condition)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/impossible-not/impossible_not.rego)
