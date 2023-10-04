---
title: Strimzi (Apache Kafka on Kubernetes)
software:
- kafka
- strimzi
labels:
  category: streaming
  layer: data
blogs:
- https://strimzi.io/blog/2020/08/05/using-open-policy-agent-with-strimzi-and-apache-kafka/
- https://strimzi.io/blog/2020/09/01/enforce-custom-resource-policies-with-opa-gatekeeper/
code:
- https://github.com/strimzi/strimzi-kafka-operator
- https://github.com/scholzj/demo-opa-kafka-authorization
- https://github.com/StyraInc/opa-kafka-plugin
inventors:
- redhat
docs_features:
  rest-api-integration:
    note: |
      Strimzi can be configured to use OPA via the REST API as the Kafka
      authorizer using [this project](https://github.com/scholzj/demo-opa-kafka-authorization).
---
Strimzi provides a way to run an Apache Kafka cluster on Kubernetes in various deployment configurations. Strimzi ships with the OPA authorizer plugin right out of the box, and supports OPA as an option for Kafka authorization.
