---
title: Moat
subtitle: A Data Control Plane for Trino & OPA
labels:
  category: updates
  layer: application
inventors:
- moat-io
software:
- trino
code:
- https://github.com/moat-io/moat
tutorials:
- https://moat-io.github.io/moat/
docs_features:
  opa-bundles:
    note: |
      Moat supplies bundles to OPA to provide powerful data authorisation for Trino
      See [Moat Bundles](https://moat-io.github.io/moat/bundles/).
  external-data:
    note: |
      Moat aggregates data from multiple identity and metadata sources to provide powerful data authorisation for Trino
      See [Trino & OPA](https://moat-io.github.io/moat/trino_and_opa/).
---

OPA and Trino are an awesome combination, but maintaining the policy documents and required data object can be painful. Moat makes this easy with managed curation of principals and tables/views, as well as a predefined set of RBAC/ABAC policies suitable for most use cases. These policies can be used as-is, modified or completely replaced as needed.

## Moat provides a Data Control Plane to serve bundles to OPA, including

- SCIM2.0 server to allow integration with most identity providers (e.g Okta, EntraId)
- Data objects and attributes ingested from various sources (SQL DBs, data catalogs etc)
- Principals, attributes and groups ingested from identity providers (SQL DB, LDAP, etc)
- Pre-built rego policy documents to support common use cases (e.g. RBAC)
- OPA-compliant bundle API with built-in caching to allow fast polling

Moat itself is not involved in policy decisions at runtime, it simply provides the information to the battle-hardened OPA.

Moat can serve bundles to any number of OPA/Trino installations. This makes it very convenient to manage permissions across a fleet of trino clusters as well as ephemeral clusters. Simply add an OPA container to the coordinator deployment and point its bundle service to moat
