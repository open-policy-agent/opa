# with-outside-test-context

**Summary**: `with` used outside of test context

**Category**: Performance

**Avoid**
```rego
package policy

allow if {
    some user in data.users

    # mock input to pass data to `allowed_user` rule
    allowed_user with input as {"user": user}
}

verified := io.jwt.verify_rs256(input.token, data.keys.verification_key)

allowed_user := input.user if {
    # this expensive rule will be evaluated for each user!
    verified
    "admin" in input.user.roles
}
```

**Prefer**
```rego
package policy

allow if {
    some user in data.users

    allowed_user({"user": user})
}

verified := io.jwt.verify_rs256(input.token, data.keys.verification_key)

allowed_user(user) := user if {
    # this expensive rule will be evaluated only once
    verified
    "admin" in user.roles
}
```

## Rationale

The `with` keyword exists primarily as a way to easily mock `input` or `data` in unit tests. While it's not forbidden to
use `with` in other contexts, and it's occasionally useful to do so, `with` is not optimized for performance and can
easily result in increased evaluation time if not used with care.

One optimization that OPA does all the time is to cache the result of rule evaluation. If OPA needs to evaluate the same
rule more than once as part of evaluating a query, the result of the first evaluation is memorized and the cost of
subsequent evaluations is essentially zero. Caching however assumes that the conditions that produced the result of the
first evaluation won't _change_ — and changing the conditions (i.e. `input` or `data`) for evaluation is the very
purpose of `with`! This means that rules evaluated in the context of `with` won't be cached, and an expensive operation,
like the `io.jwt.verify_rs256` built-in function called in the examples above would be evaluated for each `user` in
`data.users`, even if the `with` clause in this case doesn't change any value that the JWT verification function depends
on.

## Exceptions

The obvious exception is stated already in the title of this rule: unit tests! Use `with` as much as want here, as that
is what `with` is for.

Using `with` outside the context of unit tests is most commonly seen in policies using
[dynamic policy composition](https://www.styra.com/blog/dynamic-policy-composition-for-opa/), which typically involves
a "main" policy dispatching to a number of other policies and aggregating the result of evaluating each one. In this
scenario it's quite common to need to alter either `input` or `data` before evaluating a policy or rule, and `with` is
commonly used for this purpose. If you need to use `with` outside of tests, make sure that rules evaluated frequently
are done so outside of the scope of `with` to avoid performance issues.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  performance:
    with-outside-test-context:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [With Keyword](https://www.openpolicyagent.org/docs/policy-language/#with-keyword)
- Styra Blog: [Dynamic Policy Composition for OPA](https://www.styra.com/blog/dynamic-policy-composition-for-opa/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/performance/with-outside-test-context/with_outside_test_context.rego)
