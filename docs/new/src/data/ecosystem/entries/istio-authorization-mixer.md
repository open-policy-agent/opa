---
title: Container Network Authorization with Istio (as part of Mixer)
labels:
  category: servicemesh
  layer: network
software:
- istio
tutorials:
- https://istio.io/docs/reference/config/policy-and-telemetry/adapters/opa/
code:
- https://github.com/istio/istio/tree/master/mixer/adapter/opa
inventors:
- google
---
Istio is a networking abstraction for cloud-native applications. In this Istio integration OPA hooks into the centralized Mixer component of Istio, to provide fine-grained, context-aware authorization for network or HTTP requests.
