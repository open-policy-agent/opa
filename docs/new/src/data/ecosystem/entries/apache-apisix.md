---
title: Authorization Integration with Apache APISIX
software:
- apache-apisix
labels:
  category: gateway
  layer: network
code:
- https://github.com/apache/apisix
blogs:
- https://apisix.apache.org/blog/2021/12/24/open-policy-agent
- https://medium.com/@ApacheAPISIX/apache-apisix-integrates-with-open-policy-agent-to-enrich-its-ecosystem-15569fe3ab9c
docs_features:
  rest-api-integration:
    note: |
      Apache APISIX routes can be configured to call an OPA instance over
      the REST API.
      [This blog post](https://apisix.apache.org/blog/2021/12/24/open-policy-agent/)
      explains how such a configuration can be achieved.
---
Apache APISIX provides a plugin for delegating fine-grained authorization decisions to OPA.
