# inconsistent-args

**Summary**: Inconsistently named function arguments

**Category**: Bugs

**Avoid**
```rego
package policy

find_vars(rule, node) if node in rule

# Order of arguments changed, or at least it looks like it
find_vars(node, rule) if {
    walk(rule, [path, value])
    # ...
}
```

**Prefer**
```rego
package policy

find_vars(rule, node) if node in rule

find_vars(rule, node) if {
    walk(rule, [path, value])
    # ...
}
```

## Rationale

Whenever a custom function declaration is repeated, the argument names should remain consistent in each declaration.

Inconsistently named function arguments is a likely source of bugs, and should be avoided.

## Exceptions

Using wildcards (`_`) in place of unused arguments is always allowed, and in fact enforced by the compiler:

```rego
package policy

find_vars(rule, node) if node in rule

# We don't use `node` here
find_vars(rule, _) if {
    walk(rule, [path, value])
    # ...
}
```

Using [pattern matching for equality](https://openpolicyagent.org/projects/regal/rules/idiomatic/equals-pattern-matching) checks is
also allowed.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    inconsistent-args:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [equals-pattern-matching](https://openpolicyagent.org/projects/regal/rules/idiomatic/equals-pattern-matching)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/inconsistent-args/inconsistent_args.rego)
