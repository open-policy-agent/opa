---
title: Open Policy Registry
subtitle: A Docker-inspired workflow for OPA policies
labels:
  category: containers
  layer: application
inventors:
- aserto
software:
- open-policy-registry
code:
- https://github.com/opcr-io/policy
tutorials:
- https://www.openpolicyregistry.io/docs/tutorial
blogs:
- https://www.openpolicyregistry.io/blog/docker-workflow-for-opa
docs_features:
  go-integration:
    note: |
      Makes use of the
      [OPA Repl package](https://pkg.go.dev/github.com/open-policy-agent/opa/repl)
      to interact with an OPA instance.
  opa-bundles:
    note: |
      OPCR policy images can be loaded in over the Bundle API. The feature
      it documented in the
      [OPCR docs](https://openpolicycontainers.com/docs/opa).
  opa-bundles-discovery:
    note: |
      OPCR images can be loaded in over the Bundle API and contain
      discovery bundles. The feature it documented in the
      [OPCR docs](https://openpolicycontainers.com/docs/opa).
  external-data:
    note: |
      OPCR policy images can contain data as well as policy. If you need to
      distribute data to OPA from an OCI registry, OPCR can build and push
      such images. See the docs for
      [building images here](https://openpolicycontainers.com/docs/cli/build).
---
The Open Policy Registry project provides a docker-style workflow for OPA policies.
The policy CLI can be used to build, tag, sign, push, and pull OPA policies as OCIv2 container images,
in conjunction with any container registry.
The Open Policy Registry (OPCR) is a reference implementation of a policy registry, built and hosted on GCP.

