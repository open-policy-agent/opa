---
title: Kubescape
subtitle: Kubernetes security posture scanner
labels:
  category: security
  layer: application
software:
- kubescape
code:
- https://github.com/kubescape/kubescape
- https://github.com/kubescape/regolibrary
inventors:
- armo
tutorials:
- https://hub.armosec.io/docs
docs_features:
  go-integration:
    note: |
      Kubescape uses the Go Repo API to test Kubernetes objects against
      a range of posture controls.
---
This integration uses OPA for defining security controls over Kubernetes clusters. Kubescape is a simple extensible tool
finding security problems in your environment. OPA enables Kubescape to implement and extend very fast to answer new problems.

