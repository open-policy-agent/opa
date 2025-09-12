# print-or-trace-call

**Summary**: Call to `print` or `trace` function

**Category**: Testing

**Avoid**
```rego
package policy

reasons contains sprintf("%q is a dog!", [user.name]) if {
    some user in input.users
    user.species == "canine"

    # Useful for debugging, but leave out before committing
    print("user:", user)
}
```

## Rationale

The `print` function is really useful for development and debugging, but should normally not be included in production
policy. In order to be as useful for debugging purposes as possible, some performance optimizations are disabled when
`print` calls are encountered. Prefer decision logging in production.

The `trace` function serves no real purpose since the introduction of `print`, and should be considered deprecated.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  testing:
    print-or-trace-call:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Blog: [Introducing the OPA print function](https://blog.openpolicyagent.org/introducing-the-opa-print-function-809da6a13aee)
- OPA Docs: [Policy Reference: Debugging](https://www.openpolicyagent.org/docs/policy-reference/#debugging)
- OPA Docs: [Decision Logs](https://www.openpolicyagent.org/docs/management-decision-logs/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/testing/print-or-trace-call/print_or_trace_call.rego)
