---
title: Cloudflare Worker Enforcement of OPA Policies Using Wasm
software:
- cloudflare
labels:
  layer: application
  category: serverless
code:
- https://github.com/open-policy-agent/contrib/tree/master/wasm/cloudflare-worker
tutorials:
- https://github.com/open-policy-agent/contrib/blob/master/wasm/cloudflare-worker/README.md
docs_features:
  wasm-integration:
    note: |
      This example project in
      [OPA contrib](https://github.com/open-policy-agent/contrib/tree/main/wasm/cloudflare-worker)
      uses the
      [NodeJS OPA Wasm Module](https://github.com/open-policy-agent/npm-opa-wasm)
      to enforce policy at the edge of Cloudflare's network.
---
Cloudflare Workers are a serverless platform that supports Wasm.
This integration uses OPA's Wasm compiler to generate code enforced at the edge of Cloudflare's network.

