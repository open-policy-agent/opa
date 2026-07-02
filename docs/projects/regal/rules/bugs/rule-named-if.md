# rule-named-if

**Summary**: Rule named `if`

**Category**: Bugs

## Notice: Rule disabled by default since OPA 1.0

This rule is only enabled for projects that have either been explicitly configured to target versions of OPA before 1.0,
or if no configuration is provided — where Regal is able to determine that an older version of OPA/Rego is being
targeted. Consult the documentation on Regal's [configuration](https://www.openpolicyagent.org/projects/regal#configuration)
for information on how to best work with older versions of OPA and Rego.

Since OPA v1.0, this rule is automatically disabled, as the parser itself will throw an error if a rule is named `if`,
as that is made a keyword in Rego v1.0.

**Avoid**
```rego
package policy

allow := true if {
    authorized
}
```

Which actually means:

```rego
package policy

allow := true

if {
    authorized
}
```

**Prefer**
```rego
package policy

import rego.v1

allow := true if {
    authorized
}
```

## Rationale

Forgetting to import the `if` keyword (using `import future.keywords.if`, or from OPA v0.59.0+ `import rego.v1`) is a
common mistake. While this often results in a parse error, there are some situations where the parser can't tell if the
`if` is intended to be used as the imported keyword, or a new rule named `if`. This is almost always a mistake, and if
it isn't — consider using a better name for your rule!

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  bugs:
    rule-named-if:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/bugs/rule-named-if/rule_named_if.rego)
