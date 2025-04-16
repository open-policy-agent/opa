---
title: Styra Enterprise OPA
software:
- enterprise-opa
labels:
  category: authorization
  type: poweredbyopa
tutorials:
- https://docs.styra.com/enterprise-opa/tutorials
- https://docs.styra.com/enterprise-opa/tutorials/performance-testing
- https://docs.styra.com/enterprise-opa/tutorials/grpc-basic-tutorial
- https://docs.styra.com/enterprise-opa/tutorials/grpc-go-tutorial
- https://docs.styra.com/enterprise-opa/tutorials/lia
- https://docs.styra.com/enterprise-opa/tutorials/decision-logs/
- https://docs.styra.com/enterprise-opa/tutorials/kafka
- https://docs.styra.com/enterprise-opa/tutorials/abac-with-sql
code:
- https://github.com/StyraInc/enterprise-opa
inventors:
- styra
blogs:
- https://www.styra.com/blog/introducing-styra-load-enterprise-opa-distribution-for-data-heavy-authorization/
videos:
- title: Start Loving Your Data-heavy Authorization
  speakers:
  - name: Torin Sandall
    organization: styra
  - name: Chris Hendrix
    organization: styra
  venue: online
  link: https://www.youtube.com/watch?v=Is1iBPr1YVs
docs_features:
  opa-bundles:
    note: |
      Its possible to configure bundles for both
      [policy](https://docs.styra.com/enterprise-opa/reference/configuration/policy/bundle-api)
      and
      [data](https://docs.styra.com/enterprise-opa/reference/configuration/data/bundle-api)
      in Enterprise OPA.
  external-data:
    note: |
      [Enterprise OPA](https://docs.styra.com/enterprise-opa/)
      supports various external data sources, including:
      [Kafka](https://docs.styra.com/enterprise-opa/reference/configuration/data/kafka),
      [Okta](https://docs.styra.com/enterprise-opa/reference/configuration/data/okta),
      [LDAP](https://docs.styra.com/enterprise-opa/reference/configuration/data/ldap),
      [HTTP](https://docs.styra.com/enterprise-opa/reference/configuration/data/http),
      [Git](https://docs.styra.com/enterprise-opa/reference/configuration/data/git)
      and
      [S3](https://docs.styra.com/enterprise-opa/reference/configuration/data/s3).
      Runtime support for
      [SQL](https://docs.styra.com/enterprise-opa/tutorials/abac-with-sql)
      is also available.
  external-data-runtime:
    note: |
      [Enterprise OPA](https://docs.styra.com/enterprise-opa/)
      can load data from SQL sources at runtime. The feature
      [is documented](https://docs.styra.com/enterprise-opa/tutorials/abac-with-sql)
      here with an ABAC example.
  external-data-realtime-push:
    note: |
      It's possible to steam data updates to
      [Enterprise OPA](https://docs.styra.com/enterprise-opa/)
      using Kafka. The Kafka integration is
      [documented](https://docs.styra.com/enterprise-opa/tutorials/kafka)
      here.
  policy-testing:
    note: |
      [Enterprise OPA's](https://docs.styra.com/enterprise-opa/)
      [Live Impact Analysis (LIA) feature](https://docs.styra.com/enterprise-opa/tutorials/lia)
      allows you to test changes to Rego policy on running instances.
  decision-logging:
    note: |
      It's possible to send decision logs to a enterprise tools like Splunk,
      and Kafka. This enhanced decision logging functionality is
      [documented here](https://docs.styra.com/enterprise-opa/tutorials/decision-logs/).
---
An enterprise-grade drop-in replacement for the Open Policy Agent with improved performance and out of the box enterprise integrations
