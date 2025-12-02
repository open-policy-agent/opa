# unused-output-variable

**Summary**: Unused output variable

**Category**: Bugs

**Avoid**
```rego
package policy

allow if {
    some x
    role := input.user.roles[x]

    # do something with "role", but not "x"
}
```

**Prefer**
```rego
package policy

allow if {
    # don't declare `x` output var as it is redundant
    role := input.user.roles[_]

    # do something with "role"
}

# or better (see prefer-some-in-iteration rule)

allow if {
    some role in input.user.roles

    # do something with "role"
}

# or actually _use_ value bound to `x` somewhere, like in another
# reference, function call, etc

allow if {
    some x
    input.user.roles[x] == data.required_roles[x]
}
```

## Rationale

Output variables are variables "automatically" bound to values during evaluation, most commonly in iteration. This is
a powerful feature of Rego that when used correctly can create concise but readable policies. However, output variables
that are declared but not later referenced are _effectively_ unused and should be replaced by wildcard variables (`_`),
or the use of `some .. in` iteration.

OPA itself has two methods for detecting and reporting unused variables as errors — one when using `some`:

```rego
allow if {
    # `x` is never used in the body — this is a compiler error
    some x
    input.user.roles[role]

    role == "admin"
}
```

And a [strict mode](https://www.openpolicyagent.org/docs/policy-language/#strict-mode) check for unused
variables defined in assignment (`:=`), or as a function arguments:

```rego
allow(role, required) {
    required_roles := data.required_roles

    role == "admin"

    # `required` never used in body, and neither is `required_roles`
    # both would be errors when strict mode is enabled
}
```

Neither of these methods however considers an unused output variable as "unused".

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    unused-output-variable:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [prefer-some-in-iteration](https://www.openpolicyagent.org/projects/regal/rules/style/prefer-some-in-iteration)
- OPA Docs: [Strict Mode](https://www.openpolicyagent.org/docs/policy-language/#strict-mode)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/unused-output-variable/unused_output_variable.rego)
