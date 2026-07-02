# opa-fmt

**Summary**: File should be formatted with `opa fmt`

**Category**: Style

**Automatically fixable**: [Yes](https://www.openpolicyagent.org/projects/regal/fixing)

**Avoid**

Inconsistent style across policy files and repositories.

## Rationale

The `opa fmt` tool ensures consistent formatting across teams and projects. Unified formatting is a big win, and saves a
lot of time in code reviews arguing over details around style.

A good idea could be to run `opa fmt --write` on save, which can be configured in most editors.

**Tip**: `opa fmt` uses tabs for indentation. By default, GitHub uses 8 spaces to display tabs, which is arguably a bit
much. You can change this preference for your account in `github.com/settings/appearance`, or provide an `.editorconfig`
file in your policy repository, which will be used by GitHub (and other tools) to properly display your Rego files:

```ini
[*.rego]
end_of_line = lf
insert_final_newline = true
charset = utf-8
indent_style = tab
indent_size = 4
```

## OPA Format with Rego v1 and v0

OPA 1.0 makes Rego v1 the default. This change mandated some changes to the
functionality of the `opa fmt` command and a number of new options for working
with mixed version code bases. See the
[OPA documentation](https://www.openpolicyagent.org/docs/cli/#opa-fmt)
for the command's options.

In Regal, a v0 file will have the `opa-fmt` violation unless it's been formatted
with `opa fmt --v0-v1`. A v1 file will have the `opa-fmt` violation unless it's
been formatted with `opa fmt` (the rego.v1 keyword is permitted but not added).

When formatting, a file expected to be v1 based on the configuration, but with
v0 syntax is still formatted as `opa fmt –-v0-v1`. Please see
[Configuring Rego Version](https://www.openpolicyagent.org/projects/regal#configuring-rego-version)
for more configuration help for multi version projects.

## Configuration Options

This linter rule provides the following configuration options:

```yaml
rules:
  style:
    opa-fmt:
      # one of "error", "warning", "ignore"
      level: error
```

## Related Resources

- OPA Docs: [CLI Reference `opa fmt`](https://www.openpolicyagent.org/docs/cli/#opa-fmt)
- GitHub: [Source Code](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/rules/style/opa-fmt/opa_fmt.rego)
