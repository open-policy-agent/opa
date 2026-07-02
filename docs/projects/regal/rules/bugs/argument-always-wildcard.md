# argument-always-wildcard

**Summary**: Argument is always a wildcard

**Category**: Bugs

**Avoid**
```rego
package policy

# there's only one definition of the last_name function in
# this package, and the second argument is never used
last_name(name, _) := lname if {
    parts := split(name, " ")
    lname := parts[count(parts) - 1]
}
```

**Prefer**
```rego
package policy

last_name(name) := lname if {
    parts := split(name, " ")
    lname := parts[count(parts) - 1]
}
```

## Rationale

Function definitions may use wildcard variables as arguments to indicate that the value is not used in the body of
the function. This helps make the function definition more readable, as it's immediately clear which of the arguments
are used in that definition of the function. This is particularly useful for incrementally defined functions:

```rego
package policy

default authorized(_, _) := false

authorized(user, _) if {
    # some logic to determine if authorized
}

# or

authorized(user, _) if {
    # some further logic to determine if authorized
}
```

In the example above, the second argument is a wildcard in all definitions, and could just as well be removed for a
cleaner definition. More likely, the argument was meant to be _used_, if only in one of the definitions:

```rego
package policy

default authorized(_, _) := false

authorized(user, _) if {
    # some logic to determine if authorized
}

# or

authorized(_, request) if {
    # some further logic to determine if authorized
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    argument-always-wildcard:
      # one of "error", "warning", "ignore"
      level: error
      # function name patterns for which this rule should make an exception
      # default is to ignore any function name starting with "mock_" as these
      # commonly don't need named arguments
      except-function-name-pattern: "^mock_"
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/argument-always-wildcard/argument_always_wildcard.rego)
