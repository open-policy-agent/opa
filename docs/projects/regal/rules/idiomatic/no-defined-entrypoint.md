# no-defined-entrypoint

**Summary**: Missing entrypoint annotation

**Category**: Idiomatic

**Type**: Aggregate - only runs when more than one file is provided for linting

**Avoid**
```rego
package policy

default allow := false

# Nothing wrong with this rule, but an
# entrypoint should be documented as such
allow if user_is_admin
allow if public_resource_read

user_is_admin if {
    some role in input.user.roles
    role in data.permissions.admin_roles
}

public_resource_read if {
    input.request.method == "GET"
    input.request.path[0] == "public"
}
```

**Prefer**
```rego
package policy

default allow := false

# METADATA
# description: Allow only admins, or reading public resources
# entrypoint: true
allow if user_is_admin
allow if public_resource_read

user_is_admin if {
    some role in input.user.roles
    role in data.permissions.admin_roles
}

public_resource_read if {
    input.request.method == "GET"
    input.request.path[0] == "public"
}
```

## Rationale

Defining one or more entrypoints for your policies is a good practice to follow. An entrypoint is simply a package or
rule that is meant to be queried for decisions from the outside. While it might seem obvious to the policy author which
rules are meant to be queried, adding an extra line of two of metadata will help make it obvious to others.

Marking a package or rule via an
[entrypoint annotation attribute](https://www.openpolicyagent.org/docs/policy-language/#entrypoint) not only
provides good documentation for others, but also unlocks programmatic possibilities, like:

1. Your policy library may be compiled to WebAssembly without extra entrypoint arguments
1. Your policy library may be compiled to an
   [intermediate representation](https://blog.openpolicyagent.org/i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d)
   (IR) format without extra entrypoint arguments
1. External applications may present your entrypoints as part of rendered documentation
1. External applications may use your entrypoints to know what to evaluate
1. External applications — like Regal — may use this information to determine what other rules are used or not

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    no-defined-entrypoint:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Metadata](https://www.openpolicyagent.org/docs/policy-language/#metadata)
- OPA Docs: [Entrypoint](https://www.openpolicyagent.org/docs/policy-language/#entrypoint)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/no-defined-entrypoint/no_defined_entrypoint.rego)
