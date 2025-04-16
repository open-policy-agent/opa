---
title: Docker controls via OPA Policies
software:
- docker
labels:
  layer: server
  category: container
code:
- https://github.com/open-policy-agent/opa-docker-authz
tutorials:
- https://www.openpolicyagent.org/docs/latest/docker-authorization/
inventors:
- styra
---
Docker's out of the box authorization model is all or nothing.  This integration demonstrates how to use OPA's context-aware policies to exert fine-grained control over Docker.
