# unresolved-reference

**Summary**: Unresolved Reference

**Category**: Imports

**Avoid**
References to unresolved packages and rules.

## Rationale

This rule is similar to `unresolved-import` rule and has the same rationale:
to avoid accidentally referencing rules and data that does not exist.
As the name suggests, this rule is stricter than the `unresolved-import` rule,
and will check for references to packages and rules that may not exist throughout the entire policy,
rather than just the imports.

This rule will have Regal try to resolve all references to external packages and rules by scanning all the policies it is
provided for **packages**, **rules** and **functions** that may resolve the reference. Note that Regal does not scan any
_data_ files. If no reference is found, the rule will flag it as unresolved.

## Configuration Options

This linter rule provides the following configuration options:
```yaml
rules:
  imports:
    unresolved-reference:
      # one of "error", "warning", "ignore"
      level: error
      # list of paths that should be ignored
      # these may be paths to data, or rules that may
      # not be present at the time of linting
      # using glob syntax
      except-paths:
        - data.identity.users
        - data.permissions.*
```

## Related Resources

- Unresolved Import Rule: [unresolved-import](./unresolved-import)
- OPA Docs: [Imports](https://www.openpolicyagent.org/docs/policy-language/#imports)
- OPA Docs: [Collaboration Using Import](https://www.openpolicyagent.org/docs/faq/#collaboration-using-import)
- OPA Issues: [Missing import should create error](https://github.com/open-policy-agent/opa/issues/491)
