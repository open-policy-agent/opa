---
sidebar_position: 6
---


# Ignoring Rules

If one of Regal's rules doesn't align with your team's preferences, don't worry! Regal is not meant to be the law,
and some rules may not make sense for your project, or parts of it.
Regal provides several different methods to ignore rules with varying precedence.
The available methods are (ranked highest to lowest precedence):

- [Inline Ignore Directives](#inline-ignore-directives) cannot be overridden by any other method.
- Enabling or Disabling Rules with CLI flags.
  - Enabling or Disabling Rules with `--enable` and `--disable` CLI flags.
  - Enabling or Disabling Rules with `--enable-category` and `--disable-category` CLI flags.
  - Enabling or Disabling All Rules with `--enable-all` and `--disable-all` CLI flags.
  - See [Ignoring Rules via CLI Flags](#ignoring-rules-via-cli-flags) for more details.
- [Ignoring a Rule In Config](#ignoring-a-rule-in-config)
- [Ignoring a Category In Config](#ignoring-a-category-in-config)
- [Ignoring All Rules In Config](#ignoring-all-rules-in-config)

In summary, the CLI flags will override any configuration provided in the file, and inline ignore directives for a
specific line will override any other method.

It's also possible to ignore messages on a per-file basis. The available methods are (ranked High to Lowest precedence):

<!-- markdownlint-disable MD051 -->

- Using the `--ignore-files` CLI flag.
  See [Ignoring Rules via CLI Flags](#ignoring-rules-via-cli-flags).
- [Ignoring Files Globally](#ignoring-files-globally) or
  [Ignoring a Rule in Some Files](#ignoring-a-rule-in-some-files).

## Ignoring a Rule in Config

If you want to ignore a rule, set its level to `ignore` in the configuration file:

```yaml
rules:
  style:
    prefer-snake-case:
      # At example.com, we use camel case to comply with our naming conventions
      level: ignore
```

## Ignoring a Category in Config

If you want to ignore a category of rules, set its default level to `ignore` in the configuration file:

```yaml
rules:
  style:
    default:
      level: ignore
```

## Ignoring All Rules in Config

If you want to ignore all rules, set the default level to `ignore` in the configuration file:

```yaml
rules:
  default:
    level: ignore
  # then you can re-enable specific rules or categories
  testing:
    default:
      level: error
  style:
    opa-fmt:
      level: error
```

**Tip**: providing a comment on ignored rules is a good way to communicate why the decision was made.

## Ignoring a Rule in Some Files

You can use the `ignore` attribute inside any rule configuration to provide a list of files, or patterns, that should
be ignored for that rule:

```yaml
rules:
  style:
    line-length:
      level: error
      ignore:
        files:
        # ignore line length in test files to accommodate messy test data
        - "*_test.rego"
        # specific file used only for testing
        - "scratch.rego"
```

## Ignoring Files Globally

**Note**: Ignoring files will disable most language server features
for those files. Only formatting will remain available.
Ignored files won't be used for completions, linting, or definitions
in other files.

If you want to ignore certain files for all rules, you can use the global ignore attribute in your configuration file:

```yaml
ignore:
  files:
  - file1.rego
  - "*_tmp.rego"
```

## Inline Ignore Directives

If you'd like to ignore a specific violation in a file, you can add an ignore directive above the line in question, or
alternatively on the same line to the right of the expression:

```rego
package policy

# regal ignore:prefer-snake-case
camelCase := "yes"

list_users contains user if { # regal ignore:avoid-get-and-list-prefix
    some user in data.db.users
    # ...
}
```

The format of an ignore directive is `regal ignore:<rule-name>,<rule-name>...`, where `<rule-name>` is the name of the
rule to ignore. Multiple rules may be added to the same ignore directive, separated by commas.

Note that at this point in time, Regal only considers the same line or the line following the ignore directive, i.e. it
does not apply to entire blocks of code (like rules, functions or even packages). See [configuration](#configuration)
if you want to ignore certain rules altogether.

## Ignoring Rules via CLI Flags

For development and testing, rules or classes of rules may quickly be enabled or disabled using the relevant CLI flags
for the `regal lint` command:

- `--disable-all` disables **all** rules
- `--disable-category` disables all rules in a category, overriding `--enable-all` (may be repeated)
- `--disable` disables a specific rule, overriding `--enable-all` and `--enable-category` (may be repeated)
- `--enable-all` enables **all** rules
- `--enable-category` enables all rules in a category, overriding `--disable-all` (may be repeated)
- `--enable` enables a specific rule, overriding `--disable-all` and `--disable-category` (may be repeated)
- `--ignore-files` ignores files using glob patterns, overriding `ignore` in the config file (may be repeated)

**Note:** all CLI flags override configuration provided in file.
