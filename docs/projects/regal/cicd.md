---
sidebar_label: CI/CD
sidebar_position: 8
---

<head>
  <title>CI/CD | Regal</title>
</head>

# Using Regal in your build pipeline

It's possible to use Regal to lint your Rego policies in your CI/CD pipeline(s)!

This document will guide you on how to do so. Please also review the
[CLI](./cli) documentation for more information on the available options.

## GitHub Actions

If you'd like to run Regal in GitHub actions, please consider using
[`setup-regal`](https://github.com/open-policy-agent/setup-regal). A simple `.github/workflows/lint.yml` to run regal
on PRs could look like this, where `policy` contains Rego files:

```yaml
name: Regal Lint
on:
  pull_request:
jobs:
  lint-rego:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    - uses: open-policy-agent/setup-regal@v1
      with:
        # For production workflows, use a specific version, like v0.22.0
        version: latest

    - name: Lint
      run: regal lint --format=github ./policy
```

Please see [`setup-regal`](https://github.com/open-policy-agent/setup-regal) for more information.

## GitLab CI/CD

To use Regal in GitLab CI/CD, you could for example use the following stage in your `.gitlab-ci.yml`:

```yaml
regal_lint_policies:
  stage: regal-lint
  image:
    # For production workflows, use a specific version, like v0.22.0
    name: ghcr.io/open-policy-agent/regal:latest
    entrypoint: ['/bin/sh', '-c']
  script:
    - regal lint ./policy --format junit > regal-results.xml
  artifacts:
    reports:
      junit: regal-results.xml
    when: always
  rules:
    - if: '$CI_PIPELINE_SOURCE == "merge_request_event"'
```

The above will run Regal on the `policy` directory when a merge request is created or updated and will show linting
violations as part of the merge request.
