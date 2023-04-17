---
title: Overview & Architecture
kind: envoy
weight: 1
---

[Envoy](https://www.envoyproxy.io/docs/envoy/latest/intro/what_is_envoy) is a
L7 proxy and communication bus designed for large modern service oriented
architectures. Envoy (v1.7.0+) supports an [External Authorization filter](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html)
which calls an authorization service to check if the incoming request is
authorized or not.

This feature makes it possible to delegate authorization decisions to an
external service and also makes the request context available to the service. The request context contains information
such as the source of a network activity, destination of a network activity, the network request (eg. http request).
All this information can be used by the external service to make an informed decision about the fate of the
incoming request received by Envoy.

## What is OPA-Envoy Plugin?

[OPA-Envoy](https://github.com/open-policy-agent/opa-envoy-plugin) plugin extends OPA with a gRPC server that
implements the [Envoy External Authorization API](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html).
You can use this version of OPA to enforce fine-grained, context-aware access control policies with Envoy _without_
modifying your microservice.

## How does it work?

In addition to the Envoy sidecar, your application pods will include an OPA-Envoy
sidecar. When Envoy receives API requests destined for your
microservice, it checks with OPA to decide if the request should be allowed.

Evaluating policies locally with Envoy is preferable because it
avoids introducing a network hop (which has implications on performance and
availability) in order to perform the authorization check.

{{< figure src="envoy-ext-authz-flow.png" width="150" caption="Envoy External Authorization Flow" >}}

> 💡 The OPA-Envoy plugin is frequently deployed in Kubernetes environments as a sidecar container however it can also
> be used in other environments as a standalone process running next to Envoy.

## Configuration

The OPA-Envoy plugin supports the following configuration fields:

| Field                              | Required | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ---------------------------------- | -------- |-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `plugins["envoy_ext_authz_grpc"].addr` | No       | Set listening address of Envoy External Authorization gRPC server. This must match the value configured in the Envoy config. Default: `:9191`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| `plugins["envoy_ext_authz_grpc"].path` | No       | Specifies the hierarchical policy decision path. The policy decision can either be a `boolean` or an `object`. If boolean, `true` indicates the request should be allowed and `false` indicates the request should be denied. If the policy decision is an object, it **must** contain the `allowed` key set to either `true` or `false` to indicate if the request is allowed or not respectively. It can optionally contain a `headers` field to send custom headers to the downstream client or upstream. An optional `body` field can be included in the policy decision to send a response body data to the downstream client. Also an optional `http_status` field can be included to send a HTTP response status code to the downstream client other than `403 (Forbidden)`. Default: `envoy/authz/allow`. |
| `plugins["envoy_ext_authz_grpc"].dry-run` | No       | Configures the Envoy External Authorization gRPC server to unconditionally return an `ext_authz.CheckResponse.Status` of `google_rpc.Status{Code: google_rpc.OK}`. Default: `false`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              |
| `plugins["envoy_ext_authz_grpc"].enable-reflection` | No       | Enables gRPC server reflection on the Envoy External Authorization gRPC server. Default: `false`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| `plugins["envoy_ext_authz_grpc"].proto-descriptor` | No       | Set the path to a pb that enables the capability to decode the raw body to the parsed body. Default: turns this capability off.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `plugins["envoy_ext_authz_grpc"].grpc-max-recv-msg-size` | No       | Set the max message size in bytes the gRPC server can receive. Defaults to 4MB.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `plugins["envoy_ext_authz_grpc"].grpc-max-send-msg-size` | No       | Set the max message size in bytes the gRPC server can send. Defaults to 2048MB.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| `plugins["envoy_ext_authz_grpc"].skip-request-body-parse` | No       | Specifies if the plugin should skip parsing the input request body. Default: `false`.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |



If the configuration does not specify the `path` field, `envoy/authz/allow` will be considered as the default policy
decision path. `data.envoy.authz.allow` will be the name of the policy decision to query in the default case.

The `dry-run` parameter is provided to enable you to test out new policies. You can set `dry-run: true` which will
unconditionally allow requests. Decision logs can be monitored to see what "would" have happened. This is especially
useful for initial integration of OPA or when policies undergo large refactoring.

The `enable-reflection` parameter registers the Envoy External Authorization gRPC server with reflection. After enabling
server reflection, a command line tool such as [grpcurl](https://github.com/fullstorydev/grpcurl) can be used to invoke
RPC methods on the gRPC server. See [Interacting with the gRPC server](../envoy-debugging#interacting-with-the-grpc-server)
section for more details.

Providing a file containing a protobuf descriptor set allows the plugin to decode gRPC message payloads.
So far, only unary methods using uncompressed protobuf-encoded payloads are supported.
The protoset can be generated using `protoc`, e.g. `protoc --descriptor_set_out=protoset.pb --include_imports`.

## Additional Resources

See the following pages on [envoyproxy.io](https://www.envoyproxy.io/) for more
information on external authorization:

* [External Authorization](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html)
  to learn about the External Authorization filter.
* [Network](https://www.envoyproxy.io/docs/envoy/latest/configuration/listeners/network_filters/ext_authz_filter#config-network-filters-ext-authz)
  and [HTTP](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/ext_authz_filter#config-http-filters-ext-authz)
  for details on configuring the External Authorization filter.
  
  