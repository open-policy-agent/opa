---
sidebar_position: 6
sidebar_label: Pre-Commit Hooks
---

<head>
  <title>Pre-Commit Hooks | Regal</title>
</head>

# Pre-Commit Hooks

[Pre-Commit](https://pre-commit.com) is a framework for managing and maintaining multi-language pre-commit hooks.
This allows running Regal automatically whenever (and as the name implied before) a Rego file is about to be committed.

To use Regal with pre-commit, add this to your `.pre-commit-config.yaml`

```yaml
- repo: https://github.com/open-policy-agent/regal
  rev: v0.7.0 # Use the ref you want to point at
  hooks:
    - id: regal-lint
  # -   id: ...
```

## Hooks Available

### `regal-lint`

![commit-msg hook](https://img.shields.io/badge/hook-pre--commit-informational?logo=git)

Runs Regal against all staged `.rego` files, aborting the commit if any fail.

- requires the `go` build chain is installed and available on `$PATH`
- will build and install the tagged version of Regal in an isolated `GOPATH`
- ensures compatibility between versions

### `regal-lint-use-path`

![commit-msg hook](https://img.shields.io/badge/hook-pre--commit-informational?logo=git)

Runs Regal against all staged `.rego` files, aborting the commit if any fail.

- requires the `regal` package is already installed and available on `$PATH`.

### `regal-download`

![commit-msg hook](https://img.shields.io/badge/hook-pre--commit-informational?logo=git)

Runs Regal against all staged `.rego` files, aborting the commit if any fail.

- Downloads the latest `regal` binary from Github.

### `regal-fix`

![commit-msg hook](https://img.shields.io/badge/hook-pre--commit-informational?logo=git)

Runs `regal fix` against all staged `.rego` files, applying any
auto-fixable rule violations in place. Use this alongside `regal-lint` when you
want the hook to repair style-level issues automatically rather than asking the
contributor to re-run `regal fix` themselves.

- requires the `go` build chain is installed and available on `$PATH`
- will build and install the tagged version of Regal in an isolated `GOPATH`
- ensures compatibility between versions

### `regal-fix-use-path`

![commit-msg hook](https://img.shields.io/badge/hook-pre--commit-informational?logo=git)

Same as `regal-fix`, but uses the `regal` binary already on `$PATH`.

- requires the `regal` package is already installed and available on `$PATH`.

### `regal-fix-download`

![commit-msg hook](https://img.shields.io/badge/hook-pre--commit-informational?logo=git)

Same as `regal-fix`, but downloads the latest `regal` binary from GitHub instead of building or relying on `$PATH`.

- Downloads the latest `regal` binary from Github.
