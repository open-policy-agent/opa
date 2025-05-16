---
title: Dapr
software:
- dapr
labels:
  category: application
  layer: network
tutorials:
- https://docs.dapr.io/reference/components-reference/supported-middleware/middleware-opa/
code:
- https://github.com/dapr/dapr
- https://github.com/dapr/components-contrib/blob/master/middleware/http/opa/middleware.go
docs_features:
  go-integration:
    note: |
      Dapr's contrib middleware include an OPA integration built on the Go
      API.
      [This tutorial](https://docs.dapr.io/reference/components-reference/supported-middleware/middleware-opa/)
      explains how to configure it.
---
Middleware to apply Open Policy Agent policies on incoming requests
