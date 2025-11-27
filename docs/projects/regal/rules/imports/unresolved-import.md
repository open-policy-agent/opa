# unresolved-import

**Summary**: Unresolved import

**Category**: Imports

**Type**: Aggregate - only runs when more than one file is provided for linting

**Avoid**

Imports that can't be resolved.

## Rationale

OPA does no compile time checks to ensure that references in imports _resolve_ to anything, and unresolved references at
runtime are simply **undefined**. This is not a bug in OPA, but a necessary feature to allow for dynamic loading of data
and policy at runtime. The fact that it's not a bug does however not mean that it can't be
[a problem](https://github.com/open-policy-agent/opa/issues/491)! A simple typo, a refactoring, or a mistake, could
easily lead to an an import being unresolved, and as such undefined at runtime.

This rule takes a stricter approach to imports, and will have Regal try to resolve them by scanning all the policies it
is provided for **packages**, **rules** and **functions** that may resolve the import. Note that Regal does not scan any
_data_ files. If no reference is found, the rule will flag it as unresolved.

Since unresolved imports may be perfectly valid — for example when an import points to data — this rule provides an
option in its configuration to except certain paths from being checked. These paths may even contain a wildcard suffix
to indicate that any path past the wildcard (e.g. `data.users.*`) should be ignored. It is also possible to use a
regular [ignore directive](https://www.openpolicyagent.org/projects/regal#inline-ignore-directives):

```rego
package example

# this is provided as data!
# regal ignore:unresolved-import
import data.users
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    unresolved-import:
      # one of "error", "warning", "ignore"
      level: error
      # list of paths that should be ignored
      # these may be paths to data, or rules that may
      # not be present at the time of linting
      except-imports:
        - data.identity.users
        - data.permissions.*
```

## Related Resources

- OPA Docs: [Imports](https://www.openpolicyagent.org/docs/policy-language/#imports)
- OPA Docs: [Collaboration Using Import](https://www.openpolicyagent.org/docs/faq/#collaboration-using-import)
- OPA Issues: [Missing import should create error](https://github.com/open-policy-agent/opa/issues/491)
  - GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/imports/unresolved-import/unresolved_import.rego)
