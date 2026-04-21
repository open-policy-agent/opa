# invalid-regexp

**Summary**: Invalid regular expression

**Category**: Bugs

**Avoid**

```rego
package policy

invalid if regex.match(`[abc`, input.text)
```

**Prefer**

```rego
package policy

valid if regex.match(`[abc]`, input.text)
```

## Rationale

An invalid regular expression typically fails silently (i.e. the result is undefined) at runtime when OPA evaluates the
function call, or with a runtime error if the `show-builtin-errors` option is enabled. While hopefully caught by unit
tests, tracking down a typo in a regular expression is still time consuming. This rule instead analyzes any regular
expressions found in a policy as you author it (using OPA's own `regex.is_valid` function) and reports invalid patterns
directly.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    invalid-regexp:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Regex Functions](https://www.openpolicyagent.org/docs/latest/policy-reference/#regex-functions)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/invalid-regexp/invalid_regexp.rego)
