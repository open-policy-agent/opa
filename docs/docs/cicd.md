---
sidebar_label: CI/CD
---

# Using OPA in CI/CD Pipelines

OPA is a great tool for implementing policy-as-code guardrails in
<abbr title="continuous integration/continuous deployment">CI/CD</abbr>
pipelines. With OPA, you can automatically verify configurations, validate
outputs, and enforce organizational policies before code reaches production. OPA
serves as a powerful 'swiss army knife' for implementing custom checks required
by your organization that might be difficult to implement in a script or in
another tool.

For users looking to parse and validate configuration files or Infrastructure as
Code (IaC) committed to git, [Conftest](https://www.conftest.dev) is typically
the better choice as it supports many file formats (HCL, Jsonnet etc.).
However, OPA's `eval` command excels at connecting other tools and making checks
against runtime data, as it can only parse JSON and YAML formats.

OPA as a CLI tool provides powerful capabilities for testing and validating
various types of data in your continuous integration workflows:

- **Repository governance** - Use OPA to call GitHub APIs to validate commit
  message formats and pull request metadata compliance.
- **Software supply chain validation** - Check package manager configurations
  (package.json, requirements.txt, go.mod) to ensure dependencies meet security
  and licensing requirements.
- **Test selection** - Determine which tests to run based on the
  files that changed, optimizing CI runtime by only executing relevant test
  suites based on changed files.
- **Checking JSON test coverage reports** - Ensure test coverage meets minimum
  thresholds or that benchmarks results are within acceptable limits.
- **Defining dependencies between jobs** - Validate that deployment pipelines
  follow proper sequencing and dependency requirements. An example of this can
  be found in the OPA Repo's [own PR checks](https://github.com/open-policy-agent/opa/blob/aee10e4a8deef80f3110237426a64fa5d4e229de/.github/workflows/pull-request.yaml#L476-L521).
- **Test coverage enforcement** - Check that test files are added when code
  files are created (e.g., ensuring each `foo.js` has a corresponding
  `foo_test.js` in the appropriate directory).

The [`opa eval`](./cli#eval) command provides
several flags that are particularly useful for CI/CD scenarios:

- `--fail` and `--fail-defined` - Set the exit code to 1 based on query results
  (`--fail` when undefined or false, `--fail-defined` when defined), making it
  easy to fail CI jobs when policies are violated
- `--stdin-input` - Reads input data from stdin, allowing you to pipe output
  from other commands directly into OPA for evaluation
- `-d` - load in JSON or YAML data files for evaluation.

These flags help ensure your CI/CD pipelines respond appropriately to policy evaluation results and integrate smoothly with other tools in your pipeline.

## Guardrail Pattern: Stable Input Contract + Deny Reasons

For CI/CD guardrails, a useful pattern is to define:

1. A stable input shape (for example `changed_files`, `attestations`, and
   `metadata`), and
2. A policy that returns a set of deny messages.

This makes policies easier to compose and easier to debug in pipeline logs.

```rego title="policy.rego"
package cicd.guardrails

import rego.v1

required_attestations := {"prompt", "eval"}

deny contains msg if {
  some f in input.changed_files
  startswith(f, "prompts/")
  missing := required_attestations - {a | some a in input.attestations}
  count(missing) > 0
  msg := sprintf("missing required attestations for prompt changes: %v", [sort(missing)])
}

deny contains "unapproved plaintext payload detected" if {
  some a in input.attestations
  a == "plaintext_explicit"
  not input.metadata.allow_plaintext
}
```

```json title="input.json"
{
  "changed_files": ["prompts/system.txt"],
  "attestations": ["prompt"],
  "metadata": {
    "allow_plaintext": false
  }
}
```

```bash title="pipeline command"
opa eval -d policy.rego -i input.json --fail-defined 'data.cicd.guardrails.deny'
```

If the query is defined, OPA returns exit code `1` (because of
`--fail-defined`), and your pipeline can fail with actionable deny messages.

## GitHub Actions Integration

For GitHub users, the easiest way to get started is using the official OPA setup
action. This will make the `opa` command available in your workflow, allowing
you to run OPA policies against your codebase.

```yaml title="OPA installation step"
- name: Download OPA
  uses: open-policy-agent/setup-opa
  with:
    version: latest # install the latest version
```

```yaml title="Example workflow checking test coverage"
name: OPA Checks
on: [pull_request]

jobs:
  validate-configs:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v6

    - name: Download OPA
      uses: open-policy-agent/setup-opa
      with:
        version: edge

    - name: Check tests results coverage remains above 70%
      run: |
        my test command |
          opa eval --fail-defined \
          --stdin-input \
          'input.results[_].coverage < 0.7'
```

Here's some examples of how we use these actions in our own CI/CD pipelines for OPA!

- [Pull Request Workflow File](https://github.com/open-policy-agent/opa/blob/main/.github/workflows/pull-request.yaml)
- [PR Where Action was Introduced](https://github.com/open-policy-agent/opa/pull/8183/files#diff-a8619735ff14304aa0514284f86ff5145b0a6bae2e76a37faeb0ad899a3d8db4R27-R30)

## Other CI/CD Platforms

For users of other CI/CD platforms (GitLab CI, Jenkins, Azure DevOps, etc.), you
can download OPA directly from the [official installation page](../docs?current-os=linux#1-download-opa).
This provides installation instructions for various operating systems and
package managers.
