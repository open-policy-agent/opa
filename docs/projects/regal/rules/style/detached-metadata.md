# detached-metadata

**Summary**: Detached metadata annotation

**Category**: Style

**Avoid**
```rego
package authz

 # METADATA
 # description: allow any requests by admin users

allow if {
    "admin" in input.user.roles
}
```

**Prefer**
```rego
package authz

# METADATA
# description: allow any requests by admin users
allow if {
    "admin" in input.user.roles
}
```

## Rationale

Metadata annotations should be placed directly above the package, rule or function they are annotating. While OPA
accepts any number of newlines between an annotation and the package/rule it applies to, this makes it difficult to
connect the two when reading the policy. Always optimize for readability!

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    detached-metadata:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Annotations](https://www.openpolicyagent.org/docs/policy-language/#annotations)
- OPA Docs: [Accessing Annotations](https://www.openpolicyagent.org/docs/policy-language/#accessing-annotations)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/detached-metadata/detached_metadata.rego)
