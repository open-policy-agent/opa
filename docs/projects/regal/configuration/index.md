---
sidebar_position: 4
---


# Configuration

A custom configuration file may be used to override the [default configuration](https://github.com/open-policy-agent/regal/blob/main/bundle/regal/config/provided/data.yaml)
options provided by Regal. The most common use case for this is to change the severity level of a rule. These three
levels are available:

- `ignore` — disable the rule entirely
- `warning` — report the violation without changing the exit code of the lint command
- `error` — report the violation and have the lint command exit with a non-zero exit code (default)

Additionally, some rules may have configuration options of their own. See the documentation page for a rule to learn
more about it.

**.regal/config.yaml** or **.regal.yaml**

```yaml
rules:
  style:
    todo-comment:
      # don't report on todo comments
      level: ignore
    line-length:
      # custom rule configuration
      max-line-length: 100
      # warn on too long lines, but don't fail
      level: warning
    opa-fmt:
      # not needed as error is the default, but
      # being explicit won't hurt
      level: error
      # files can be ignored for any individual rule
      # in this example, test files are ignored
      ignore:
        files:
        - "*_test.rego"
  custom:
    # custom rule configuration
    naming-convention:
      level: error
      conventions:
      # ensure all package names start with "acmecorp" or "system"
      - pattern: '^acmecorp\.[a-z_\.]+$|^system\.[a-z_\.]+$'
        targets:
        - package

capabilities:
  from:
    # optionally configure Regal to target a specific version of OPA
    # this will disable rules that has dependencies to e.g. built-in
    # functions or features not supported by the given version
    #
    # if not provided, Regal will use the capabilities of the latest
    # version of OPA available at the time of the Regal release
    engine: opa
    version: v0.58.0

ignore:
  # files can be excluded from all lint rules according to glob-patterns
  files:
  - file1.rego
  - "*_tmp.rego"

project:
  roots:
  # declares the 'main' and 'lib/jwt' directories as project roots
  - main
  - lib/jwt
  # may also be provided as an object with additional options
  - path: lib/legacy
    rego-version: 0
```

Regal will automatically search for a configuration file (`.regal/config.yaml`
or `.regal.yaml`) in the current directory, and if not found, traverse the
parent directories either until either one is found, or the top of the directory
hierarchy is reached. If no configuration file is found, and no file is found at
`~/.config/regal/config.yaml` either, Regal will use the default configuration.

A custom configuration may be also be provided using the `--config-file`/`-c`
option for `regal lint`, which when provided will be used to override the
default configuration.

## User-level Configuration

Generally, users will want to commit their Regal configuration file to the repo
containing their Rego source code. This allows configurations to be shared
among team members and makes the configuration options available to Regal when
running as a [CI linter](https://openpolicyagent.org/projects/regal/cicd) too.

Sometimes however it can be handy to have some user defaults when a project
configuration file is not found, hasn't been created yet or is not applicable.

In such cases Regal will check for a configuration file at
`~/.config/regal/config.yaml` instead.
