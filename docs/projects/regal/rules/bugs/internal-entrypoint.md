# internal-entrypoint

**Summary**: Entrypoint can't be marked internal

**Category**: Bugs

**Avoid**
```rego
package policy

# METADATA
# entrypoint: true
_authorized if {
    # some conditions
}
```

**Prefer**
```rego
package policy

# METADATA
# entrypoint: true
allow if _authorized

_authorized if {
    # some conditions
}
```

## Rationale

Rules marked as internal using the [underscore prefix convention](https://github.com/StyraInc/rego-style-guide#optionally-use-leading-underscore-for-rules-intended-for-internal-use)
cannot be used as entrypoints, as entrypoints by definition are public. Either rename the rule to mark it as public,
or use another public rule as an entrypoint, which may reference the internal rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    internal-entrypoint:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Rego Style Guide: [Optionally, use leading underscore for rules intended for internal use](https://github.com/StyraInc/rego-style-guide#optionally-use-leading-underscore-for-rules-intended-for-internal-use)
- Regal Docs: [no-defined-entrypoint](https://openpolicyagent.org/projects/regal/rules/idiomatic/no-defined-entrypoint)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/internal-entrypoint/internal_entrypoint.rego)
