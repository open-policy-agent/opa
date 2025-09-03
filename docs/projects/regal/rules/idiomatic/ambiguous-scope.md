# ambiguous-scope

**Summary**: Ambiguous metadata scope

**Category**: Idiomatic

**Avoid**
```rego
package policy

# METADATA
# description: allow is true if the user is admin, or the requested resource is public
allow if user_is_admin

allow if public_resource
```

**Prefer**
```rego
package policy

# METADATA
# description: allow is true if the user is admin, or the requested resource is public
# scope: document
allow if user_is_admin

allow if public_resource
```

**Or (scope `rule` implied, but _all_ incremental definitions are annotated)**
```rego
package policy

# METADATA
# description: allow is true if the user is admin
allow if user_is_admin

# METADATA
# description: allow is true if the requested resource is public
allow if public_resource
```

**Or (scope `rule` explicit)**
```rego
package policy

# METADATA
# description: allow is true if the user is admin
# scope: rule
allow if user_is_admin

allow if public_resource
```

## Rationale

The default scope for metadata annotating a rule is the `rule` scope, which
"[applies to the individual rule statement](https://www.openpolicyagent.org/docs/policy-language/#scope)" only.
This default is sensible for a rule defined only once, but is somewhat ambiguous for a rule defined incrementally, like
the `allow` rule in the examples above. Was the intention really to annotate that single definition, or the rule as
whole? Most likely the latter, and that's what the `document` scope is for.

If only a single rule in a group of incremental rule definitions is annotated, it should have it's `scope` set explicitly
to either `document` or `rule`. If all incremental definitions are annotated, explicit `scope: rule` is not required.

## Exceptions

If a single incremental rule definition is annotated as `entrypoint: true`, this rule will allow that.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    ambiguous-scope:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Annotations](https://www.openpolicyagent.org/docs/policy-language/#annotations)
- Regal Docs: [no-defined-entrypoint](https://openpolicyagent.org/projects/regal/rules/idiomatic/no-defined-entrypoint)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/ambiguous-scope/ambiguous_scope.rego)
