---
title: Terraform Policy
software:
- terraform
- aws
- gcp
- azure
labels:
  category: publiccloud
  layer: orchestration
tutorials:
- https://www.openpolicyagent.org/docs/terraform.html
- https://github.com/instrumenta/conftest/blob/master/README.md
code:
- https://github.com/instrumenta/conftest
- https://github.com/fugue/regula
- https://github.com/accurics/terrascan
- https://github.com/Checkmarx/kics
- https://github.com/open-policy-agent/library/tree/master/terraform
- https://github.com/accurics/terrascan/tree/master/pkg/policies/opa/rego
- https://github.com/Checkmarx/kics/tree/master/assets/queries/terraform
blogs:
- https://blog.styra.com/blog/policy-based-infrastructure-guardrails-with-terraform-and-opa
inventors:
- fugue
- accurics
- checkmarx
- medallia
- styra
- docker
- snyk
---
Terraform lets you describe the infrastructure you want and automatically creates, deletes, and modifies your existing infrastructure to match. OPA makes it possible to write policies that test the changes Terraform is about to make before it makes them.
