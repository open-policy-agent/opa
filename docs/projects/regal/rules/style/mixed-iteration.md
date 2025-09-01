# mixed-iteration

**Summary**: Mixed iteration style

**Category**: Style

**Avoid**
```rego
package policy

allow if {
    # mixing 'some .. in' and reference iteration
    some resource in input.assets[_]

    # do something with resource
}
```

**Prefer**
```rego
package policy

allow if {
    # using 'some .. in' iteration consistently
    some asset in input.assets
    some resource in asset

    # do something with resource
}

# alternatively

allow if {
    # using reference iteration consistently
    resource := input.assets[_][_]

    # do something with resource
}
```

## Rationale

Using `some .. in` is often the [best choice](https://openpolicyagent.org/projects/regal/rules/style/prefer-some-in-iteration) for
iteration in modern Rego, as it clearly communicates what's going on and which variables will be bound in each
iteration. An alternative approach is to place variables (including the special "wildcard" variable `_`) in parts of a
references, which unless assigned elsewhere automatically will be bound to every possible value in the collection being
traversed (often called "output variables").

"Reference style" iteration is often preferred when deeply nested structures are traversed, as they allow
expressing that in a very concise (and sometimes, more performant) manner.

While both forms of iteration are valid and have their place, mixing both forms in a single iteration is arguably
confusing. Feel free to choose either `some .. in` or reference style depending on your preference (and when in doubt,
use `some .. in`), but don't mix the two different styles in a single iteration expression.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    mixed-iteration:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [prefer-some-in-iteration](https://openpolicyagent.org/projects/regal/rules/style/prefer-some-in-iteration)
- OPA Docs: [Membership and iteration](https://www.openpolicyagent.org/docs/policy-language/#membership-and-iteration-in)
