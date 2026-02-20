# redundant-existence-check

**Summary**: Redundant existence check

**Category**: Bugs

**Automatically fixable**: [Yes](/regal/fixing)

**Avoid**
```rego
package policy

employee if {
    input.user.email
    endswith(input.user.email, "@acmecorp.com")
}

is_admin(user) if {
    user
    "admin" in user.roles
}
```

**Prefer**
```rego
package policy

employee if {
    endswith(input.user.email, "@acmecorp.com")
}

# alternatively

employee if endswith(input.user.email, "@acmecorp.com")

is_admin(user) if {
    "admin" in user.roles
}
```

## Rationale

Checking that a reference (like `input.user.email`) is defined before immediately using it is redundant. If the
reference is undefined, the next expression will fail anyway, as the value will be checked before the rest of the
expression is evaluated. While an extra check doesn't "hurt", it also serves no purpose, similarly to an unused
variable.

**Note**: This rule only applies to references that are immediately used in the next expression. If the reference is
used later in the rule, it won't be flagged. While the existence check _could_ be redundant even in that case, it could
also be used to avoid making some expensive computation, an `http.send` call, or whatnot.

## Exceptions

Function arguments where a boolean value is expected will be flagged as redundant existence checks, even though the
intent was to check the boolean condition.

```rego
report(user, is_admin) if {
    is_admin

    # more conditions
}
```

For these cases, prefer to be explicit about what the assertion is checking:

```rego
report(user, is_admin) if {
    is_admin == false # or true, != false, etc.

    # more conditions
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    redundant-existence-check:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/redundant-existence-check/redundant_existence_check.rego)
