# duplicate-rule

**Summary**: Duplicate rule

**Category**: Bugs

**Avoid**
```rego
package policy

allow if user.is_admin

allow if user.is_developer

# we already covered this!
allow if user.is_admin
```

**Prefer**
```rego
package policy

allow if user.is_admin

allow if user.is_developer
```

## Rationale

Duplicated rules are likely a mistake, perhaps from pasting contents from another file.

This rule identifies rules that are _identical_ in terms of their name, assigned value, and body — excluding
whitespace. In technical terms, if two or more rules share the same abstract syntax tree, they are considered
to be duplicates.

## Exceptions

Note that this rule currently works at the scope of a single file. If you're using the same package across multiple
files, there could still be duplicates across those files. This will be addressed in a future version of this rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    duplicate-rule:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/duplicate-rule/duplicate_rule.rego)
