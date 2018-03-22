# Monitoring and Diagnostics

This document explains how to monitor OPA and use the diagnostics capabilities
exposed by the server.

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

## Diagnostics

The OPA server can record diagnostics on policy queries for debugging purposes.
The diagnostic configuration is controlled by the
`data.system.diagnostics.config` document (referred to simply as "config"
below).

The config below enables diagnostics for all POST requests received by OPA:

```
package system.diagnostics

config = {"mode": "on"} {
    input.method = "POST"
}
```

The table below summarizes the keys that can be specified in the config:

| Field | Value | Behavior |
| --- | --- | --- |
| `mode` | "off" | No diagnostics are recorded. |
| `mode` | "on"  | Enables collection of inexpensive values. This includes the query, input, result, and performance metrics. |
| `mode` | "all" | All diagnostics are collected. This includes a full trace of the query evaluation. |

OPA provides input to the config (which itself is a policy) so that diagnostics
can be recorded dynamically. The input document contains the following
information:

| Field | Example | Description |
| --- | --- | --- |
| `method` | `"POST"` | HTTP method from OPA API call. |
| `path` | `"/v1/data/authz/allow"` | HTTP path from OPA API call. |
| `body` | `{"input": {"user": "bob", "operation": "read", "resource": "bookstore"}}` | HTTP message body from OPA API call. |
| `params` | `{"partial": [""]}` | HTTP query parameters from OPA API call. |

Diagnostics can be queried at `/v1/data/system/diagnostics`. The response will contain the following fields:

| Field | Always Present | Description |
| --- | --- | --- |
| `timestamp` | Yes | Nanoseconds since the Unix Epoch time |
| `query` | Yes | The query, if the request for the record was a query request. Otherwise the data path from the original request. |
| `input` | No | Input provided to the `query` at evaluation time. |
| `result` | No | Result of evaluating `query`. See the [Data](rest-api.md#data-api) and [Query](rest-api.md#query-api) APIs for detailed descriptions of formats. |
| `error` | No | [Error](rest-api.md#errors) encountered while evaluating `query`. |
| `metrics` | No | [Performance Metrics](rest-api.md#performance-metrics) for `query`. |
| `explanation` | No | [Explanation](rest-api.md#explanations) of how `result` was found. |

The server will only store a finite number of diagnostics. If the server's
diagnostics storage becomes full, it will delete the oldest diagnostic to make
room for the new one. The size of the storage may be configured when the server
is started.