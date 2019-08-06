---
title: Status
kind: documentation
weight: 10
---

OPA can periodically report status updates to remote HTTP servers. The
updates contain status information for OPA itself as well as the
[Bundles](../bundles) that have been downloaded and activated.

OPA sends status reports whenever bundles are downloaded and activated. If
the bundle download or activation fails for any reason, the status update
will include error information describing the failure.

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
            "active_revision": "TODO",
            "last_successful_download": "2018-01-01T00:00:00.000Z",
            "last_successful_activation": "2018-01-01T00:00:00.000Z"
        }
    },
    "metrics": {
        "prometheus":  [
            {
                "help": "A summary of the GC invocation durations.",
                "name": "go_gc_duration_seconds",
                "type": 2,
                "metric": [
                    {
                        "summary": {
                            "quantile": [
                                {
                                    "quantile": 0,
                                    "value": 0.000044358
                                },
                                {
                                    "quantile": 0.25,
                                    "value": 0.000045003
                                },
                                {
                                    "quantile": 0.5,
                                    "value": 0.000049726
                                },
                                {
                                    "quantile": 0.75,
                                    "value": 0.000219553
                                },
                                {
                                    "quantile": 1,
                                    "value": 0.000219553
                                }
                            ],
                           "sample_count": 4,
                           "sample_sum": 0.00035864
                        }
                    }
                ]
            },
            {
                "help": "Number of goroutines that currently exist.",
                "name": "go_goroutines",
                "type": 1,
                "metric": [
                    {
                        "gauge": {
                            "value": 11
                        }
                    }
                ]
            },
            {
                "help": "Information about the Go environment.",
                "name": "go_info",
                "type": 1,
                "metric": [
                    {
                        "gauge": {
                            "value": 1
                        },
                        "label": [
                            {
                                "name": "version",
                                "value": "go1.12.7"
                            }
                        ]
                    }
                ]
            }
        ]
    }
}
```

Status updates contain the following fields:

| Field | Type | Description |
| --- | --- | --- |
| `labels` | `object` | Set of key-value pairs that uniquely identify the OPA instance. |
| `bundle.name` | `string` | (Deprecated) Name of bundle that the OPA instance is configured to download. Omitted when `bundles` are configured. |
| `bundle.active_revision` | `string` | (Deprecated) Opaque revision identifier of the last successful activation. Omitted when `bundles` are configured. |
| `bundle.last_successful_download` | `string` | (Deprecated) RFC3339 timestamp of last successful bundle download. Omitted when `bundles` are configured. |
| `bundle.last_successful_activation` | `string` | (Deprecated) RFC3339 timestamp of last successful bundle activation. Omitted when `bundles` are configured. |
| `bundles` | `object` | Set of objects describing the status for each bundle configured with OPA. |
| `bundles[_].name` | `string` | Name of bundle that the OPA instance is configured to download. |
| `bundles[_].active_revision` | `string` | Opaque revision identifier of the last successful activation. |
| `bundles[_].last_successful_download` | `string` | RFC3339 timestamp of last successful bundle download. |
| `bundles[_].last_successful_activation` | `string` | RFC3339 timestamp of last successful bundle activation. |
| `discovery.name` | `string` | Name of discovery bundle that the OPA instance is configured to download. |
| `discovery.active_revision` | `string` | Opaque revision identifier of the last successful discovery activation. |
| `discovery.last_successful_download` | `string` | RFC3339 timestamp of last successful discovery bundle download. |
| `discovery.last_successful_activation` | `string` | RFC3339 timestamp of last successful discovery bundle activation. |
| `metrics` | `object` | Application metrics. Optional, single key object. |
| `metrics[provider_name]` | JSON (`interface{}`) | Metrics in provider-dependent format. |

If the bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `bundle.code` | `string` | If present, indicates error(s) occurred. |
| `bundle.message` | `string` | Human readable messages describing the error(s). |
| `bundle.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

If the bundle download or activation failed, the status update will contain
the following additional fields.

| Field | Type | Description |
| --- | --- | --- |
| `discovery.code` | `string` | If present, indicates error(s) occurred. |
| `discovery.message` | `string` | Human readable messages describing the error(s). |
| `discovery.errors` | `array` | Collection of detailed parse or compile errors that occurred during activation. |

Services should reply with HTTP status `200 OK` if the status update is
processed successfully.
