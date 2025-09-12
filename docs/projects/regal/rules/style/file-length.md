# file-length

**Summary**: Max file length exceeded

**Category**: Style

**Avoid**

Excessively large policy files.

**Prefer**

Splitting large policy files into smaller ones.

## Rationale

Putting too much logic into a single file makes your policy harder to browse, read and maintain. Splitting logic into
several smaller files, and composing policy by proper use of packages and imports, highlights dependencies and
makes it easier to reason about.

Note that even a single **package** may be split up across several files! This is sometimes useful when different
features or functions belong in the same "group", but are not directly related to each other. An example of this could
be having a single package for configuration split across different files for different parts or functions of that
configuration.

As an added bonus, some tools — like Regal! — may even benefit from avoiding huge files as they process files in
parallel and thus will be able to handle many smaller files faster than a few large ones.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    file-length:
      # one of "error", "warning", "ignore"
      level: error
      # default limit is 500 lines
      max-file-length: 500
```

## Related Resources

- Styra Blog: [Dynamic Policy Composition](https://www.styra.com/blog/dynamic-policy-composition-for-opa/)
- Regal Docs: [line-length](https://openpolicyagent.org/projects/regal/rules/style/line-length)
- Regal Docs: [rule-length](https://openpolicyagent.org/projects/regal/rules/style/rule-length)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/file-length/file_length.rego)
