# rule-name-repeats-package

**Summary**: Avoid repeating package path in rule names

**Category**: Style

**Avoid**
```rego
package policy.authz

authz_allow if {
    user.is_admin
}
```

**Prefer**
```rego
package policy.authz

allow if {
    user.is_admin
}
```

## Rationale

When rules are referenced outside the package in which they are defined, they will be referenced using the package path.
For example, the `allow` rule in the `example` package, is available at `data.example.allow`. When rule names include
all or part of their package paths, this creates repetition in such references. For example, `authz_allow` in a package
`authz` is referenced with: `data.authz.authz_allow`. This repetition is undesirable as the reference is longer than
needed, and harder to read.

This rule was inspired by [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments#package-names).

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    rule-name-repeats-package:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/rule-name-repeats-package/rule_name_repeats_package.rego)
