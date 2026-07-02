---
title: KubeStellar Console
subtitle: Multi-cluster Kubernetes dashboard with OPA Gatekeeper policy management and AI-driven compliance scoring
software:
- kubernetes
labels:
  category: kubernetes
  layer: application
inventors:
- kubestellar
code:
- https://github.com/kubestellar/console
tutorials:
- https://console.kubestellar.io/security-posture?card=opa_policies
- https://console.kubestellar.io/missions/install-open-policy-agent-opa
blogs:
- https://kubestellar.io/docs/news/ai-maintained-codebase
docs_features:
  kubernetes:
    note: |
      KubeStellar Console provides a unified multi-cluster dashboard with
      native OPA Gatekeeper integration. It discovers constraint templates
      and constraints across all connected clusters, displays policy
      violations in real time, and lets operators create new Rego policies
      using an AI-assisted workflow that generates ConstraintTemplate and
      Constraint YAML from plain-English descriptions.

      The [AI Cloud Maturity Model (ACMM)](https://arxiv.org/abs/2604.09388)
      built into the console scores clusters across 8 dimensions — including
      a policy-as-code dimension that checks for OPA/Gatekeeper, Kyverno,
      and Conftest artifacts — producing a quantified maturity score with a
      public [leaderboard](https://console.kubestellar.io/leaderboard).

      The Fleet Compliance Heatmap card aggregates OPA Gatekeeper status
      alongside Kyverno, Trivy, Kubescape, Falco, and Compliance Trestle
      into a single cross-cluster compliance view.
---

KubeStellar Console is a CNCF Sandbox multi-cluster Kubernetes dashboard that
provides fleet-wide visibility into OPA Gatekeeper policies and violations. It
connects to any number of clusters and aggregates constraint templates,
constraints, and violations into a single pane of glass.

Key OPA integration features:

- **OPA Policies card** — lists all Gatekeeper constraint templates per cluster
  with violation counts and enforcement mode (audit/warn/deny)
- **Fleet Compliance Heatmap** — visualizes OPA Gatekeeper health across every
  cluster alongside other compliance tools
- **AI-assisted policy creation** — describe a policy in plain English and the
  console generates the ConstraintTemplate and Constraint YAML, ready to apply
- **Per-cluster drill-down** — inspect individual violations with full resource
  context, namespace, and remediation guidance
- **ACMM scoring** — the AI Cloud Maturity Model evaluates each cluster's
  policy-as-code posture, checking for OPA/Gatekeeper artifacts as part of an
  8-dimension maturity assessment ([paper](https://arxiv.org/abs/2604.09388))
- **Guided install mission** — a built-in [AI mission](https://console.kubestellar.io/missions/install-open-policy-agent-opa)
  walks operators through installing and configuring OPA Gatekeeper on any
  connected cluster, with preflight checks and cluster selection
