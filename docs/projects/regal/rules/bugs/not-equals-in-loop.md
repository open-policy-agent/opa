# not-equals-in-loop

**Summary**: Use of != in loop

**Category**: Bugs

**Avoid**
```rego
package policy

deny if {
    "admin" != input.user.roles[_]
}
```

**Prefer**
```rego
package policy

deny if {
    not "admin" in input.user.roles
}

# Or as a one-liner
deny if not "admin" in input.user.roles
```

## Rationale

Likely one of the most common mistakes in Rego is to use `!=` in a loop thinking it means "not in". It took some years
for the `in` keyword to be added to Rego, so perhaps it's not surprising that this mistake is a common one even to this
day. If it doesn't mean "not in", what does it mean?

```rego
package policy

deny if {
    "admin" != input.user.roles[_]
}
```

The body of the `deny` rule above roughly translates to "for any item in `input.user.roles`, return true if the item is
not `admin`". This is almost never what the policy author intended. What the policy author likely intended was
"deny if `admin` is not in `input.user.roles`". The above policy would thus **not** deny a user with the roles
`["user", "admin"]` since the first item in the array is not "admin". This is almost never what the policy author
intended.

**Note**: This linter rule currently only checks for `!=` in a non-nested comparison where iteration happens on either
side of the comparison in the same expression. This will be improved in time. Another limitation is that this rule
currently only checks for wildcard iteration (`[_]`).

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    not-equals-in-loop:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/not-equals-in-loop/not_equals_in_loop.rego)
