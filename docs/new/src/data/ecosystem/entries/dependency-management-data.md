---
title: dependency-management-data
subtitle: A set of tooling to get a better understanding of the use of dependencies across your organisation.
labels:
  category: tooling
  layer: shell
inventors:
- jamie-tanna
blogs:
- https://dmd.tanna.dev/cookbooks/custom-advisories-opa/
code:
- https://gitlab.com/tanna.dev/dependency-management-data
tutorials:
- https://dmd.tanna.dev/cookbooks/custom-advisories-opa/
docs_features:
  go-integration:
    note: |
      dependency-management-data uses the Go Rego API to make it possible to
      write more complex rules around usages of Open Source and internal
      dependencies.

      Example policies can be found [in DMD's example project](https://gitlab.com/tanna.dev/dependency-management-data-example/-/tree/main/policies?ref_type=heads)
      and provide an indication of some common use-cases.
---
dependency-management-data is a set of tooling that makes it easier to
understand the usage of Open Source and internal dependencies in an
organisation, taking data from Renovate, GitHub Dependabot, or Software Bill of
Materials (SBOMs) and providing an SQLite database that can be used to query it.

Alongside this base functionality, it's possible to write "advisories" to flag
usage of certain dependencies for i.e. "this internal library has a security
vulnerability" or "this Open Source project is no longer maintained".

As a step further than this, it's now possible to write "policies", using Open
Policy Agent to provide much more powerful control over usage of dependencies,
leveraging the excellent support Rego and OPA has for common operations.