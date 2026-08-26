---
title: Scanara EU AI Act Compliance Scanning
labels:
  category: compliance
tutorials:
- https://scanara.io/en/methodology/
inventors:
- scanara
docs_features:
  document-compliance-scanning:
    note: |
      Scanara uses OPA to evaluate technical documentation against EU AI Act obligations. Rego policies encode 22 document-level requirements (Annex IV technical documentation, Annex III risk-classification evidence, GPAI/systemic-risk obligations), running alongside a complementary Semgrep-based source-code scan. Findings from both engines are mapped to specific EU AI Act articles and feed an automated compliance dossier generator.
---

Scanara is an automated EU AI Act conformity platform for AI system providers and deployers. This integration uses OPA/Rego to evaluate uploaded technical documentation for EU AI Act compliance, turning document-level regulatory obligations into executable policy checks alongside Scanara's source-code scanning.
