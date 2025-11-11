# boolean-assignment

**Summary**: Prefer `if` over boolean assignment

**Category**: Idiomatic

**Avoid**

```rego
package policy

more_than_one_member := count(input.members) > 1
```

**Prefer**
```rego
package policy

more_than_one_member if count(input.members) > 1
```

## Rationale

Assigning the result of a boolean function is almost always redundant, as the boolean value returned by the expression
rarely is used for anything but to determine whether to continue evaluation. Moving the condition to the body following
an `if` will have the rule either evaluate to `true` or be undefined. For the few cases where `false` should be
returned, using a `default` rule assignment is preferable, as it is guaranteed to be assigned a value even on undefined
input:

```rego
package policy

default more_than_one_member := false

# will be assigned `false` even if input.members is undefined
more_than_one_member if count(input.members) > 1
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    boolean-assignment:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Styra Blog: [How to express OR in Rego](https://www.styra.com/blog/how-to-express-or-in-rego/)
- Regal Docs: [default-over-else](https://www.openpolicyagent.org/projects/regal/rules/style/default-over-else)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/boolean-assignment/boolean_assignment.rego)
