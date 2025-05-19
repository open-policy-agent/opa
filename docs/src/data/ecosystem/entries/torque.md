---
title: Torque
labels:
  category: Environments as a Service
  layer: Infrastructure
  type: poweredbyopa
software:
- terraform
- helm
- cloudformation
- kubernetes
- ansible
- aws
- gcp
- azure
tutorials:
- https://docs.qtorque.io/governance/policies
code:
- https://github.com/QualiTorque/opa
inventors:
- quali
blogs:
- https://www.quali.com/blog/enforcing-open-policy-agent-guardrails-across-your-cloud-configurations/
docs_features:
  cli-integration:
    note: |
      Torque policy evaluation is done using the OPA CLI. See an
      [example command](https://github.com/QualiTorque/opa#perform-policy-evaluation)
      in the documentation.
  terraform:
    note: |
      Torque supports Terraform policy enforcement and defines some
      [sample policies here](https://github.com/QualiTorque/opa).
---
Torque by Quali is a cloud-based platform that provides infrastructure automation and orchestration solutions for digital transformation and DevOps initiatives. Troque utilizes Open Policy Agent (OPA) to enforce policy-as-code, enabling users to define and automate their own security, compliance, and governance policies across their infrastructure.
