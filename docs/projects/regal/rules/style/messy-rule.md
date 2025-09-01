# messy-rule

**Summary**: Messy incremental rule

**Category**: Style

**Avoid**

```rego
package policy

allow if something

unrelated_rule if {
    # ...
}

allow if something_else
```

**Prefer**

```rego
package policy

allow if something

allow if something_else

unrelated_rule if {
    # ...
}
```

## Rationale

Rules that are defined incrementally should have their definitions grouped together, as this makes the code easier to
follow. While this is mostly a style preference, having incremental rules grouped also allows editors like VS Code to
"know" that the rules belong together, allowing them to be smarter when displaying the symbols of a workspace.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    messy-rule:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/messy-rule/messy_rule.rego)
