---
title: Library-based Microservice Authorization
labels:
  category: servicemesh
  layer: library
videos:
- title: How Netflix is Solving Authorization Across Their Cloud
  speakers:
  - name: Manish Mehta
    organization: netflix
  - name: Torin Sandall
    organization: styra
  venue: Kubecon Austin 2017
  link: https://www.youtube.com/watch?v=R6tUNpRpdnY
allow_missing_image: true
---
Microservice authorization can be enforced through a network proxy like Envoy/Istio/Linkerd/... or can be enforced by modifying the microservice code to use a common library.  In both cases OPA makes the authorization decision that the network proxy or the library enforce.
