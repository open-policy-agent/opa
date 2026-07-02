# non-loop-expression

**Summary**: Non loop expression in loop

**Category**: Performance

**Avoid**

```rego
package policy

allow if {
    some email in input.emails
    "admin" in input.roles # <- this is not required in the loop
    endswith(email, "@example.com")
}
```

**Prefer**

```rego
package policy

allow if {
    "admin" in input.roles # <- moved out of the loop
    some email in input.emails
    endswith(email, "@example.com")
}
```

## Rationale

Expressions in loops are evaluated in each iteration of the loop. Expressions
that do not depend on the loop variable should be moved out of the loop to
save computation time.

'Loops' in Rego refers to anywhere a rule branches, for example:

- `some foo, bar in data.baz`
- `foo := data.baz[_]` (prefer using `some`)
- `walk(data.baz, [path, value])`
- ...

## Exceptions

This rule cannot yet detect the following cases.

Expressions overly nested in more than one loop:

```rego
package policy

allow if {
    some role in data.roles
    #                                    <--- Should be Here
    some permission in data.permissions[role]
    startswith(role, "admin-") # <- this is not required in the permission loop
    operation.permission == permission
}
```

Expressions nested within comprehensions:

```rego
package policy

allow if {
    roles := {role |
        prefix := data.prefix
        #                          <--- Should be Here
        some role in data.roles
        prefix != "" # <- this is not required in the role loop
        startswith(role, prefix)
    }
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  performance:
    non-loop-expression:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [defer-assignment](https://www.openpolicyagent.org/projects/regal/rules/performance/defer-assignment)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/performance/non-loop-expression/non_loop_expression.rego)
