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

GOPAL is a library of Rego policies that encode AI-governance regulations as executable allow/deny checks. It covers the EU AI Act, the UK pro-innovation principles together with the automated decision-making regime in UK GDPR Articles 22A to 22D, the NIST AI Risk Management Framework, ICAO, FAA and EASA aviation standards, PRA SS1/23 and the FCA Consumer Duty in financial services, FERPA and COPPA in education, and professional-conduct rules for AI use in legal practice.

The project publishes per-article coverage matrices stating which obligations are implemented and which are not, and every obligation in those matrices is currently implemented. Each framework is versioned under `v1/` with semver guarantees, so an amended regulation ships as `v2/` alongside the old reading rather than breaking a pinned deployment. Every policy carries allow/deny tests, and `opa check`, Regal and `opa test` run in CI.

The policies are plain Rego with no wrapper on top, so they load into any OPA deployment: `opa eval` in a CI job, Conftest, or a running OPA server queried on the request path. Organizations can keep proprietary rules in a git-ignored `custom/` tree that evaluates alongside the public set, which lets internal AI use-case policy sit next to the regulatory baseline without forking.

AICertify is the companion Python framework. It captures the input context for an AI application, evaluates it against GOPAL policies through OPA, and produces audit-ready PDF, Markdown, and JSON reports. In both paths OPA is the decision engine, which brings the policy-as-code workflow the OPA community already uses for Kubernetes admission and cloud authorization to AI compliance.
