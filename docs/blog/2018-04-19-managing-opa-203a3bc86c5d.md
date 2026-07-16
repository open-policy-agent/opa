---
title: "Managing OPA"
authors: ["tsandall"]
date: 2018-04-19
slug: managing-opa-203a3bc86c5d
---

OPA is described as "a general-purpose policy engine that let's you offload decisions from your service," requiring access to policy and data for decision-making.

Before version 0.8, OPA "only exposed low-level HTTP APIs that let you push policy and data into the engine." Version 0.8 adds new management capabilities for distributing policies/data and monitoring agent health.

![Architecture diagram showing Control Plane connected via Policy Distribution and Policy Telemetry to two Nodes, each containing a Service and an OPA instance](/img/blog/managing-opa-203a3bc86c5d/banner.webp)

## Bundle API

OPA can now be configured to download "bundles" — described as "gzipped tarballs containing Rego and JSON files" — from remote HTTP endpoints on a periodic basis.

```
GET /bundles/example/authz HTTP/1.1
```

Services should respond with a gzipped tarball. If an ETag header is included, it gets reused in subsequent `If-None-Match` requests, allowing servers to reply with an HTTP 301 Not Modified instead of resending the same bundle.

More info: [http://www.openpolicyagent.org/docs/bundles.html](http://www.openpolicyagent.org/docs/bundles.html)

## Status API

Since OPA already supports Prometheus-based metrics reporting ([http://www.openpolicyagent.org/docs/monitoring-diagnostics.html#prometheus](http://www.openpolicyagent.org/docs/monitoring-diagnostics.html#prometheus)), the new Status API answers two additional questions: which policy/data version is active, and what errors occurred during load.

OPA can be configured to periodically POST status updates to a remote endpoint:

```json
POST /status HTTP/1.1
Content-Type: application/json
{
  "labels": {
    "app": "my-example-app",
    "id": "1780d507-aea2-45cc-ae50-fa153c8e4a5a"
  },
  "bundle": {
    "name": "http/example/authz",
    "active_revision": "660daf152602bd95ea2d2139d215678b0ed[...]",
    "last_successful_download": "2018-04-17T12:34:40.258Z",
    "last_successful_activation": "2018-04-17T12:34:42.196Z"
  }
}
```

The payload also includes labels that "uniquely identify the agent."

More info: [http://www.openpolicyagent.org/docs/status.html](http://www.openpolicyagent.org/docs/status.html)

## Decision Log API

For auditing and debugging, OPA can buffer and periodically upload batches of policy decisions to a remote endpoint. Each event includes:

- The name of the policy decision requested by your service.
- The input provided in the query by your service.
- The result returned by OPA to your service.

Events also include metadata such as bundle revision and agent labels.

More info: [http://www.openpolicyagent.org/docs/decision_logs.html](http://www.openpolicyagent.org/docs/decision_logs.html)

## Wrap Up

The author frames these features as simplifying large-scale OPA management while enabling more advanced tooling. Feedback is welcomed via Slack: [http://slack.openpolicyagent.org](http://slack.openpolicyagent.org)
