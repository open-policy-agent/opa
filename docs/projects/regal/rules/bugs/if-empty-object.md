# if-empty-object

**This rule has been deprecated and replaced by the
[if-object-literal](https://openpolicyagent.org/projects/regal/rules/bugs/if-object-literal) rule. Documentation kept here only for
the sake of posterity.**

**Summary**: Empty object following `if`

**Category**: Bugs

**Avoid**
```rego
package policy

allow if {}
```

## Rationale

An empty rule body would previously be considered an error by OPA. With the introduction, and use of the `if` keyword,
that is no longer the case. In fact, empty `{}` is not considered a rule body _at all_, but rather an empty object,
meaning that `if {}` will always evaluate. This is likely a mistake, and while hopefully caught by tests, should be
avoided.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    if-empty-object:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [constant-condition](https://openpolicyagent.org/projects/regal/rules/bugs/constant-condition)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/if-empty-object/if_empty_object.rego)
