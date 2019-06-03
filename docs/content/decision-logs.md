---
title: Decision Logs
kind: documentation
weight: 11
---

OPA can periodically report decision logs to remote HTTP servers. The decision
logs contain events that describe policy queries. Each event includes the policy
that was queried, the input to the query, bundle metadata, and other information
that enables auditing and offline debugging of policy decisions.

When decision logging is enabled the OPA server will include a `decision_id`
field in API calls that return policy decisions.

See the [Configuration Reference](../configuration) for configuration details.

## Decision Log Service API

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
      "id": "1780d507-aea2-45cc-ae50-fa153c8e4a5a",
      "version": "{{< current_version >}}"
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
| `[_].path` | `string` | Hierarchical policy decision path, e.g., `/http/example/authz/allow`. Receivers should tolerate slash-prefixed paths. |
| `[_].query` | `string` | Ad-hoc Rego query received by Query API. |
| `[_].input` | `any` | Input data provided in the policy query. |
| `[_].result` | `any` | Policy decision returned to the client, e.g., `true` or `false`. |
| `[_].requested_by` | `string` | Identifier for client that executed policy query, e.g., the client address. |
| `[_].timestamp` | `string` | RFC3999 timestamp of policy decision. |
| `[_].metrics` | `object` | Key-value pairs of [performance metrics](../rest-api#performance-metrics). |
| `[_].erased` | `array[string]` | Set of JSON Pointers specifying fields in the event that were erased. |

## Masking Sensitive Data

Policy queries may contain sensitive information in the `input` document that
must be removed before decision logs are uploaded to the remote API (e.g.,
usernames, passwords, etc.) Similarly, parts of the policy decision itself may
be considered sensitive.

By default, OPA queries the `data.system.log.mask` path prior to encoding and
uploading decision logs. OPA provides the decision log event as input to the
policy query and expects the query to return a set of JSON Pointers that refer
to fields in the decision log event to erase.

For example, assume OPA is queried with the following `input` document:

```json
{
  "resource": "user",
  "name": "bob",
  "password": "passw0rd"
}
```

To remove the `password` field from decision log events related to "user"
resources, supply the following policy to OPA:

```ruby
package system.log

mask["/input/password"] {
  # OPA provides the entire decision log event as input to the masking policy.
  # Refer to the original input document under input.input.
  input.input.resource == "user"
}

# To mask certain fields unconditionally, omit the rule body.
mask["/input/ssn"]
```

When the masking policy generates one or more JSON Pointers, they will be erased
from the decision log event. The erased paths are recorded on the event itself:

```json
{
  "decision_id": "b4638167-7fcb-4bc7-9e80-31f5f87cb738",
  "erased": [
    "/input/password",
    "/input/ssn"
  ],
  "input": {
    "name": "bob",
    "resource": "user"
  },
------------------------- 8< -------------------------
  "path": "system/main",
  "requested_by": "127.0.0.1:36412",
  "result": true,
  "timestamp": "2019-06-03T20:07:16.939402185Z"
}
```

There are a few restrictions on the JSON Pointers that OPA will erase:

* Pointers must be prefixed with `/input` or `/result`.
* Pointers may be undefined. For example `/input/name/first` in the example
  above would be undefined. Undefined pointers are ignored.
* Pointers must refer to object keys. Pointers to array elements will be treated
  as undefined.