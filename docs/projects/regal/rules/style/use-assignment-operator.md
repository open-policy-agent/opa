# use-assignment-operator

**Summary**: Prefer `:=` over `=` for assignment

**Category**: Style

**Automatically fixable**: [Yes](https://openpolicyagent.org/projects/regal/fixing)

**Avoid**
```rego
package policy

default allow = false

first_name(name) = split(name, " ")[0]

allow if {
    username = input.user.name
    # .. more conditions ..
}
```

**Prefer**
```rego
package policy

default allow := false

first_name(full_name) := split(full_name, " ")[0]

allow if {
    username := input.user.name
    # .. more conditions ..
}
```

## Rationale

Rego has three operators related to assignment and equality:

- `:=` is the assignment operator, and is only used to assign values to variables
- `==` is the equality operator, and is only used to compare values
- `=` is the unification operator, and is used both to assign values to variables **and** compare values

While it often is "harmless" to use the unification operator (`=`) for assignment, the assignment operator (`:=`)
removes any ambiguities around intent, and prevents some hard to debug issues. Consider:

```rego
allow if {
    username = input.user.name
    # .. more conditions ..
}
```

Using the unification operator, `username` is either assigned (if `username` isn't defined elsewhere in the
policy) or being checked for equality (if `username` is defined elsewhere in the policy). Using `:=` for assignment,
and `==` for equality comparison removes this ambiguity and make the intent obvious.

In some cases, `=` and `:=` may be used interchangeably, as the result is the same either way:

```rego
first_name(full_name) = split(full_name, " ")[0]
# same as
first_name(full_name) := split(full_name, " ")[0]
```

Even when that is the case, using `:=` consistently should be considered a best practice.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    use-assignment-operator:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Equality: Assignment, Comparison, and Unification](https://www.openpolicyagent.org/docs/policy-language/#equality-assignment-comparison-and-unification)
- Rego Style Guide: [Don't use unification operator for assignment or comparison](https://github.com/StyraInc/rego-style-guide#dont-use-unification-operator-for-assignment-or-comparison)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/use-assignment-operator/use_assignment_operator.rego)
