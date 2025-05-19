---
title: Ceph Object Storage Authorization
software:
- ceph
labels:
  category: object
  layer: data
tutorials:
- https://docs.ceph.com/docs/master/radosgw/opa/
- https://www.katacoda.com/styra/scenarios/opa-ceph
inventors:
- styra
- redhat
videos:
- https://www.youtube.com/watch?v=9m4FymEvOqM&feature=share
docs_feature:
  rest-api-integration:
    note: |
      The Ceph Object Gateway implements a native integration with OPA using
      the REST API.
      The [integration is documented](https://docs.ceph.com/en/latest/radosgw/opa/)
      in the Ceph docs.
---
Ceph is a highly scalable distributed storage solution that uniquely delivers object, block, and file storage in one unified system.  OPA provides fine-grained, context-aware authorization of the information stored within Ceph.
