# identically-named-tests

**Summary**: Multiple tests with same name

**Category**: Testing

**Avoid**
```rego
package policy_test

import data.policy

test_allow_if_admin {
    policy.allow with input as {"user": {"roles": ["admin"]}}
}

test_allow_if_admin {
    policy.allow with input as {"user": {"roles": ["superadmin"]}}
}
```

**Prefer**
```rego
package policy_test

import data.policy

test_allow_if_admin {
    policy.allow with input as {"user": {"roles": ["admin"]}}
}

test_allow_if_superadmin {
    policy.allow with input as {"user": {"roles": ["superadmin"]}}
}
```

## Rationale

While OPA allows multiple tests with the same name, using unique names for tests makes for easier to read test code, as
well as more informative test output. Since a single test may include any number of assertions, there's no need to reuse
test names within the same test package.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  testing:
    identically-named-tests:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Policy Testing](https://www.openpolicyagent.org/docs/policy-testing/)
- OPA GitHub: [Support running of individual test rules sharing same name](https://github.com/open-policy-agent/opa/issues/5766)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/testing/identically-named-tests/identically_named_tests.rego)
