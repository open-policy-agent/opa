---
title: OPA MCP
subtitle: Model Context Protocol server for authoring and debugging Rego
labels:
  category: tooling
  layer: editor
inventors:
- independent
code:
- https://github.com/OrygnsCode/opa-mcp-server
docs_features:
  learning-rego:
    note: |
      OPA MCP exposes higher-level helpers (rego_describe_policy,
      rego_suggest_fix, rego_explain_decision) that compose opa fmt,
      opa parse, and opa eval into AI-friendly tools. Agents in any MCP
      client (Claude Desktop, Cursor, VS Code, Zed) can author, format,
      and review Rego with structured output instead of free-form CLI text.
  policy-testing:
    note: |
      The rego_test tool runs opa test over a directory and returns
      pass/fail per test with optional coverage. rego_generate_test_skeleton
      produces a _test.rego scaffold covering each rule in a policy.
  debugging-rego:
    note: |
      rego_explain_decision wraps opa eval --explain=full and returns a
      structured trace, letting agents answer "why was this rejected"
      without reading raw traces. rego_eval_with_profile and
      rego_eval_with_coverage surface hot rules and per-line coverage.
  editors:
    note: |
      OPA MCP is a stdio MCP server that plugs into any MCP-compatible
      client. One-line Smithery install for Claude Desktop, drop-in
      configs for Cursor, VS Code, Zed, and Windsurf. Multi-arch Docker
      image and a signed .mcpb bundle are also published.
---

OPA MCP is a Model Context Protocol server that gives MCP-compatible
clients a structured interface to Rego. It wraps the OPA CLI (opa fmt,
opa check, opa eval, opa test, opa build, opa sign), the OPA REST API,
and the Regal linter, exposing 32 tools with stable error codes and
schema-validated input/output.

Higher-level helpers (rego_explain_decision, rego_describe_policy,
rego_generate_test_skeleton, rego_suggest_fix) compose the primitives
into the tasks agents typically perform. A curated resource set exposes
the OPA built-in function catalog, the Rego style guide, and a pattern
library covering RBAC, ABAC, Kubernetes admission, IaC gates, API
authorization, and rate limiting.

Distributed via npm (@orygn/opa-mcp), Docker Hub (orygn/opa-mcp), and a
signed .mcpb bundle.
