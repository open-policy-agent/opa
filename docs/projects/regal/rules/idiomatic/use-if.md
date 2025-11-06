# use-if

**Summary**: Use the `if` keyword

**Category**: Idiomatic

## Notice: Rule disabled by default since OPA 1.0

This rule is only enabled for projects that have either been explicitly configured to target versions of OPA before 1.0,
or if no configuration is provided — where Regal is able to determine that an older version of OPA/Rego is being
targeted. Consult the documentation on Regal's [configuration](https://www.openpolicyagent.org/projects/regal#configuration)
for information on how to best work with older versions of OPA and Rego.

Since OPA v1.0, this rule is no longer needed simply because the Rego v1 syntax is made mandatory, and the use of `if`
is now enforced before all rule bodies.

**Avoid**
```rego
package policy

import future.keywords.in

is_admin {
    "admin" in input.user.roles
}
```

**Prefer**
```rego
package policy

import future.keywords.if
import future.keywords.in

is_admin if {
    "admin" in input.user.roles
}

# alternatively

is_admin if "admin" in input.user.roles
```

## Rationale

The `if` keyword helps communicate what Rego rules really are — conditional assignments. Using `if` in other words makes
the rule read the same way in English as it will be interpreted by OPA, i.e:

```rego
rule := "some value" if some_condition
```

OPA version 1.0, which is planned for 2024, will make the `if` keyword mandatory. This rule helps you get ahead of the
curve and start using it today.

**Note**: don't forget to `import future.keywords.if`! Or from OPA v0.59.0 and onwards, `import rego.v1`.

**Tip**: When either of the imports mentioned above are found in a Rego file, the `if` keyword will be inserted
automatically at any applicable location by the `opa fmt` tool.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    use-if:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [use-contains](https://www.openpolicyagent.org/projects/regal/rules/idiomatic/use-contains)
- OPA Docs: [Future Keywords](https://www.openpolicyagent.org/docs/policy-language/#future-keywords)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/use-if/use_if.rego)
