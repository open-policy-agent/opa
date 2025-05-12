---
title: Kubernetes Authorization
code:
- https://github.com/open-policy-agent/contrib/tree/main/k8s_authorization
blogs:
- https://blog.styra.com/blog/kubernetes-authorization-webhook
- https://itnext.io/kubernetes-authorization-via-open-policy-agent-a9455d9d5ceb
- https://itnext.io/optimizing-open-policy-agent-based-kubernetes-authorization-via-go-execution-tracer-7b439bb5dc5b
inventors:
- styra
docs_features:
  rest-api-integration:
    note: |
      The Kubernetes API server can be configured to use OPA as an
      authorization webhook. Such an integration can be configured by
      following [the documentation](https://github.com/open-policy-agent/contrib/tree/main/k8s_authorization)
      in the contrib repo.
  kubernetes:
    note: |
      View
      [an example project](https://github.com/open-policy-agent/contrib/tree/main/k8s_authorization)
      showing how it's possible to integrate OPA with Kubernetes User Authorization.
---
Kubernetes Authorization is a pluggable mechanism that lets administrators control which users can run which APIs and
is often handled by builtin RBAC.  OPA's policy language is more flexible than the RBAC, for example,
writing policy using a prohibited list of APIs instead of the usual RBAC style of listing the permitted APIs.

