# repeated-computation

**Summary**: Repeated built-in call in the same scope

**Category**: Performance

**Avoid**

```rego
package policy

allow if {
    count(input.subjects) > 0
    count(input.subjects) < 100
}
```

**Prefer**

```rego
package policy

allow if {
    subject_count := count(input.subjects)
    subject_count > 0
    subject_count < 100
}
```

## Rationale

Some built-in functions allocate or otherwise perform work for each call. When
the same deterministic built-in is called multiple times with the same stable
arguments in the same scope, the result can be assigned once and reused.

This is especially useful for aggregate built-ins such as `count`, where
repeating the call makes the policy do the same work more than once.

## Exceptions

This rule is intentionally conservative. It only reports deterministic built-in
calls with arguments that are constants or stable `input`/`data` references.
Calls that depend on local variables are ignored because the variable binding
may change across scopes.

Calls inside comprehensions and `every` bodies are also ignored. These forms
introduce nested scopes where variables can be shadowed, so a textually
identical call is not always the same computation.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  performance:
    repeated-computation:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Policy Performance](https://www.openpolicyagent.org/docs/policy-performance)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/performance/repeated-computation/repeated_computation.rego)
