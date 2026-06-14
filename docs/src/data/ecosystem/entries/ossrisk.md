---
title: ossrisk
subtitle: Dependency supply-chain risk scanner with Rego policy gates
labels:
  category: security
  layer: cicd
  type: poweredbyopa
inventors:
- depkeep
code:
- https://github.com/depkeep/ossrisk
- https://www.npmjs.com/package/ossrisk
tutorials:
- https://github.com/depkeep/ossrisk#policy-as-code-opa
- https://github.com/depkeep/ossrisk/blob/main/examples/policies/supply-chain.rego
software:
- npm
- pypi
- github-actions
---

ossrisk scans npm and PyPI dependency trees for supply-chain and long-term
viability risk: known CVEs (via OSV.dev), end-of-life versions, abandonment
signals, typosquatting, license compliance, maintainer-takeover patterns, and
install scripts.

Beyond a simple `--fail-on <severity>` threshold, ossrisk delegates gating
decisions to OPA. The scan result is passed to `opa eval` as `input`; policies
live in `package ossrisk` and add human-readable messages to a `deny` set, and
any violation fails the scan. This enables cross-signal rules a severity
threshold cannot express - for example "no strong-copyleft licenses in direct
dependencies" or "block packages that add install scripts under a brand-new
publisher" (the event-stream takeover pattern). The JSON output is a stable
contract, so it also works piped into conftest.
