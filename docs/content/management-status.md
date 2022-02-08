---
title: "Status"
kind: management
weight: 4
---

OPA can periodically report status updates to remote HTTP servers. The
updates contain status information for OPA itself as well as the
[Bundles](../management-bundles) that have been downloaded and activated.

OPA sends status reports whenever one of the following happens:

* Bundles are downloaded and activated -- If the bundle download or activation fails for any reason, the status update
  will include error information describing the failure. This includes Discovery bundles.
* A plugin state has changed -- All plugin status is reported, and an update to any plugin will
  trigger a Status API report which contains the latest state.

The status updates will include a set of labels that uniquely identify the
OPA instance. OPA automatically includes an `id` value in the label set that
provides a globally unique identifier or the running OPA instance and a
`version` value that provides the version of OPA.

See the [Configuration Reference](../configuration) for configuration details.

### Status Service API

OPA expects the service to expose an API endpoint that will receive status
updates.

```http
POST /status[/<partition_name>] HTTP/1.1
Content-Type: application/json
```

The partition name is an optional path segment that can be used to route
status updates to different backends. If the partition name is not configured
on the agent, updates will be sent to `/status`.

```json
{
  "labels": {
    "app": "my-example-app",
    "id": "1780d507-aea2-45cc-ae50-fa153c8e4a5a",
    "version": "{{< current_version >}}"
  },
  "bundles": {
    "http/example/authz": {
      "active_revision": "ABC",
      "last_request": "2018-01-01T00:00:00.000Z",
      "last_successful_request": "2018-01-01T00:00:00.000Z",
      "last_successful_download": "2018-01-01T00:00:00.000Z",
      "last_successful_activation": "2018-01-01T00:00:00.000Z",
      "metrics": {
        "timer_rego_data_parse_ns": 12345,
        "timer_rego_module_compile_ns": 12345,
        "timer_rego_module_parse_ns": 12345
      }
    }
  },
  "plugins": {
    "bundle": {
      "state": "OK"
    },
    "discovery": {
      "state": "OK"
    },
    "status": {
      "state": "OK"
    }
  },
  "metrics": {
    "prometheus": {
      "go_gc_duration_seconds": {
        "help": "A summary of the GC invocation durations.",
        "metric": [
          {
            "summary": {
              "quantile": [
                {
                  "quantile": 0,
                  "value": 0.000011799
                },
                {
                  "quantile": 0.25,
                  "value": 0.000011905
                },
                {
                  "quantile": 0.5,
                  "value": 0.000040002
                },
                {
                  "quantile": 0.75,
                  "value": 0.000065238
                },
                {
                  "quantile": 1,
                  "value": 0.000104897
                }
              ],
              "sample_count": 7,
              "sample_sum": 0.000309117
            }
          }
        ],
        "name": "go_gc_duration_seconds",
        "type": 2
      },
------------------------------8< SNIP 8<------------------------------
      "http_request_duration_seconds": {
        "help": "A histogram of duration for requests.",
        "metric": [
          {
            "histogram": {
              "bucket": [
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.005
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.01
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.025
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.05
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.1
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.25
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 0.5
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 1
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 2.5
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 5
                },
                {
                  "cumulative_count": 1,
                  "upper_bound": 10
                }
              ],
              "sample_count": 1,
              "sample_sum": 0.003157399
            },
            "label": [
              {
                "name": "code",
                "value": "200"
              },
              {
                "name": "handler",
                "value": "v1/data"
              },
              {
                "name": "method",
                "value": "get"
              }
            ]
          }
        ],
        "name": "http_request_duration_seconds",
        "type": 4
      }
    }
  }
}
```

Status updates contain the following fields:

| Field | Type | Description |
| --- | --- | --- |
| `labels` | `object` | Set of key-value pairs that uniquely identify the OPA instance. |
| `bundles` | `object` | Set of objects describing the status for each bundle configured with OPA. |
| `bundles[_].name` | `string` | Name of bundle that the OPA instance is configured to download. |
| `bundles[_].active_revision` | `string` | Opaque revision identifier of the last successful activation. |
| `bundles[_].last_request` | `string` | RFC3339 timestamp of last bundle request. This timestamp should be >= to the successful request timestamp in normal operation. |
| `bundles[_].last_successful_request` | `string` | RFC3339 timestamp of last successful bundle request. This timestamp should be >= to the successful download timestamp in normal operation. |
| `bundles[_].last_successful_download` | `string` | RFC3339 timestamp of last successful bundle download. |
| `bundles[_].last_successful_activation` | `string` | RFC3339 timestamp of last successful bundle activation. |
| `bundles[_].metrics` | `object` | Metrics from the last update of the bundle. |
| `bundles[_].code` | `string` | If present, indicates error(s) occurred activating this bundle. |
| `bundles[_].message` | `string` | Human readable messages describing the error(s). |
| `bundles[_].http_code` | `number` | If present, indicates an erroneous HTTP status code that OPA received downloading this bundle. |
| `bundles[_].errors` | `array` | Collection of detailed parse or compile errors that occurred during activation of this bundle. |
| `discovery.name` | `string` | Name of discovery bundle that the OPA instance is configured to download. |
| `discovery.active_revision` | `string` | Opaque revision identifier of the last successful discovery activation. |
| `discovery.last_request` | `string` | RFC3339 timestamp of last discovery bundle request. This timestamp should be >= to the successful request timestamp in normal operation. |
| `discovery.last_successful_request` | `string` | RFC3339 timestamp of last successful discovery bundle request. This timestamp should be >= to the successful download timestamp in normal operation. |
| `discovery.last_successful_download` | `string` | RFC3339 timestamp of last successful discovery bundle download. |
| `discovery.last_successful_activation` | `string` | RFC3339 timestamp of last successful discovery bundle activation. |
| `plugins` | `object` | A set of objects describing the state of configured plugins in OPA's runtime. |
| `plugins[_].state` | `string` | The state of each plugin. |
| `metrics.prometheus` | `object` | Global performance metrics for the OPA instance. |

If the discovery bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `discovery.code` | `string` | If present, indicates error(s) occurred. |
| `discovery.message` | `string` | Human readable messages describing the error(s). |
| `discovery.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

Services should reply with HTTP status `200 OK` if the status update is
processed successfully.

### Local Status Logs

Local console logging of status updates can be enabled via the `console` config option.
This does not require any remote server. Example of minimal config to enable:

```yaml
status:
    console: true
```
This will dump all status updates to the console. See
[Configuration Reference](../configuration) for more details.

> Warning: Status update messages are somewhat infrequent but can be very verbose! The
>`metrics.prometheus` portion of the status update in particular can create a considerable
> amount of log text at info level.

### Prometheus Status Metrics

Prometheus status metrics can be enabled via the `prometheus` config option. (see [the configuration documentation](../configuration/#status))
Example of minimal config to enable:

```yaml
status:
    prometheus: true
```

When enabled the OPA instance's Prometheus endpoint exposes the metrics described on [the monitoring documentation](../monitoring/#status-metrics).
