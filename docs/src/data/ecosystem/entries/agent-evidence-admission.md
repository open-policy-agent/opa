---
title: Agent Evidence Admission
subtitle: Rego admission rails for agent execution evidence, measured against a conformance corpus
labels:
  type: poweredbyopa
  category: authorization
  layer: configuration
software:
- kubernetes
code:
- https://github.com/probityai/agent-evidence-admission
- https://github.com/probityai/agent-evidence-vectors
docs_features:
  kubernetes:
    note: |
      A Rego policy admits or refuses a Kubernetes workload on the
      adversarial-execution-evidence in-toto predicate attached to its image,
      and declares per obligation what the engine enforces, what it only
      approximates, and what it cannot reach.
  policy-testing:
    note: |
      `rego/execution_evidence_test.rego` runs the policy over a corpus
      generated from
      [agent-evidence-vectors](https://github.com/probityai/agent-evidence-vectors)
      at tag `v0.10.1`, so the policy and the specification are checked against
      the same bytes. CI refuses a tag that has moved off the commit it is
      pinned to, and refuses any row that declares an obligation enforced while
      the run shows the rail answering differently from the oracle on a vector
      citing it.
allow_missing_image: true
---

Agent Evidence Admission carries four policy engines that decide whether a Kubernetes workload may run, on the strength of an in-toto attestation describing what an automated agent did while producing it. The Rego rail is `rego/execution_evidence.rego`.

The point of the repository is the honesty of its coverage table rather than the policy itself. `PROFILE-REGISTRY.md` records, obligation by obligation, whether each engine enforces it, approximates it, or cannot reach it, and `scripts/profile-map-gate.py` refuses a row that claims enforcement the run does not support. A reader can rebuild every one of those claims from a fresh clone in four commands, which is the property the project is built around: an admission decision nobody can re-derive is a decision you have to take on trust.
