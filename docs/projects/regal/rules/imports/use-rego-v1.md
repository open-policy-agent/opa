# use-rego-v1

**Summary**: Use `import rego.v1`

**Category**: Imports

**Automatically fixable**: [Yes](https://openpolicyagent.org/projects/regal/fixing)

## Notice: Rule disabled by default since OPA 1.0

Since Regal v0.30.0, this rule is only enabled for projects that have either been explicitly configured to target
versions of OPA before 1.0, or if no configuration is provided — where Regal is able to determine that an older version
of OPA/Rego is being targeted. Consult the documentation on Regal's
[configuration](https://openpolicyagent.org/projects/regal#configuration) for information on how to best work with older versions of
OPA and Rego.

Since OPA v1.0, the `rego.v1` import is effectively a no-op. Developers working on a **policy library**, or other
Rego polices that are expected to be used with many different OPA versions, may however benefit from enabling this rule,
as having an `import rego.v1` in the policy ensures that v1 keywords will work correctly with OPA versions both
before and after OPA v1.0.

**Avoid**
```rego
package policy

# before OPA v0.59.0, this was best practice
import future.keywords.contains
import future.keywords.if

report contains item if {
    # ...
}
```

**Prefer**
```rego
package policy

# with OPA v0.59.0 and later, use import rego.v1 instead
# with OPA v1.0 and later, this import is unnecessary
import rego.v1

report contains item if {
    # ...
}
```

## Rationale

OPA [v0.59.0](https://github.com/open-policy-agent/opa/releases/tag/v0.59.0) introduced a new `rego.v1` import, which
allows policy authors to prepare for language changes coming in the future OPA 1.0 release. Some notable changes include:

- All "future" keywords that currently must be imported through `import future.keywords` will be part of Rego by
  default, without the need to first import them
- The `if` keyword will be required before the body of a rule
- The `contains` keyword will be required when declaring a multi-value rule (partial set rule)
- Deprecated built-in functions will be removed

Using `import rego.v1` ensures that these requirements are met in any package including the import, and tools like
`opa check` and `opa fmt` have been updated to help users in this transition.

See the [OPA v0.59.0 release notes](https://github.com/open-policy-agent/opa/releases/tag/v0.59.0) for more details.

### Capabilities

If you aren't yet using OPA v0.59.0 or later, it is recommended that you use the
[capabilities](https://openpolicyagent.org/projects/regal#capabilities) setting in your Regal configuration file to tell Regal what
version of OPA to target. This way you won't need to disable rules that require capabilities that aren't in the version
of OPA you're targeting, and allows for a smoother transition to newer versions of OPA when you're ready for that.
Another benefit of using capabilities is that Regal will include notices in the report when there are rules that have
been disabled due to missing capabilities, kindly reminding you of them, but without having the command fail.

In the example below we're using the capabilities setting to target OPA v0.55.0 (where `import rego.v1` is not
available):

**.regal/config.yaml** or **.regal.yaml**
```yaml
capabilities:
  from:
    engine: opa
    version: v0.55.0
```

Linting with the above configuration will exclude the `use-rego-v1` rule, but add a notice to the report reminding you
that it was disabled due to missing capabilities:

```shell
$ regal lint bundle
131 files linted. No violations found. 1 rule skipped:
- use-rego-v1: Missing capability for `import rego.v1`
```

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  imports:
    use-rego-v1:
      # one of "error", "warning", "ignore"
      level: error

# rather than disabling this rule, use the capabilities setting
# to tell Regal which version of OPA to target:
capabilities:
  from:
    engine: opa
    version: v0.58.0
```

## Related Resources

- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/imports/use-rego-v1/use_rego_v1.rego)
