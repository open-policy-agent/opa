---
title: Scalr
subtitle: Policy enforcement for Terraform
labels:
  category: Infrastructure as Code
  layer: cicd
software:
- terraform
tutorials:
- https://iacp.docs.scalr.com/en/latest/working-with-iacp/opa.html#creating-the-opa-policy
code:
- https://github.com/Scalr/sample-tf-opa-policies
inventors:
- scalr
blogs:
- https://www.scalr.com/blog/opa-is-to-policy-automation-as-terraform-is-to-iac/
docs_features:
  cli-integration:
    note: |
      These policies can be run using OPA at the command line against a
      Terraform plan JSON. See
      [the example](https://github.com/Scalr/sample-tf-opa-policies#policy-evaluation)
      in the README.
  terraform:
    note: |
      These policies can be run using OPA at the command line against a
      Terraform plan JSON. See
      [the example](https://github.com/Scalr/sample-tf-opa-policies#policy-evaluation)
      in the README.
---
Scalr allows teams to easily collaborate on Terraform through its pipeline that runs all Terraform operations, policy checks, and stores state. Scalr uses OPA to check the auto-generated Terraform JSON plan to ensure that it meets your organization standards prior to an apply.
