# rule-shadows-builtin

**Summary**: Rule name shadows built-in

**Category**: Bugs

**Avoid**
```rego
package policy

# `or` is an operator
or := 1 + 1

# `startswith` is a built-in function
startswith := indexof("rego", "r")
```

## Rationale

Using the name of built-in functions or operators as rule and variable names can lead to confusion and unexpected
behavior.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    rule-shadows-builtin:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [Built-in Functions](https://www.openpolicyagent.org/docs/policy-reference/#built-in-functions)
- OPA Repo: [builtin_metadata.json](https://github.com/open-policy-agent/opa/blob/main/builtin_metadata.json)
- Regal Docs: [var-shadows-builtin](https://www.openpolicyagent.org/projects/regal/rules/bugs/var-shadows-builtin)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/rule-shadows-builtin/rule_shadows_builtin.rego)
