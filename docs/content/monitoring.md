---
title: Monitoring
kind: documentation
weight: 11
---

## Prometheus

OPA exposes an HTTP endpoint that can be used to collect performance metrics
for all API calls. The Prometheus endpoint is enabled by default when you run
OPA as a server.

You can enable metric collection from OPA with the following `prometheus.yml` config:

```yaml
global_config:
  scrape_interval: 15s
scrape_configs:
  - job_name: "opa"
    metrics_path: "/metrics"
    static_configs:
    - targets:
      - "localhost:8181"
```

## Health Checks

OPA exposes a `/health` API endpoint that can be used to perform health checks.
The `/health` API endpoint executes a simple built-in policy query to verify
that the server is operational. Clients should check that OPA returns an HTTP
`200 OK` status. If a non-200 status is returned, clients should alarm.

> The current health check implementation does not take into account bundle
> activation.

## Diagnostics (Deprecated)

The diagnostics feature is deprecated. If you need to monitor OPA decisions, see
the [Decision Log](../decision-logs) API.
