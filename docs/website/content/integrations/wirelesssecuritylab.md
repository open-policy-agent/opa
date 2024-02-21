---
title: ccbr
labels:
  category: network
  layer: application
software:
- ccbr
code:
- https://github.com/wirelesssecuritylab/ccbr
inventors:
- wirelesssecuritylab
docs_features:
  kubernetes:
    note: 'Implements the CIS benchmark using Rego for Kubernetes workloads.'
allow_missing_image: true
---
CCBR is a policy management system project. It uses the policy language
Rego to implement the CIS benchmark test of cloud native kubernetes.
In addition, it integrates gatekeeper, manages its constraint templates,
constraints and policies, and supports policy deployment and audit inspection.

