# use-strings-count

**Summary**: Use `strings.count` where possible

**Category**: Idiomatic

**Avoid**
```rego
package policy

num_as := count(indexof_n("foobarbaz", "a"))
```

**Prefer**
```rego
package policy

num_as := strings.count("foobarbaz", "a")
```

## Rationale

The `strings.count` function added in [OPA v0.67.0](https://github.com/open-policy-agent/opa/releases/tag/v0.67.0)
is both more readable and efficient compared to using `count(indexof_n(...))` and should therefore be preferred.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    use-strings-count:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [strings.count](https://www.openpolicyagent.org/docs/policy-reference/#builtin-strings-stringscount)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/use-strings-count/use_strings_count.rego)
