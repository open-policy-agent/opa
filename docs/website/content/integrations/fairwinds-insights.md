---
title: Fairwinds Insights Configuration Validation Software
labels:
  category: kubernetes
  layer: cicd
inventors:
- fairwinds
software:
- kubernetes
- docker
- helm
tutorials:
- https://insights.docs.fairwinds.com/features/policy/
- https://insights.docs.fairwinds.com/reports/opa/
- https://insights.docs.fairwinds.com/features/admission-controller/
- https://insights.docs.fairwinds.com/features/continuous-integration/
videos:
- https://youtu.be/kmvPYjx1bpU
- https://youtu.be/gxE_Tkj6d40
blogs:
- https://www.fairwinds.com/blog/managing-opa-policies-with-fairwinds-insights
- https://www.fairwinds.com/blog/manage-open-policy-agent-opa-consistently
- https://www.fairwinds.com/blog/kubernetes-multi-cluster-visibility-why-how-to-get-it
- https://www.fairwinds.com/blog/what-is-kubernetes-policy-as-code
- https://www.fairwinds.com/blog/why-kubernetes-policy-enforcement
- https://www.fairwinds.com/blog/an-interview-with-flatfile-on-why-fairwinds-insights-kubernetes-configuration-validation
docs_features:
  kubernetes:
    note: |
      Implements auditing and admission checking of Kubernetes resources
      using Rego policy using
      [Polaris](https://github.com/FairwindsOps/Polaris).
allow_missing_image: true
---
Automate, monitor and enforce OPA policies with visibility across multiple clusters and multiple teams. It ensures the same policies are applied across all your clusters and gives some flexibility if you want certain policies to apply to only certain workloads. Run the same policies in CI/CD, Admission Control, and In-cluster scanning to apply policy consistently throughout the development and deployment process.
