---
title: Authorization Integration with Apache APISIX
software:
- apache-apisix
labels:
  category: gateway
  layer: network
code:
- https://github.com/apache/apisix
tutorials:
- https://apisix.apache.org/docs/apisix/plugins/opa/
videos:
- https://www.youtube.com/watch?v=DWl0QEYIXaA
blogs:
- https://apisix.apache.org/blog/2021/12/24/open-policy-agent/
- https://apisix.apache.org/blog/2023/03/02/security-policy-auditable/
docs_features:
  rest-api-integration:
    note: |
      Apache APISIX routes can delegate authorization decisions to OPA through
      the REST API. The
      [Apache APISIX OPA plugin documentation](https://apisix.apache.org/docs/apisix/plugins/opa/)
      describes the request and decision contract, plugin configuration, and
      example authorization policies.
---

Apache APISIX provides a plugin for delegating fine-grained authorization decisions to OPA.
