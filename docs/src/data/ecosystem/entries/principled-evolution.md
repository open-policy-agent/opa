---
title: Principled Evolution (GOPAL & AICertify)
software:
- gopal
- aicertify
inventors:
- principled-evolution-ltd
code:
- https://github.com/principled-evolution/gopal
- https://github.com/principled-evolution/aicertify
---

GOPAL is a library of 85 Rego policies that encode AI-governance regulations as executable allow/deny checks: the EU AI Act, NIST AI RMF, ICAO, FAA and EASA aviation standards, FERPA and COPPA in education, fair lending in banking, plus healthcare and automotive verticals. Each framework is versioned under `v1/` with semver guarantees, so an amended regulation ships alongside the old one rather than breaking pinned users. Every policy has allow/deny tests, and `opa check` and Regal run in CI.

The policies are plain Rego with no wrapper on top, so they load into any OPA deployment: `opa eval` in a CI job, Conftest, or a running OPA server. Organizations can keep proprietary rules in a git-ignored `custom/` tree that evaluates alongside the public set, which lets internal AI use-case policy live next to the regulatory baseline without forking.

AICertify is the companion Python framework. It captures the input context for an AI application, evaluates it against GOPAL policies through OPA, and produces audit-ready PDF, Markdown, and JSON reports. In both paths OPA is the decision engine, which brings the policy-as-code workflow the OPA community already uses for Kubernetes admission and cloud authorization to AI compliance.
