# confusing-alias

**Summary**: Confusing alias of existing import

**Category**: Imports

**Avoid**
```rego
package policy

# both 'users' and 'employees' point to the same imported resource
import data.resources.users
import data.resources.users as employees
```

**Prefer**
```rego
package policy

# a single import for any given resource
import data.resources.users
```

**or**

```rego
package policy

# a single aliased import for any given resource
import data.resources.users as employees
```

## Rationale

Using an alias for an import occasionally helps improve intent and readability by using a name that's relevant to the
context in which the import is used. But an aliased import should never be used for a reference also imported
**without** an alias, as that's just confusing. Either use and alias or don't, but stick to one convention for any
given import.

Using two different aliases for the same import is also likely a mistake, and is similarly flagged by this rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    confusing-alias:
      # one of "error", "warning", "ignore"
      level: error
```
