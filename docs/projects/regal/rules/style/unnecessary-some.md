# unnecessary-some

**Summary**: Unnecessary use of `some`

**Category**: Style

**Avoid**
```rego
package policy

is_developer if some "developer" in input.user.roles
```

**Prefer**

```rego
package policy

is_developer if "developer" in input.user.roles
```

## Rationale

Use the `some .. in` construct when you want to loop over a collection and assign variables in the iteration. If you
know the value you're looking for, just use the `in` keyword directly without using `some`.

## Exceptions

Note that `some .. in` iteration can be used with a limited form of pattern matching where either the key or the value
should match for the loop assignment to succeed. This is not commonly needed, but considered OK.

```rego
package policy

developers contains name if {
    # name will only be bound when the value is "developer"
    some name, "developer" in {"alice": "developer", "bob": "developer", "charlie": "manager"}
}
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    unnecessary-some:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Membership and iteration: `in`](https://www.openpolicyagent.org/docs/policy-language/#membership-and-iteration-in)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/unnecessary-some/unnecessary_some.rego)
