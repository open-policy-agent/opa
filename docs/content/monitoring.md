---
title: Monitoring
kind: operations
weight: 30
restrictedtoc: true
---

## Monitoring

### Prometheus

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

### Health Checks

OPA exposes a `/health` API endpoint that can be used to perform health checks.
See [Health API](../rest-api#health-api) for details.
