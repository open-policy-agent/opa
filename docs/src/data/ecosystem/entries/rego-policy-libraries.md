---
title: Rego Policy Libraries
subtitle: 11,645 executable compliance checks across 54 standards, in Rego v1
labels:
  type: poweredbyopa
  layer: configuration
  category: security
code:
- https://github.com/ynotbhatc/rego_policy_libraries
software:
- kubernetes
- terraform
- ansible
- aws
- azure
- gcp
- docker
inventors:
- ynotbhatc
tutorials:
- https://ynotbhatc.github.io/rego_policy_libraries/
- https://github.com/ynotbhatc/rego_policy_libraries/blob/main/docs/INTEGRATIONS.md
- https://github.com/ynotbhatc/rego_policy_libraries/blob/main/docs/COMPARISON.md
docs_features:
  opa-bundles:
    note: |
      Published as an OPA bundle on every merge and release
      (`ghcr.io/ynotbhatc/rego_policy_libraries:<tag>`) with explicit manifest
      roots, so it composes with a site's own bundles. See
      [INTEGRATIONS.md](https://github.com/ynotbhatc/rego_policy_libraries/blob/main/docs/INTEGRATIONS.md).
  wasm-integration:
    note: |
      Every framework entrypoint is also compiled to WASM per OPA class and
      attached to each release, with SDK notes for JavaScript, Go, Python,
      Java, .NET and Rust under
      [converters/packaging/wasm](https://github.com/ynotbhatc/rego_policy_libraries/tree/main/converters/packaging/wasm).
  kubernetes:
    note: |
      The Pod Security Standards and admission policies ship as generated
      Gatekeeper ConstraintTemplates and Kubewarden policies, each with a
      round-trip test against the original Rego
      ([converters/packaging](https://github.com/ynotbhatc/rego_policy_libraries/tree/main/converters/packaging)).
  terraform:
    note: |
      The Terraform plan gate ships as a Terraform Cloud / Spacelift / env0
      OPA policy set and works with Conftest as-is.
  policy-testing:
    note: |
      1,945 OPA tests run in CI, with a control-level count (`make count`)
      that CI re-verifies against the README on every merge.
---

Rego Policy Libraries is the largest open-source library of executable compliance policy anyone can verify: 740 OPA Rego v1 policy files, 11,645 control-level checks across 54 tracked standards, with the count reproduced by CI on every merge. It covers CIS Benchmarks (with Level 2 profiles), DISA STIGs across 19 platforms, CISA SCuBA M365, NIST SP 800-53, 800-171, CSF and AI RMF, FedRAMP 20x, ISO 27001, SOC 2, PCI DSS, HIPAA, GDPR, CCPA/CPRA, the EU AI Act, DORA, NIS2, NERC-CIP, IEC 62443, and financial and privacy regulators across Asia-Pacific (MAS, APRA, HKMA, BNM, CERT-In, DPDP, METI). Every framework exposes the same fail-closed `compliance_report` entrypoint and carries package-scoped OPA METADATA, so a consumer classifies and loads policies from `opa inspect -a` rather than directory names. It is standalone and dependency-free, Apache 2.0, and runs unchanged in OPA, Enterprise OPA, Conftest, Gatekeeper, Kubewarden, Trivy, KICS and Terraform Cloud; the `converters/` tree ships the same policies as WASM bundles for every language SDK, Gatekeeper ConstraintTemplates, Kubewarden policies, a Conftest layout and Terraform Cloud policy sets, each with a round-trip test. The size comparison against other public libraries is in `docs/COMPARISON.md`.
