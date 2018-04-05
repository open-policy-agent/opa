# Decision Logs

OPA can periodically report decision logs to remote HTTP servers. The
decision logs contain events that describe policy queries. Each event
includes the policy that was queried, the input to the query, bundle
metadata, and other information that enables auditing and offline debugging
of policy decisions.

## Decision Log Configuration

You can configure OPA to periodically report decision logs by starting OPA
with a configuration file (both YAML and JSON files are supported):

```bash
opa run --server --config-file config.yaml
```

**config.yaml**:

```yaml
services:
  - name: acmecorp
    url: https://example.com/
    credentials:
      bearer:
        token: "Bearer <base64 encoded string>"
bundle:
  name: http/example/authz
  service: acmecorp
labels:
  app: example
decision_logs:
  service: acmecorp
  reporting:
    min_delay_seconds: 300
    max_delay_seconds: 600
```

With this configuration OPA will report decision logs to
`https://example.com` every 5-10 minutes. The reporting delay is randomized
(between min and max) to stagger upload requests when there are a large
number of OPAs uploading decision logs.

### Configuration File Format

```yaml
labels: object
decision_logs:
  service: string
  partition_name: string
  reporting:
    upload_size_limit_bytes: number
    min_delay_seconds: number
    max_delay_seconds: number
```

| Field | Required | Description |
| --- | --- | --- |
| `labels` | No |  Set of key-value pairs that uniquely identify the OPA instance. |
| `decision_logs.service` | Yes | Name of the service to use to contact remote server. |
| `decision_logs.partition_name` | No | Path segment to include in status updates. |
| `decision_logs.reporting.buffer_size_limit_bytes` | No | Decision log buffer size limit in bytes. OPA will drop old events from the log if this limit is exceeded. |
| `decision_logs.reporting.upload_size_limit_bytes` | No | Decision log upload size limit in bytes. OPA will chunk uploads to cap message body to this limit. |
| `decision_logs.reporting.min_delay_seconds` | No | Minimum amount of time to wait between uploads. |
| `decision_logs.reporting.max_delay_seconds` | No | Maximum amount of time to wait between uploads. |

### Decision Log Service API

OPA expects the service to expose an API endpoint that will receive decision logs.

```http
POST /logs[/<partition_name>] HTTP/1.1
Content-Encoding: gzip
Content-Type: application/json
```

The partition name is an optional path segment that can be used to route logs
to different backends. If the partition name is not configured on the agent,
updates will be sent to `/logs`.

The message body contains a gzip compressed JSON array. Each array element (event)
represents a policy decision returned by OPA.

```json
[
  {
    "labels": {
      "app": "my-example-app",
      "id": "1780d507-aea2-45cc-ae50-fa153c8e4a5a"
    },
    "decision_id": "4ca636c1-55e4-417a-b1d8-4aceb67960d1",
    "revision": "W3sibCI6InN5cy9jYXRhbG9nIiwicyI6NDA3MX1d",
    "path": "http/example/authz/allow",
    "input": {
      "method": "GET",
      "path": "/salary/bob"
    },
    "result": "true",
    "requested_by": "[::1]:59943",
    "timestamp": "2018-01-01T00:00:00.000000Z"
  }
]
```

Decision log updates contain the following fields:

| Field | Type | Description |
| --- | --- | --- |
| `[_].labels` | `object` | Set of key-value pairs that uniquely identify the OPA instance. |
| `[_].decision_id` | `string` | Unique identifier generated for each decision for traceability. |
| `[_].revision` | `string` | Bundle revision that contained the policy used to produce the decision. |
| `[_].path` | `string` | Hierarchical policy decision path, e.g., `/http/example/authz/allow`. |
| `[_].input` | `any` | Input data provided in the policy query. |
| `[_].result` | `any` | Policy decision returned to the client, e.g., `true` or `false`. |
| `[_].requested_by` | `string` | Identifier for client that executed policy query, e.g., the client address. |
| `[_].timestamp` | `string` | RFC3999 timestamp of policy decision. |
