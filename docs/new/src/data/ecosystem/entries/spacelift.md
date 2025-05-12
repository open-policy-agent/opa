---
title: Spacelift
labels:
  category: Infrastructure as Code
  layer: cicd
software:
- terraform
- pulumi
- cloudformation
- kubernetes
- ansible
- aws
- gcp
- azure
tutorials:
- https://docs.spacelift.io/concepts/policy
code:
- https://github.com/spacelift-io/spacelift-policies-example-library
inventors:
- spacelift
blogs:
- https://spacelift.io/blog/what-is-open-policy-agent-and-how-it-works
docs_features:
  rego-language-embedding:
    note: |
      Spacelift supports Rego as a language to describe policies for IaC
      resources. View the docs on
      [creating Rego policies](https://docs.spacelift.io/concepts/policy/).
  terraform:
    note: |
      Spacelift supports Rego as a language to describe policies for Terraform
      JSON plans.
      [This blog](https://spacelift.io/blog/what-is-open-policy-agent-and-how-it-works)
      outlines how the integration works.
  kubernetes:
    note: |
      Spacelift supports Rego as a language to describe policies for various
      resource types, including Kubernetes. View the
      [policy documentation](https://docs.spacelift.io/concepts/policy/) for
      more information.
---
Spacelift is a sophisticated CI/CD platform for Infrastructure as Code including Terraform, Pulumi, CloudFormation, Kubernetes, and Ansible. Spacelift utilizes Open Policy Agent to support a variety of policy types within the platform and Policy as Code for secure and compliance Infrastructure as Code.
