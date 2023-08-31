---
title: Pulumi
software:
- pulumi
- aws
- gcp
- azure
labels:
  category: publiccloud
  layer: orchestration
code:
- https://github.com/pulumi/pulumi-policy-opa
blogs:
- https://www.pulumi.com/blog/opa-support-for-crossguard/
videos:
- title: Testing Configuration with Open Policy Agent
  speakers:
  - name: Gareth Rushgrove
    organization: snyk
  venue: Cloud Engineering Summit 2020
  link: https://www.pulumi.com/resources/testing-configuration-with-open-policy-agent/
inventors:
- pulumi
docs_features:
  go-integration:
    note: |
      The Pulumi OPA bridge uses the Rego API to evaluate policies in a
      policy pack. View the [docs and code](https://github.com/pulumi/pulumi-policy-opa).
---
Build infrastructure as code in familiar languages. CrossGuard is Pulumi's policy as code offering, providing OPA as one of the options to use for defining policy.
