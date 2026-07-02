# leaked-internal-reference

**Summary**: Outside reference to internal rule or function

**Category**: Bugs

**Avoid**

```rego
package policy

# Import of rule or functions marked as internal
import data.users._all_users

allow if {
    # reference to rule or function marked as internal
    some role in data.permissions._roles
    # ...some conditions
}
```

## Rationale

OPA doesn't have a concept of "internal", or private rules and functions — and all rules can be queried or referenced
from the outside. Despite this fact, it has become a common convention to use an underscore prefix in the name of
rules and functions to indicate that they should be considered internal to the package that they're in:

```rego
# `allow` may be referenced from outside the package
allow if _user_is_developer

# `_user_is_developer` should not be referenced from outside the package
_user_is_developer if "developer" in input.users.roles
```

While this might seem like a pointless convention if it isn't enforced by OPA, it comes with a number of benefits:

- While OPA doesn't enforce it, other tools like linters can help with that. Like this rule does!
- It clearly communicates intent to other policy authors, and as a simple form of documentation
- Completion suggestions in editors can be filtered to exclude internal rules and functions
- Tools that render documentation from Rego policies and metadata annotations can exclude internal rules and functions
- Checking for unused rules and functions can be done much faster if they're known not to be referenced from outside

Do note that if you disagree with this rule, you don't need to disable it unless you use underscore prefixes to mean
something else. If you don't use underscore prefixes, nothing will be reported by this rule anyway. It does however
mean that the benefits listed above won't apply to your project.

## Exceptions

This rule is not enabled by default for test files. In tests, it can be useful
to reference internal rules and functions to achieve good test coverage, which
would be a violation of this rule. If you want to run this rule for tests
too, you can set `include-test-files: true` in the configuration for this rule
in your Regal config file.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    leaked-internal-reference:
      # one of "error", "warning", "ignore"
      level: error
      include-test-files: false # default is false
```

## Related Resources

- Rego Style Guide: [Optionally, use leading underscore for rules intended for internal use](https://www.openpolicyagent.org/docs/style-guide#optionally-use-leading-underscore-for-rules-intended-for-internal-use)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/leaked-internal-reference/leaked_internal_reference.rego)
