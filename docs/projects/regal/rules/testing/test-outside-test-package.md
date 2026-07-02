# test-outside-test-package

**Summary**: Test outside of test package

**Category**: Testing

**Avoid**
```rego
package policy

allow if {
    "admin" in input.user.roles
}

# Tests in same package as policy
test_allow_if_admin {
    allow with input as {"user": {"roles": ["admin"]}}
}
```

**Prefer**
```rego
# Tests in separate package with _test suffix
package policy_test

import data.policy

test_allow_if_admin {
    policy.allow with input as {"user": {"roles": ["admin"]}}
}
```

## Rationale

While OPA's test runner will evaluate any rules with a `test_` prefix, it is a good practice to clearly separate tests
from production policy. This is easily done by placing tests in a separate package with a `_test` suffix, and correctly
[naming](https://www.openpolicyagent.org/projects/regal/rules/testing/file-missing-test-suffix) the test files.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  testing:
    test-outside-test-package:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Policy Testing](https://www.openpolicyagent.org/docs/policy-testing/)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/testing/test-outside-test-package/test_outside_test_package.rego)
