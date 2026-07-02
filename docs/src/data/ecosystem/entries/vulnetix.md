---
title: Vulnetix
subtitle: Unified CLI security scanner powered by Rego policies
labels:
  category: security
  layer: application
  type: poweredbyopa
software:
- github
- kubernetes
- docker
- terraform
inventors:
- vulnetix
code:
- https://github.com/vulnetix/cli
tutorials:
- https://www.vulnetix.com/features/code-scanner
- https://docs.cli.vulnetix.com/docs/getting-started/
docs_features:
  policy-testing:
    note: |
      Vulnetix ships with 250+ built-in Rego rules spanning SCA, IaC, container,
      secrets, SAST, license and SBOM checks, and lets teams extend coverage
      with their own policy-as-code rule repositories via the `--rule` flag.
      See the
      [Code Scanner docs](https://docs.cli.vulnetix.com/docs/sast-rules/custom-rules/)
      for details.
---

Vulnetix is a unified CLI security scanner that uses Rego to evaluate findings
across Software Composition Analysis (SCA), Infrastructure as Code (IaC),
containers, secrets, Static Application Security Testing (SAST), license
compliance, and SBOM generation for 35+ ecosystems.

Policy-as-code is a first-class concern: detections, severity thresholds, and
supply-chain controls (such as `--block-malware`, `--block-unpinned`,
`--version-lag`, and `--cooldown`) are expressed as Rego rules, so organizations
can tune or replace the built-in rule set to match their own security posture.
Results can be emitted as SARIF, CycloneDX, SPDX, VEX, or token-efficient plain
text for [use in CI](https://www.vulnetix.com/integrations) quality gates and
[AI coding agents](https://ai-docs.vulnetix.com/docs/getting-started/installation/).
