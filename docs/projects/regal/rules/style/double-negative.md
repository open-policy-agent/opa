# double-negative

**Summary**: Avoid double negatives

**Category**: Style

**Avoid**
```rego
package negative

fine if not not_fine

with_friends if not without_friends

not_fine := input.fine != true

without_friends if count(input.friends) == 0
```

**Prefer**
```rego
package negative

fine if input.fine == true

with_friends if count(input.friends) > 0
```

## Rationale

While rules using double negatives — like `not no_funds` — occasionally make sense, it is often worth considering
whether the rule could be rewritten without the negative. For example, `not no_funds` could be rewritten as `funds` or
`has_funds`, or `funds_available`.

Access control policy often includes rules using some form of double negatives, like `allow if not deny`. That's
considered OK, and the `double-negative` rule is limited to check for a limited list of words:

- `not cannot_`
- `not no_`
- `not non_`
- `not not_`,

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    double-negative:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/double-negative/double_negative.rego)
