---
title: i2scim.io SCIM Restful User/Group Provisioning API
labels:
  category: security
  layer: application
software:
- i2scim
code:
- https://github.com/i2-open/i2scim
- https://i2scim.io
inventors:
- i2
tutorials:
- https://i2scim.io/OPA_AccessControl.html
docs_features:
  rest-api-integration:
    note: |
      i2scim supports externalized access control decisions using OPA's REST API.
      The integration is described in the [i2scim documentation](https://i2scim.io/OPA_AccessControl.html).
---
i2scim.io is an open source, Apache 2 Licensed, implementation of SCIM (System for Cross-domain Identity Management RFC7643/7644) for use
cloud-native kubernetes platforms. i2scim supports externalized access control decisions through OPA. SCIM is a RESTful HTTP API that can be
used to provide a standardized way to provision accounts from Azure, Okta, PingIdentity and other providers and tools. SCIM can also be used
as a backing identity store for OAuth and other authentication services.

