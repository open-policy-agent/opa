# redundant-loop-count

**Summary**: Redundant count before loop

**Category**: Bugs

**Avoid**
```rego
package policy

allow if {
    # redundant count and > comparison
    count(input.user.roles) > 0
    some role in input.user.roles
    # .. do more with role ..
}
```

**Prefer**
```rego
package policy

allow if {
    some role in input.user.roles
    # .. do more with role ..
}
```

## Rationale

A loop that iterates over an empty collection evaluates to nothing, and counting the collection before the loop to
ensure it's not empty is therefore redundant.

## Exceptions

Note that this check is currently only performed on `some` loops, and not "ref-style" loops:

```rego
package policy

allow if {
    # this won't be flagged
    count(input.user.roles) > 0
    role := input.user.roles[_]
    # .. do more with role ..
}
```

Another good reason to
[prefer some .. in for iteration](https://www.openpolicyagent.org/projects/regal/rules/style/prefer-some-in-iteration)!

### `every` iteration

Counting to ensure a non-empty collection is used before `every` loops may **not** be redundant, as `every` evaluates
to `true` when an empty collection is passed.

```rego
package policy

allow if {
    # every would otherwise be `true` on empty input.user.roles
    # so this may be valid, depending on the outcome you expect
    count(input.user.roles) > 0
    every role in input.user.roles {
        # .. do more with each role ..
    }
}
```

If you want to have empty collections fail on `every` conditions, do make sure to use `count`!

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    redundant-loop-count:
      # one of "error", "warning", "ignore"
      level: error
```
