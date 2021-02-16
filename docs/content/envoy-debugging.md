---
title: Debugging Tips
kind: envoy
weight: 110
---

This page provides some pointers that could assist in addressing issues encountered while using the
OPA-Envoy plugin. If none of these tips work, feel free to join
[slack.openpolicyagent.org](https://slack.openpolicyagent.org) and ask for help.

## Debugging Performance Issues

### Benchmarking Queries

The `opa bench` command evaluates a Rego query multiple times and reports metrics. You can also profile your polices using
`opa eval` to understand expression evaluation time. More information on improving policy performance can be found [here](https://www.openpolicyagent.org/docs/latest/policy-performance/).

### Analyzing Decision Logs

The OPA-Envoy plugin logs every decision that it makes. These logs contain lots of useful information including metrics like
gRPC server handler time and Rego query evaluation time which can help in measuring the OPA-Envoy plugin's performance.
To enable local console logging of decisions see [this](https://www.openpolicyagent.org/docs/latest/management/#local-decision-logs).

### Envoy External Authorization Filter Configuration

Envoy's External authorization gRPC service configuration uses either Envoy’s in-built gRPC client, or the Google C++ gRPC client.
From the [benchmarking](../envoy-performance#opa-benchmarks) results, lower latency numbers are seen while using Envoy’s gRPC client versus Google's. Experimenting
with the gRPC service configuration may help in improving performance.

The filter configuration also has a `status_on_error` field that can be used to indicate a network error between the filter
and the OPA-Envoy plugin. The default status on such an error is HTTP `403 Forbidden`. Changing the default value of this
field will help uncover potential network issues as `403 Forbidden` is also generated when a request is denied.

## Interacting with the gRPC server

This section provides examples of interacting with the Envoy External Authorization gRPC server using the [grpcurl](https://github.com/fullstorydev/grpcurl) tool.

* List all services exposed by the server

  ```bash
  $ grpcurl -plaintext localhost:9191 list
  ```

  Output:

  ```bash
  envoy.service.auth.v2.Authorization
  envoy.service.auth.v3.Authorization
  grpc.reflection.v1alpha.ServerReflection
  ```

* Invoke a v3 Check RPC on the server

  ```bash
  $ grpcurl -plaintext -d '
  {
    "attributes": {
      "request": {
        "http": {
          "method": "GET",
          "path": "/api/v1/products"
        }
      }
    }
  }' localhost:9191 envoy.service.auth.v3.Authorization/Check
  ```

  Output:

  ```json
  {
    "status": {

    },
    "okResponse": {
      "headers": [
        {
          "header": {
            "key": "x-ext-auth-allow",
            "value": "yes"
          }
        }
      ]
    }
  }
  ```
