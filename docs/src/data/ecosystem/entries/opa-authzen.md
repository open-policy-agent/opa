---
title: OPA-AuthZEN
subtitle: OpenID AuthZEN Authorization API as an OPA plugin
labels:
  category: authorization
  layer: application
inventors:
- independent
code:
- https://github.com/kanywst/opa-authzen-plugin
docs_features:
  go-integration:
    note: |
      Built as an OPA plugin in the same way as opa-envoy-plugin. It adds
      the AuthZEN endpoints to OPA's own HTTP server and evaluates the
      configured rule with the Rego API.
allow_missing_image: true
---

An OPA build that serves the [OpenID AuthZEN Authorization API 1.0](https://openid.net/specs/authorization-api-1_0.html): single and batch evaluations, the search APIs and PDP metadata. A PEP that speaks AuthZEN can call OPA directly, with the decision coming from a Rego rule.
