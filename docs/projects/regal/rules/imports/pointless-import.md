# pointless-import

**Summary**: Importing own package is pointless

**Category**: Imports

**Avoid**
```rego
package policy

# pointless, as policy is the own package
import data.policy

# pointless, as rules in own package can be referenced without the import
import data.policy.rule

rule if {
    # ..conditions..
}
```

**Prefer**
```rego
package policy
```

## Rationale

There's no point importing the own package, or rules from the same module, as both can be referenced just as well
without the import.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    pointless-import:
      # one of "error", "warning", "ignore"
      level: error
```
