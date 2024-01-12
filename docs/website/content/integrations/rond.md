---
title: Rönd
software:
- rond
labels:
  category: authorization
  layer: application
code:
- https://github.com/rond-authz/rond
tutorials:
- https://github.com/rond-authz/example
videos:
- title: Rönd - The Open Source K8s sidecar that defines security policies over your
    APIs
  speakers:
  - name: Federico Maggi
    organization: mia-platform
  link: https://youtu.be/ubT31NtHV8w
inventors:
- mia-platform
blogs:
- https://mia-platform.eu/blog/rond-open-source-api-enforcement/
- https://mia-platform.eu/blog/role-based-access-control-rbac/
- https://resources.mia-platform.eu/en/white-paper-rbac-platform-engineering
docs_features:
  go-integration:
    note: |
      The Rönd sidecar uses the OPA Rego API to make API-access
      authorization decisions. See the
      [OPA evaluator](https://github.com/rond-authz/rond/blob/4c27fa6a127f68b8670a39c792b0e40dac52dafa/core/opaevaluator.go#L173)
      code.
---
Rönd is a lightweight container that distributes security policy enforcement throughout your application.

