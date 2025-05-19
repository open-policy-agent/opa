---
title: Topaz
labels:
  category: authorization
  layer: application
  type: poweredbyopa
inventors:
- aserto
software:
- topaz
code:
- https://github.com/aserto-dev/topaz
tutorials:
- https://www.topaz.sh
- https://github.com/aserto-dev/topaz#quickstart
blogs:
- https://www.aserto.com/blog/topaz-oss-cloud-native-authorization-combines-opa-zanzibar
docs_features:
  go-integration:
    note: |
      Topaz's Authorizer component makes use of the Rego API to evaluate
      policies to make authorization decisions for connected applications.
---
Topaz is an open source authorization service providing fine grained, real-time, policy based access control for applications and APIs.
Topaz uses OPA as its decision engine, and includes an embedded database that stores subjects, relations, and objects, inspired by the Google Zanzibar data model.
Topaz can be deployed as a sidecar or microservice in your cloud.

