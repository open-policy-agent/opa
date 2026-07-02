---
title: Traefik API Gateway
labels:
  category: servicemesh
  layer: gateway
tutorials:
- https://plugins.traefik.io/plugins/659e6aaf0f0494247310c69a/jwt-and-opa-access-management
code:
- https://github.com/traefik-plugins/traefik-jwt-plugin
- https://github.com/unsoon/traefik-open-policy-agent
---

The Traefik API Gateway is open-source software that controls API traffic into your application.
OPA can be configured as a plugin to implement authorization policies for those APIs.

[`traefik-jwt-plugin`](https://github.com/traefik-plugins/traefik-jwt-plugin)
is a Traefik plugin which checks JWT tokens for required fields.

[`traefik-open-policy-agent`](https://github.com/unsoon/traefik-open-policy-agent)
is another plugin available for use cases where more than JWT validation is needed.
