---
title: AGT Policies Africa
subtitle: OPA Policy Pack for African Data Protection and AI Agent Compliance
labels:
  category: security
  type: poweredbyopa
code:
- https://github.com/kingztech2019/agt-policies-nigeria
allow_missing_image: true
docs_features:
  policy-testing:
    note: |
      Each jurisdiction ships a full `opa test` suite covering all decision
      outcomes — `deny`, `escalate`, `audit`, and `allow` — for every rule.
      The test suite runs in CI and currently covers 140+ cases across
      9 African jurisdictions.
  learning-rego:
    note: |
      The policies demonstrate real-world compliance patterns including
      regex-based PII detection (national IDs, BVN/NIN biometrics),
      cross-border data transfer controls, breach-notification suppression
      guards, sector-specific transaction-limit enforcement, and
      multi-rule decision aggregation — useful reference implementations
      for anyone writing regulatory compliance policies in Rego.
---

AGT Policies Africa is an OPA policy pack for AI agents operating under African
data protection and financial-sector regulations. It provides ready-to-deploy
Rego policies and YAML `PolicyDocument` definitions for 9 jurisdictions:

- **Nigeria** — NDPA 2023, CBN NIP transaction limits, NFIU AML/CFT, BVN/NIN protection
- **Kenya** — Data Protection Act 2019
- **Ghana** — Data Protection Act 2012 (Act 843)
- **Rwanda** — Law No. 058/2021 on Personal Data Protection
- **Egypt** — Personal Data Protection Law No. 151/2020
- **Mauritius** — Data Protection Act 2017
- **South Africa** — POPIA (Protection of Personal Information Act)
- **Tanzania** — Personal Data Protection Act 2022
- **Uganda** — Data Protection and Privacy Act 2019

Policies enforce controls such as cross-border transfer adequacy checks,
special-category and biometric data guards, breach notification suppression
detection, national ID redaction (NIN, BVN, Ghana Card, Egypt NID, Mauritius NIC),
and sector-specific transaction limits — all returned as structured
`deny / escalate / audit / allow` decisions that AI agent runtimes can act on
directly without parsing error strings.
