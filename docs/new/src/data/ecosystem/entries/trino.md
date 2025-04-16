---
title: Trino
software:
- trino
tutorials:
- https://trino.io/ecosystem/add-on#open-policy-agent
- https://trino.io/docs/current/security/opa-access-control.html
- https://trinodb.github.io/presentations/presentations/what-is-trino/index.html
code:
- https://github.com/trinodb/trino/tree/master/plugin/trino-opa
blogs: # optional, links to blog posts for the integration
- https://trino.io/blog/2024/02/06/opa-arrived
videos:
- title: Trino OPA Authorizer - Stackable and Bloomberg at Trino Summit 2023
  link: https://www.youtube.com/watch?v=fbqqapQbAv0
- title: Avoiding pitfalls with query federation in data lakehouses - Raft at Trino Summit 2023
  link: https://www.youtube.com/watch?v=6KspMwCbOfI&t=1279s
---
Trino is a ludicrously fast, open source, SQL query engine designed to query
large data sets from one or more disparate data sources.

The OPA Trino plugin enables the use of Open Policy Agent (OPA) as authorization
engine for access control to catalogs, schemas, tables, and other objects in
Trino. Policies are defined in OPA, and Trino checks access control privileges
in OPA.
