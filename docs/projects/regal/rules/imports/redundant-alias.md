# redundant-alias

**Summary**: Redundant alias

**Category**: Imports

**Avoid**
```rego
package policy

import data.users.permissions as permissions
```

**Prefer**
```rego
package policy

import data.users.permissions
```

## Rationale

The last component of an import path can always be referenced by the last
component of the import path itself inside the package in which it's imported.
Using an alias with the same name is thus redundant, and should be omitted.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    redundant-alias:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/imports/redundant-alias/redundant_alias.rego)
