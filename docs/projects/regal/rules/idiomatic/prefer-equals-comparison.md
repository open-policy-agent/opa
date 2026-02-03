# prefer-equals-comparison

**Summary**: Prefer `==` for equality comparison

**Category**: Idiomatic

**Automatically fixable**: [Yes](https://www.openpolicyagent.org/projects/regal/fixing)

**Avoid**
```rego
package policy

allow if {
    input.request.method = "GET"
    # .. more conditions ..
}
```

**Prefer**
```rego
package policy

allow if {
    input.request.method == "GET"
    # .. more conditions ..
}
```

## Rationale

The unification operator (`=`) can be used for both assignment and equality comparison in Rego. This can be a really
powerful feature where appropriate, but when the intent is to perform either assignment (`:=`) **or** an equality
comparison (`==`), using the operators designed for those specific purposes helps communicate that intent much more
clearly, and avoid some behaviors of the unification operator that may potentially be surprising (like in which order
expresions are evaluated). This is not a general recommendation against using the unification operator, mind you! But to
use the operators specifically for what they are designed for: `:=` for assignment, `==` for equality comparison, and
`=` for unification.

The OPA docs provide [more information](https://www.openpolicyagent.org/docs/policy-language#equality-assignment-comparison-and-unification)
on the topic, but to demonstrate a valid use case for the unification operator, this would be a good example:

```rego
user_id := id if ["users", id] = input.path
```

Which without unification would need both comparison and assignment:

```rego
user_id := id if {
    count(input.path) == 2
    input.path[0] == "users"
    id := input.path[1]
}
```

## How comparison is determined

Since the unification operator can be used for both assignment and comparison, this rule needs to determine that `=` is
used for comparison only. The rule reports a violation only when both sides of the `=` operator are determined to be
"unassignable", which is defined as:

1. Literal values (strings, numbers, booleans, nulls) or composite values containing no variables
2. References (e.g. `input.foo.bar`)
3. An input variable — meaning a variable which has previously been assigned a value outside of the expression

To provide a simple example of the the third point:

```rego
rule if {
    x = input.x # assignment, provided that `x` is not assigned elsewhere
}

rule if {
    x := 5
    x = input.x # comparison, as `x` is assigned above and unassignable here
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    prefer-equals-comparison:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [use-assignment-operator](https://www.openpolicyagent.org/projects/regal/rules/style/use-assignment-operator)
- OPA Docs: [Equality: Assignment, Comparison, and Unification](https://www.openpolicyagent.org/docs/policy-language#equality-assignment-comparison-and-unification)
- OPA Docs: [Comparisons](https://www.openpolicyagent.org/docs/policy-reference/builtins/comparison)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/prefer-equals-comparison/prefer_equals_comparison.rego)
