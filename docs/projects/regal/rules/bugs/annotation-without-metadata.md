# annotation-without-metadata

**Summary**: Annotation without metadata

**Category**: Bugs

**Avoid**
```rego
package policy

# description: allow allows
allow if {
    # ... some conditions
}
```

**Prefer**
```rego
package policy

# METADATA
# description: allow allows
allow if {
    # ... some conditions
}
```

## Rationale

A comment that starts with `<annotation-attribute>:` but is not part of a metadata block is likely a mistake. Add
`# METADATA` above the line to turn it into a
[metadata](https://www.openpolicyagent.org/docs/policy-language/#annotations) block.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    annotation-without-metadata:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Annotations](https://www.openpolicyagent.org/docs/policy-language/#annotations)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/annotation-without-metadata/annotation_without_metadata.rego)
