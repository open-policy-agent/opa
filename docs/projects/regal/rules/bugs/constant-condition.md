# constant-condition

**Summary**: Constant condition

**Category**: Bugs

**Automatically fixable**: [Yes](https://www.openpolicyagent.org/projects/regal/fixing)

**Avoid**
```rego
package policy

allow if {
    1 == 1
}
```

**Prefer**
```rego
package policy

allow := true
```

## Rationale

While most often a mistake, constant conditions are sometimes used as placeholders, or "TODO logic". While this is
harmless, it has no place in production policy, and should be replaced or removed before deployment.

Constant operands of `and`/`or` expressions are reported as well. An expression that is constant in its entirety,
like `1 or 2`, is removed when fixed, while one where only a single operand is constant is collapsed to the operand
kept, so that `input.a and 1` becomes `input.a`. The exception is when the operand kept can't make up an expression
of its own, as brace enclosed and multi expression operands can't. There's no way of collapsing
`{ input.a; input.b } and 1`, so the constant operand of an expression like that is left alone.

## Configuration Options

This linter rule provides the following [configuration options](https://www.openpolicyagent.org/projects/regal/configuration):

```yaml title=".regal/config.yaml or .regal.yaml"
rules:
  bugs:
    constant-condition:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/constant-condition/constant_condition.rego)
