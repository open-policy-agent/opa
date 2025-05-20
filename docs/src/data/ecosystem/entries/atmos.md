---
title: Atmos
software:
- aws
- gcp
- terraform
- helm
inventors:
- cloudposse
labels:
  category: Infrastructure as Code
  type: poweredbyopa
tutorials:
- https://atmos.tools/core-concepts/components/validation/#open-policy-agent-opa
code:
- https://github.com/cloudposse/atmos
docs_features:
  terraform:
    note: |
      Atmos can validate Terraform stack before applying them. This is done
      using the `validate component` command
      [documented here](https://atmos.tools/cli/commands/validate/component).
---
Workflow automation tool for DevOps. Keep configuration DRY with hierarchical imports of configurations, inheritance, and WAY more.
