---
title: NATS Plugin for OPA
subtitle: An LRU cache backed by NATS Key-Value store for OPA
labels:
  layer: data
  category: data
inventors:
- permitio
software:
- nats
code:
- https://github.com/permitio/opa-nats
docs_features:
  go-integration:
    note: |
      opa-nats is written in Go and ships a custom OPA binary with the
      plugin pre-registered, as well as a library for embedding the plugin
      in other Go-based OPA builds.
  external-data:
    note: |
      opa-nats overrides the OPA runtime store for specific paths, backing
      them with an LRU cache populated from a NATS Key-Value store.
---

opa-nats is a Go plugin that backs OPA's runtime store with a [NATS Key-Value store](https://docs.nats.io/nats-concepts/jetstream/key-value-store), letting policies read externally managed data through an LRU cache kept in sync with NATS.
