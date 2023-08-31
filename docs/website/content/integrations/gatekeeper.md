---
title: OPA Gatekeeper
subtitle: Rego Policy Controller for Kubernetes
labels:
  type: poweredbyopa
  layer: configuration
code:
- https://github.com/open-policy-agent/gatekeeper
software:
- kubernetes
tutorials:
- https://open-policy-agent.github.io/gatekeeper/website/docs/howto
videos:
- https://youtu.be/RMiovzGGCfI?t=1049
- https://youtu.be/6RNp3m_THw4?t=864
docs_features:
  go-integration:
    note: |
      OPA Gatekeeper is written in Go and uses the
      [Rego Go API](https://www.openpolicyagent.org/docs/latest/integration/#integrating-with-the-go-api)
      to evaluate policies loaded from Custom Resources.
  kubernetes:
    note: |
      OPA Gatekeeper integrates with
      [Kubernetes Admission](https://open-policy-agent.github.io/gatekeeper/website/docs/customize-admission/)
      and also uses Custom Resources and the Kubernetes API server to
      store policy state.
allow_missing_image: true
---
Manage Rego Kubernetes admission policies using Custom Resources.
