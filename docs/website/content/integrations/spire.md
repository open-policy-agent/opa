---
title: SPIRE
labels:
  layer: network
  category: application
software:
- spiffe
- spire
blogs:
- https://www.styra.com/blog/zero-trust-with-envoy-spire-and-open-policy-agent-opa/
code:
- https://github.com/spiffe/spire/blob/v1.0.2/doc/authorization_policy_engine.md
tutorials:
- https://spiffe.io/docs/latest/microservices/envoy-opa/readme/
- https://spiffe.io/docs/latest/microservices/envoy-jwt-opa/readme/
docs_features:
  envoy:
    note: |
      SPIRE can be used to integrate with Envoy and OPA. See a
      [blog here](https://www.styra.com/blog/zero-trust-with-envoy-spire-and-open-policy-agent-opa/)
      to learn more.
  rest-api-integration:
    note: |
      SPIRE can work in tandem with the Envoy proxy to integrate with the OPA
      REST API. See the
      [tutorial here](https://spiffe.io/docs/latest/microservices/envoy-jwt-opa/readme/).
---
SPIRE is a production-ready implementation of the SPIFFE APIs that performs node and workload attestation in order to securely issue SPIFFE Verifiable Identity Documents (SVIDs) to workloads, and verify the SVIDs of other workloads, based on a predefined set of conditions.
