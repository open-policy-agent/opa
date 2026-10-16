---
title: Authz Playground
subtitle: Rego, Cedar, Casbin and ReBAC side by side in the browser
labels:
  category: tooling
  layer: rego
inventors:
- independent
code:
- https://github.com/0-draft/authz-playground
- https://0-draft.github.io/authz-playground/
docs_features:
  learning-rego:
    note: |
      Compare a Rego policy with Cedar, Casbin and Zanzibar-style tuples
      written for the same requirements. Edit the Rego policy and see which
      requests now get a different decision.
allow_missing_image: true
---

The same authorization requirements written in Rego, Cedar, Casbin and a Zanzibar-style model, with every possible request evaluated by all four. The Rego policy can be edited, and OPA, compiled to `js/wasm`, re-evaluates every request in the browser.
