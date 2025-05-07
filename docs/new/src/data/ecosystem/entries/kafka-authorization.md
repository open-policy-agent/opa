---
title: Kafka Topic Authorization
software:
- kafka
labels:
  category: streaming
  layer: data
blogs:
- https://opencredo.com/blogs/controlling-kafka-data-flows-using-open-policy-agent/
tutorials:
- https://www.openpolicyagent.org/docs/latest/kafka-authorization/
code:
- https://github.com/StyraInc/opa-kafka-plugin
- https://github.com/llofberg/kafka-authorizer-opa
- https://github.com/opencredo/opa-single-message-transformer
inventors:
- ticketmaster
- styra
videos:
- title: 'OPA at Scale: How Pinterest Manages Policy Distribution'
  speakers:
  - name: Will Fu
    organization: pinterest
  - name: Jeremy Krach
    organization: pinterest
  venue: OPA Summit at Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=LhgxFICWsA8
docs_features:
  rest-api-integration:
    note: |
      This project implements a custom
      [Kafka authorizer](https://docs.confluent.io/platform/current/kafka/authorization.html#authorizer)
      that uses OPA to make authorization decisions by calling the REST API.

      Installation and configuration instructions are available in the
      project's [README](https://github.com/StyraInc/opa-kafka-plugin#installation).
---
Apache Kafka is a high-performance distributed streaming platform deployed by thousands of companies.  OPA provides fine-grained, context-aware access control of which users can read/write which Kafka topics to enforce important requirements around confidentiality and integrity.
