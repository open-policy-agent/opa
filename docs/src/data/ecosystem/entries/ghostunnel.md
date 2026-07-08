---
title: Ghostunnel
subtitle: A simple TLS proxy with mutual authentication support
labels:
  layer: network
  category: proxy
software:
- ghostunnel
tutorials:
- https://ghostunnel.dev/docs/security/access-flags/#open-policy-agent
code:
- https://github.com/ghostunnel/ghostunnel
docs_features:
  go-integration:
    note: |
      Ghostunnel is written in Go and embeds OPA to evaluate Rego
      authorization policies, in addition to its built-in certificate-based
      access control checks.
---

Ghostunnel is a simple TLS proxy with mutual authentication support for securing non-TLS backend applications. It can enforce access control using Rego policies alongside certificate field checks.
