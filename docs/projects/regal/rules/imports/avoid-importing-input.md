# avoid-importing-input

**Summary**: Avoid importing `input`

**Category**: Imports

**Avoid**
```rego
package policy

# This is always redundant
import input

# This might be useful, but better to move to a local assignment
import input.user.email

allow if "admin" in input.user.roles

allow if {
    endswith(email, "@acmecorp.com")
}
```

**Prefer**
```rego
package policy

allow if "admin" in input.user.roles

allow if {
    email := input.user.email
    endswith(email, "@acmecorp.com")
}
```

## Rationale

Using an import for `input` is not necessary, as both `input` and `data` are globally available.

## Exceptions

Using an alias for `input` can sometimes be useful, e.g. when using `input` is known to represent something specific,
like a Terraform plan. Aliasing of specific input attributes should however be avoided in favor of local assignments.

```rego
package policy

# This is acceptable
import input as tfplan

# But this should be avoided - use assignment instead:
# username := input.user.name
import input.user.name as username

allow if {
    some resource_change in tfplan.resource_changes
    # ...
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    avoid-importing-input:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Rego Style Guide: [Avoid importing `input`](https://www.openpolicyagent.org/docs/style-guide#avoid-importing-input)
- OPA Docs: [Terraform Tutorial](https://www.openpolicyagent.org/docs/terraform)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/imports/avoid-importing-input/avoid_importing_input.rego)
