---
title: Chef Automate
subtitle: Operational Visibility Dashboard
tutorials:
- https://github.com/chef/automate/tree/master/components/authz-service#authz-with-opa
videos:
- title: 'OPA in Practice: From Angular to OPA in Chef Automate'
  speakers:
  - name: Michael Sorens
    organization: chef
  venue: OPA Summit at Kubecon San Diego 2019
  link: https://www.youtube.com/watch?v=jrrW855xL3s
docs_features:
  go-integration:
    note: |
      Chef Automate uses the Go Rego API to evaluate authorization policies
      controlling access to its own API endpoints. The feature is
      [documented here](https://github.com/chef/automate/tree/master/components/authz-service#authz-with-opa).
allow_missing_image: true
---
Application require authorization decisions made at the API gateway, frontend, backend, and database.
OPA helps developers decouple authorization logic from application code, define a custom authorization model
that enables end-users to control tenant permissions, and enforce that policy across the different components of the
application (gateway, frontend, backend, database).

