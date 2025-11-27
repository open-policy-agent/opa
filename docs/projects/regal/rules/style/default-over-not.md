# default-over-not

**Summary**: Prefer default assignment over negated condition

**Category**: Style

**Avoid**
```rego
package policy

username := input.user.name

username := "anonymous" if not input.user.name
```

**Prefer**
```rego
package policy

default username := "anonymous"

username := input.user.name
```

## Rationale

While both forms are valid, using the `default` keyword to assign a constant value in the fallback case better
communicates intent, avoids negation where it isn't needed, and requires less instructions to evaluate. Note that this
rule only covers simple cases where one rule assigns the "happy" path, and another rule assigns on the same condition
negated. This is by design, as using `not` and negation may very well be the right choice for more complex cases!

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    default-over-not:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Default Keyword](https://www.openpolicyagent.org/docs/policy-language/#default-keyword)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/default-over-not/default_over_not.rego)
