---
title: Container Network Authorization with Envoy
subtitle: Official OPA Envoy Integration
labels:
  category: servicemesh
  layer: network
software:
- envoy
tutorials:
- https://github.com/tsandall/minimal-opa-envoy-example/blob/master/README.md
- https://www.openpolicyagent.org/docs/latest/envoy-introduction/
code:
- https://github.com/open-policy-agent/opa-envoy-plugin
- https://github.com/tsandall/minimal-opa-envoy-example
inventors:
- styra
blogs:
- https://blog.openpolicyagent.org/envoy-external-authorization-with-opa-578213ed567c
videos:
- title: 'OPA at Scale: How Pinterest Manages Policy Distribution'
  speakers:
  - name: Will Fu
    organization: pinterest
  - name: Jeremy Krach
    organization: pinterest
  venue: OPA Summit at Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=LhgxFICWsA8
- title: Deploying Open Policy Agent at Atlassian
  speakers:
  - name: Chris Stivers
    organization: atlassian
  - name: Nicholas Higgins
    organization: atlassian
  venue: OPA Summit at Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=nvRTO8xjmrg
- title: How Yelp Moved Security From the App to the Mesh with Envoy and OPA
  speakers:
  - name: Daniel Popescu
    organization: yelp
  - name: Ben Plotnick
    organization: yelp
  venue: Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=Z6aN3Smt-9M
docs_features:
  envoy:
    note: |
      The
      [opa-envoy-plugin](https://github.com/open-policy-agent/opa-envoy-plugin)
      project is the official integration for OPA and Envoy.
  rest-api-integration:
    note: |
      The [opa-envoy-plugin](https://github.com/open-policy-agent/opa-envoy-plugin)
      project uses the REST API to allow and deny requests routed via an Envoy proxy.

      Read about this integration in the
      [OPA Docs](https://www.openpolicyagent.org/docs/latest/envoy-introduction/).
---
Envoy is a networking abstraction for cloud-native applications. OPA hooks into Envoy’s external authorization filter to provide fine-grained, context-aware authorization for network or HTTP requests.
