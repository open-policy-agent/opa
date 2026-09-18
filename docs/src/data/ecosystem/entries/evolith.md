---
title: Evolith
subtitle: Architecture governance as a PR gate, Rego evaluated in Wasm
labels:
  type: poweredbyopa
  category: tooling
  layer: configuration
inventors:
- beyondnet
software:
- github-actions
- nodejs
- typescript
code:
- https://github.com/beyondnetcode/evolith_arch32
- https://www.npmjs.com/package/@beyondnet/evolith-cli
tutorials:
- https://github.com/beyondnetcode/evolith_arch32#quick-start
- https://beyondnetcode.github.io/evolith_arch32/
docs_features:
  wasm-integration:
    note: |
      Evolith compiles its rule corpus to a `policy.wasm` bundle and evaluates
      it in Node.js through the
      [OPA Wasm JavaScript module](https://github.com/open-policy-agent/npm-opa-wasm),
      so its CLI, GitHub Action, MCP server and REST API all run the same policy
      build. The distribution model is written up in
      [ADR-0085](https://github.com/beyondnetcode/evolith_arch32/blob/main/reference/core/architecture/adrs/core/0085-agnostic-opa-wasm-distribution.md).
---

Evolith runs architecture rules — layering, dependencies, security, CI/CD, ADRs — against a repository from CI and fails the pull request. Rules are written in Rego and evaluated through OPA's Wasm build. A rule the engine could not evaluate is reported as a failure, never as a silent pass, so coverage and compliance stop looking identical: `skipped` is a first-class outcome, and a blocking rule that ends `skipped` fails the run.

It ships as a CLI (`npx -y @beyondnet/evolith-cli validate --engine opa`), a GitHub Action, an MCP server for AI coding agents and a REST API.
