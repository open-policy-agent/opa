---
title: Sysdig Image Scanner Admission Controller
software:
- kubernetes
- sysdigsecure
labels:
  category: containers
  layer: orchestration
code:
- https://github.com/sysdiglabs/opa-image-scanner
inventors:
- sysdig
docs_feature:
  kubernetes:
    note: |
      Integrates image scanning results with Kubernetes admission checks
      using Rego to make pod admission decisions. See the
      [README](https://github.com/sysdiglabs/opa-image-scanner#overview)
      for more details.
---
Sysdig’s OPA Image Scanner combines Sysdig Secure image scanner with OPA policy-based rego language to evaluate the scan results and the admission context, providing great flexibility on the admission decision.
