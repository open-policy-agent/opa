# metasyntactic-variable

**Summary**: Metasyntactic variable name

**Category**: Testing

**Avoid**
```rego
package policy

# Using metasyntactic names
foo := ["bar", "baz"]

# ...
```

**Prefer**
```rego
package policy

# Using names relevant to the context
roles := ["developer", "admin"]

# ...
```

## Rationale

Using "foo", "bar", "baz" and other [metasyntactic variables](https://en.wikipedia.org/wiki/Metasyntactic_variable) is
occasionally useful in examples, but should be avoided in production policy.

This linter rules forbids any metasyntactic variable names, as listed by Wikipedia:

<!-- cspell:disable -->
- foobar
- foo
- bar
- baz
- qux
- quux
- corge
- grault
- garply
- waldo
- fred
- plugh
- xyzzy
- thud
<!-- cspell:enable -->

## Exceptions

While there are no recommended exceptions to this rule, you could choose to allow metasyntactic variables in tests, or
perhaps code meant to be used in examples. When using a
[proper suffix](https://www.openpolicyagent.org/projects/regal/rules/testing/file-missing-test-suffix) for tests, like `_test.rego`,
simply configure an ignore pattern with the configuration of this rule:

```yaml
rules:
  testing:
    metasyntactic-variable:
      level: error
      ignore:
        files:
          - "*_test.rego"
```

If you'd rather use your own list of forbidden variable names or patterns, see the
[naming convention](https://www.openpolicyagent.org/projects/regal/rules/custom/naming-convention) rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  testing:
    metasyntactic-variable:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- Regal Docs: [Naming convention rule](https://www.openpolicyagent.org/projects/regal/rules/custom/naming-convention)
- Wikipedia: [Metasyntactic variable](https://en.wikipedia.org/wiki/Metasyntactic_variable)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/testing/metasyntactic-variable/metasyntactic_variable.rego)
