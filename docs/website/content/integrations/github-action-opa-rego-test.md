---
title: GitHub Action for OPA Rego Test
subtitle: GitHub Action to automate testing OPA Rego policies
labels:
  category: library
  type: GitHub Action
inventors:
- masterpoint
code:
- https://github.com/masterpointio/github-action-opa-rego-test
tutorials:
- https://github.com/masterpointio/github-action-opa-rego-test/blob/main/README.md
docs_features:
  policy-testing:
    note: |
      [GitHub Action for OPA Rego Policy Tests](docs/website/content/integrations/rego-test-assertions.md) automates testing for your OPA (Open Policy Agent) Rego policies, generates a report with coverage information, and posts the test results as a comment on your pull requests, making it easy for your team to review and approve policies.
---

[GitHub Action for OPA Rego Policy Tests](docs/website/content/integrations/rego-test-assertions.md) by [Masterpoint](https://masterpoint.io/) is used to automate testing for your OPA (Open Policy Agent) Rego policies, generates a report with coverage information, and posts the test results as a comment on your pull requests, making it easy for your team to review and approve policies.

Use this to test your OPA Rego files for [Spacelift policies](https://docs.spacelift.io/concepts/policy), [Kubernetes Admission Controller policies](https://www.openpolicyagent.org/docs/latest/kubernetes-introduction/), [Docker authorization policies](https://www.openpolicyagent.org/docs/latest/docker-authorization/), or any other use case that uses [Open Policy Agent's policy language Rego](https://www.openpolicyagent.org/docs/latest/). This Action also updates PR comments with the test results in place to prevent duplication.
