---
title: Spring Security Authorization
subtitle: Use OPA to make authorization decisions in Spring applications
for_language: java
labels:
  layer: network
  category: application
software:
- java
code:
- https://github.com/open-policy-agent/contrib/tree/main/spring_authz
- https://github.com/Bisnode/opa-spring-security
- https://github.com/massenz/jwt-opa
- https://github.com/eugenp/tutorials/tree/master/spring-security-modules/spring-security-opa
- https://github.com/big-acl/authz-spring-boot-starter
tutorials:
- https://github.com/open-policy-agent/contrib/blob/main/spring_authz/README.md
- https://github.com/massenz/jwt-opa#web-server-demo-app
- https://www.baeldung.com/spring-security-authorization-opa
inventors:
- styra
- build-security
- bisnode
- alertavert
- big-acl
docs_features:
  rest-api-integration:
    note: |
      OPA Spring Security uses the REST API to query OPA about authz decisions.
      See an example application in OPA's
      [contrib repo](https://github.com/open-policy-agent/contrib/tree/main/spring_authz).
---

Spring Security provides a framework for securing Java applications. These integrations provide simple implementations for Spring Security that use OPA for making API authorization decisions. They provide support for both traditional Spring Security (MVC), as well as an implementation for Spring Reactive (Web Flux).
