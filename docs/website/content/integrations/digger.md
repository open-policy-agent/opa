---
title: Digger
subtitle: GitOps for Terraform
labels:
  layer: cicd
  category: Infrastructure as Code
software:
- terraform
inventors:
- diggerhq
code:
- https://github.com/diggerhq/digger
tutorials:
- https://docs.digger.dev/readme/introduction
- https://docs.digger.dev/digger-api/rbac-via-opa-guide
- https://docs.digger.dev/configuration/using-opa-conftest
---
Digger is an open-source CI/CD orchestrator for Terraform. It provides role-based access control via OPA, and also integrates Conftest to check Terraform plan output against policies.