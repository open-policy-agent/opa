---
sidebar_position: 4
sidebar_label: CLI
---


# CLI

Regal's CLI is the main way to interact with Regal. In order to support
different use cases (Local, CI, etc.) Regal's CLI is designed with a number of
different formats and exit behaviors.

## Output Formats

The `regal lint` command allows specifying the output format by using the `--format` flag. The available output formats
are:

- `pretty` (default) - Human-readable table-like output where each violation is printed with a detailed explanation
- `compact` - Human-readable output where each violation is printed on a single line
- `json` - JSON output, suitable for programmatic consumption
- `github` - GitHub [workflow command](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions)
  output, ideal for use in GitHub Actions. Annotates PRs and creates a
  [job summary](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#adding-a-job-summary)
  from the linter report
- `sarif` - [SARIF](https://sarifweb.azurewebsites.net/) JSON output, for consumption by tools processing code analysis
  reports
- `junit` - JUnit XML output, e.g. for CI servers like GitLab that show these results in a merge request.

## Exit Codes

Exit codes are used to indicate the result of the `lint` command. The `--fail-level` provided for `regal lint` may be
used to change the exit code behavior, and allows a value of either `warning` or `error` (default).

If `--fail-level error` is supplied, exit code will be zero even if warnings are present:

- `0`: no errors were found
- `0`: one or more warnings were found
- `3`: one or more errors were found

This is the default behavior.

If `--fail-level warning` is supplied, warnings will result in a non-zero exit code:

- `0`: no errors or warnings were found
- `2`: one or more warnings were found
- `3`: one or more errors were found

## OPA Check and Strict Mode

OPA itself provides a "linter" of sorts, via the `opa check` command and its `--strict` flag. This checks the provided
Rego files not only for syntax errors, but also for OPA
[strict mode](https://www.openpolicyagent.org/docs/policy-language/#strict-mode) violations. Most of the strict
mode checks from before OPA 1.0 have now been made default checks in OPA, and only two additional checks are currently
provided by the `--strict` flag. Those are both important checks not covered by Regal though, so our recommendation is
to run `opa check --strict` against your policies before linting with Regal.
