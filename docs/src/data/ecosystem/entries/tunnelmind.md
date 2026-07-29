---
title: TunnelMind
labels:
  category: authorization
  layer: network
  datasource: tunnelmind
code:
- https://github.com/TunnelMind/tunnelmind-data-api
- https://github.com/TunnelMind/tunnelmind-sdk-ts
- https://github.com/TunnelMind/tunnelmind-sdk-python
tutorials:
- https://github.com/TunnelMind/tunnelmind-data-api/blob/main/docs/OPA-INTEGRATION.md
docs_features:
  external-data-runtime:
    note: |
      The [OPA integration guide](https://github.com/TunnelMind/tunnelmind-data-api/blob/main/docs/OPA-INTEGRATION.md)
      shows a ~10-line Rego policy calling `http.send` against the cached
      `GET /v1/attributes/{node}` endpoint on the decision path, with
      freshness and per-source coverage gating.
---

TunnelMind is a Policy Information Point for agentic and network traffic:
signed, coverage-honest attribute bundles about internet counterparties
(IPs, domains, ASNs, AI agents and crawlers) that OPA policies consume as
external data. Each bundle labels every source with an explicit coverage
state (observed_clean / never_observed / degraded), carries its own
freshness contract (valid_until, stale_if_error), and ships inside an
Ed25519-signed receipt that verifies offline — so a policy can gate on
facts, their blind spots, and their age rather than on an opaque score.
