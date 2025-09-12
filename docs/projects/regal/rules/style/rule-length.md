# rule-length

**Summary**: Max rule length exceeded

**Category**: Style

**Avoid**

Having too much logic placed in a single rule body.

**Prefer**

To use helper rules and functions to compose your rules.

## Rationale

Splitting up large rules into smaller ones, and liberally using helper rules and functions, makes your policy easier for
others to read and understand, and for yourself and your team to maintain.

Note that this rule only counts the number of lines of a rule, and currently does not take into account the actual
content inside of it. Neither does it try to analyze the complexity of the code in the rule.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    rule-length:
      # one of "error", "warning", "ignore"
      level: error
      # default limit is 30 lines
      max-rule-length: 30
      # default limit is 60 lines for test rules (i.e. prefixed with 'test_')
      max-test-rule-length: 60
      # whether to count comments as lines
      # by default, this is set to false
      count-comments: false
      # except rules with empty bodies from this rule, as they're
      # likely an assignment of long values rather than a "rule"
      # with conditions:
      #
      # users := [
      #     {"username": "ted"},
      #     {"username": "alice"},
      #     {"username": "bob"},
      #     # ... many more lines
      # ]
      #
      # the default value is true
      except-empty-body: true
```

## Related Resources

- Regal Docs: [file-length](https://openpolicyagent.org/projects/regal/rules/style/file-length)
- Regal Docs: [line-length](https://openpolicyagent.org/projects/regal/rules/style/line-length)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/rule-length/rule_length.rego)
