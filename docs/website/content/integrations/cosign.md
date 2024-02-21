---
title: Container Signing, Verification and Storage in an OCI registry
labels:
  category: security
  layer: application
software:
- cosign
inventors:
- sigstore
code:
- https://docs.sigstore.dev/cosign/attestation#validate-in-toto-attestations
- https://github.com/sigstore/cosign-gatekeeper-provider
vides:
- https://www.youtube.com/watch?v=gCi9_4NYyR0
docs_features:
  go-integration:
    note: |
      Cosign In-Toto attestations can be
      [written in Rego](https://docs.sigstore.dev/cosign/attestation/#cosign-custom-predicate-type-and-rego-policy),
      these are evaluated in the Cosign binary using the Go API.
---
Cosign is a tool for container image signing and verifying maintained under the Project Sigstore
in collaboration with the Linux Foundation. Among other features, Cosign supports KMS signing,
built-in binary transparency, and timestamping service with Rekor and Kubernetes policy enforcement.

