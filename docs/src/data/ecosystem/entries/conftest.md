---
title: Conftest
subtitle: Rego policy for configuration files
labels:
  type: poweredbyopa
  layer: configuration
code:
- https://github.com/open-policy-agent/conftest
software:
- kustomize
- terraform
- aws
- toml
- docker
tutorials:
- https://www.conftest.dev
- https://www.conftest.dev/examples/
videos:
- title: Applying Policy Throughout the Application Lifecycle with Open Policy Agent
  speakers:
  - name: Gareth Rushgrove
    organization: snyk
  venue: Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=cXfsaE6RKfc
- title: 'Terraform Code Reviews: Supercharged with Conftest'
  speakers:
  - name: Jay Wallace
    organization: doordash
  venue: Hashitalks 2020
  link: https://www.youtube.com/watch?v=ziKT-ZjZ7mM
docs_features:
  policy-testing:
    note: |
      Conftest supports unit testing of policy and has a number of extra language
      features for working with configuration files. The functionality is
      [documented here](https://www.conftest.dev/#testingverifying-policies).
  go-integration:
    note: |
      Conftest is written in Go and uses the
      [Rego Go API](https://www.openpolicyagent.org/docs/latest/integration/#integrating-with-the-go-api).
  opa-bundles:
    note: |
      Conftest supports the loading of policy in a bundle format. The feature
      is [documented here](https://www.conftest.dev/sharing/).
  terraform:
    note: |
      Conftest has generic support for Terraform source files defined in HCL.
      There is an example provided here on
      [GitHub](https://github.com/open-policy-agent/conftest/tree/master/examples/hcl2).
---
Conftest is a utility built on top of OPA to help you write tests against structured configuration data.
