---
title: Configuration Reference
kind: documentation
weight: 5
---

This page defines the format of OPA configuration files. Fields marked as
required must be specified if the parent is defined. For example, when the
configuration contains a `status` key, the `status.service` field must be
defined.

The configuration file path is specified with the `-c` or `--config-file`
command line argument:

```bash
opa run -s -c config.yaml
```

## Example

```yaml
services:
  - name: acmecorp
    url: https://example.com/control-plane-api/v1
    credentials:
      bearer:
        token: "bGFza2RqZmxha3NkamZsa2Fqc2Rsa2ZqYWtsc2RqZmtramRmYWxkc2tm"

labels:
  app: myapp
  region: west
  environment: production

bundle:
  name: http/example/authz
  service: acmecorp
  prefix: bundles
  polling:
    min_delay_seconds: 60
    max_delay_seconds: 120

decision_logs:
  service: acmecorp
  reporting:
    min_delay_seconds: 300
    max_delay_seconds: 600

status:
  service: acmecorp

default_decision: /http/example/authz/allow
```

## Services

Services represent endpoints that implement one or more control plane APIs
such as the Bundle or Status APIs. OPA configuration files may contain
multiple services.

{{< config "Services" >}}

> Services can be defined as an array or object. When defined as an object, the
> object keys override the `services[_].name` fields.

## Miscellaenous

{{< config "Miscellaneous" >}}

## Bundles

{{< config "Bundles" >}}

## Status

{{< config "Status" >}}

## Decision Logs

{{< config "Decision Logs" >}}

## Discovery

{{< config "Discovery" >}}
