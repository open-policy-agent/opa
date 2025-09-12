# defer-assignment

**Summary**: Assignment can be deferred

**Category**: Performance

**Avoid**
```rego
package policy

allow if {
    resp := http.send({"method": "GET", "url": "http://example.com"})

    # this check does not depend on the response above
    # and thus the resp := ... assignment can be deferred to
    # after the check
    input.user.name in allowed_users

    resp.status_code == 200

    # more done with response here
}
```

**Prefer**
```rego
package policy

allow if {
    input.user.name in allowed_users

    # the next expression *does* depend on `resp`
    resp := http.send({"method": "GET", "url": "http://example.com"})

    resp.status_code == 200

    # more done with response here
}
```

## Rationale

Assignments are normally cheap, but certainly not always. If the right-hand side of an assignment is expensive,
deferring the assignment to where it's needed can save a considerable amount of time. Even for less expensive
assignments, code tends to be more readable when assignments are placed close to where they're used.

This rule uses a fairly simplistic heuristic to determine if an assignment can be deferred:

- The next expression is not an assignment
- The next expression does not depend on the assignment
- The next expression does not initialize iteration

It is possible that the rule will be improved to cover more cases in the future.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  performance:
    defer-assignment:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/performance/defer-assignment/defer_assignment.rego)
