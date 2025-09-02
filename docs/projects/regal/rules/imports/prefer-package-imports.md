# prefer-package-imports

**Summary**: Prefer importing packages over rules

**Category**: Imports

**Type**: Aggregate - only runs when more than one file is provided for linting

**Avoid**
```rego
package policy

# Rule imported directly
import data.users.first_names

has_waldo if {
    # Not obvious where "first_names" comes from
    "Waldo" in first_names
}
```

**Prefer**
```rego
package policy

# Package imported rather than rule
import data.users

has_waldo if {
    # Obvious where "first_names" comes from
    "Waldo" in users.first_names
}
```

## Rationale

Importing packages and using the package name as a "namespace" for imported rules and functions tends to make your code
easier to follow. This is especially true for large policies, where the distance from the import to actual use may be
several hundreds of lines.

## Exceptions

Regal has no way of knowing whether an import points to a rule, function or some external data — only that it doesn't
point to a package. Use the `ignore-import-paths` configuration option if you want to make exceptions for e.g. imports
of external data, or use the various ignore options to ignore entire files.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    prefer-package-imports:
      # one of "error", "warning", "ignore"
      level: error
      ignore-import-paths:
      # Make an exception for some specific import paths
      - data.permissions.admin.users
```

## Related Resources

- Rego Style Guide: [Prefer importing packages over rules and functions](https://github.com/StyraInc/rego-style-guide#prefer-importing-packages-over-rules-and-functions)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/imports/prefer-package-imports/prefer_package_imports.rego)
