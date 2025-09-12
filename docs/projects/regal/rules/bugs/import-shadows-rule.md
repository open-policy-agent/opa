# import-shadows-rule

**Summary**: Import shadows rule

**Category**: Bugs

**Avoid**
```rego
package policy

import data.resources

# 'resources' shadowed by import 
resources contains resource if {
    # ...
}
```

**Prefer**
```rego
package policy

import data.resources

# using a different name for the rule
report contains resource if {
    # ...
}
```

```rego
package policy

# using an alias to avoid shadowing 'resources' rule
import data.resources as inventory

resources contains resource if {
    # ...
}
```

## Rationale

Imported identifers like `bar` in `import data.foo.bar` has higher precedence than a rule named `bar` in the same
package. This means that any rule that is shadowed by an import is effectively unreachable inside of the module.
Avoid shadowing either by renaming your rule or by using an alias for the imported identifier.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    import-shadows-rule:
      # one of "error", "warning", "ignore"
      level: error
```
