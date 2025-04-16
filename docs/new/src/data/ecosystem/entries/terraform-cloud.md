---
title: Terraform Cloud
software:
- terraform
- terraform-cloud
- aws
- gcp
- azure
labels:
  category: publiccloud
  layer: orchestration
tutorials:
- https://developer.hashicorp.com/terraform/cloud-docs/policy-enforcement/opa
- https://developer.hashicorp.com/terraform/tutorials/cloud/drift-and-opa
- https://developer.hashicorp.com/terraform/cloud-docs/policy-enforcement/opa/vcs
videos:
- title: 'Terraform Cloud Learn Lab: Validate Infrastructure and Enforce OPA Policies'
  speakers:
  - name: Rita Sokolova
    organization: HashiCorp
  - name: Cole Morrison
    organization: HashiCorp
  venue: HashiConf Europe 2022
  link: https://www.youtube.com/watch?v=jO2CiYMPxFE
inventors:
- hashicorp
docs_features:
  terraform:
    note: |
      Terraform cloud has native support for enforcing Rego policy on plans.
      The feature is [documented here](https://developer.hashicorp.com/terraform/cloud-docs/policy-enforcement/opa).
---
Policies are rules that Terraform Cloud enforces on runs.
You use the Rego policy language to write policies for the
Open Policy Agent (OPA) framework.

