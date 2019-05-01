---
title: Monitoring
kind: documentation
weight: 12
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
See [Health API](/docs/{{< current_version >}}/rest-api#health-api) for details.

## Diagnostics (Deprecated)

The diagnostics feature is deprecated. If you need to monitor OPA decisions, see
the [Decision Log](../decision-logs) API.
