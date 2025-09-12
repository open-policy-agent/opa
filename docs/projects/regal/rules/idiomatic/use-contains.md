# use-contains

**Summary**: Use the `contains` keyword

**Category**: Idiomatic

## Notice: Rule made obsolete by OPA 1.0

Since Regal v0.30.0, this rule is only enabled for projects that have either been explicitly configured to target
versions of OPA before 1.0, or if no configuration is provided — where Regal is able to determine that an older version
of OPA/Rego is being targeted. Consult the documentation on Regal's
[configuration](https://openpolicyagent.org/projects/regal#configuration) for information on how to best work with older versions of
OPA and Rego.

Since OPA v1.0, this rule is no longer needed as the Rego v1 syntax is now mandatory, and using `contains` is now the
de-facto way to define multi-value rules.

**Avoid**
```rego
package policy

import future.keywords.in

report[item] if {
    some item in input.items
    startswith(item, "report")
}

# unconditionally add an item to report
report["report1"]
```

**Prefer**
```rego
package policy

import future.keywords.contains
import future.keywords.if
import future.keywords.in

report contains item if {
    some item in input.items
    startswith(item, "report")
}

# unconditionally add an item to report
report contains "report1"
```

## Rationale

The `contains` keyword helps to clearly distinguish *multi-value rules* (or "partial rules") from
single-value rules ("complete rules"). Just like the `if` keyword, `contains` additionally makes the rule read the same
way in English as OPA interprets its meaning — a set that contains one or more values given some (optional) conditions.

OPA version 1.0, which is planned for 2024, will make the `contains` keyword mandatory. This rule helps you get ahead of
the curve and start using it today.

**Note**: don't forget to `import future.keywords.contains`! Or from OPA v0.59.0 and onwards, `import rego.v1`.

**Tip**: When either of the imports mentioned above are found in a Rego file, the `contains` keyword will be inserted
automatically at any applicable location by the `opa fmt` tool.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  idiomatic:
    use-contains:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [use-if](https://openpolicyagent.org/projects/regal/rules/idiomatic/use-if)
- OPA Docs: [Future Keywords](https://www.openpolicyagent.org/docs/policy-language/#future-keywords)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/idiomatic/use-contains/use_contains.rego)
