---
title: Minio API Authorization
labels:
  layer: data
  category: authorization
tutorials:
- https://github.com/minio/minio/blob/master/docs/iam/opa.md
inventors:
- minio
- styra
docs_features:
  rest-api-integration:
    note: |
      Minio implements a native integration with OPA using the REST API.
      The [integration is documented](https://github.com/minio/minio/blob/master/docs/iam/opa.md)
      in the Minio docs.
---
Minio is an open source, on-premise object database compatible with the Amazon S3 API.  This integration lets OPA enforce policies on Minio's API.
