# time-now-ns-twice

**Summary**: Repeated calls to `time.now_ns`

**Category**: Bugs

**Avoid**

```rego
package policy

timed if {
    now := time.now_ns()

    # do some work here

    # this doesn't work! result is always 0
    print("work done in:", time.now_ns() - now, "ns)
}
```

**Prefer**

To use the tools OPA provides for measuring performance.

## Rationale

An important property of Rego is that it makes policy evaluation _predictable_. Using the same input to query OPA for a
decision multiple times should result in the same decision being made each time! A few built-in functions, like
`http.send`, or [time.now_ns](https://www.openpolicyagent.org/docs/policy-reference/#builtin-time-timenow_ns) are
however not **deterministic**. This means that repeated queries to policies where such functions are used may result in
different decisions being made. For example, a policy that validates JSON Web Tokens would normally check if the current
time is past the expiry value of the token, and deny any request where a token is found to be expired.

But while the use of non-deterministic built-in functions may result in different outcomes across different
queries, all built-in functions are deterministic **within the scope of a single evaluation**. This means that calling
e.g. `http.send` twice in a policy using the exact same arguments never results in different values being returned.
This is equally true for `time.now_ns`. In order to ensure predictable evaluation, the time returned by `time.now_ns` is
set once at the start of the evaluation, and never changes for the course of the request. Calling `time.now_ns` several
times within a rule is thus pointless, as the same value will be returned each time.

This mistake is most commonly observed when developers try to measure elapsed time in some parts of their policy, the
same way they'd normally do it using a traditional programming language (that is not deterministic). While this won't
work, OPA provides several tools to help measure performance, and learning how to use them well is the best way to
understand the performance characteristics of policy evaluation.

See the [performance](https://www.openpolicyagent.org/docs/policy-performance) section of the OPA docs for an
introduction to these tools, as well as advice on how to write performant policies.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    time-now-ns-twice:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [time.now_ns](https://www.openpolicyagent.org/docs/policy-reference/#builtin-time-timenow_ns)
- OPA Docs: [Policy Performance](https://www.openpolicyagent.org/docs/policy-performance)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/time-now-ns-twice/time_now_ns_twice.rego)
