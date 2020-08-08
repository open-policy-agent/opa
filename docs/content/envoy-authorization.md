---
title: Envoy
kind: tutorial
weight: 8
---

[Envoy](https://www.envoyproxy.io/docs/envoy/v1.10.0/intro/what_is_envoy) is a
L7 proxy and communication bus designed for large modern service oriented
architectures. Envoy (v1.7.0+) supports an [External Authorization
filter](https://www.envoyproxy.io/docs/envoy/v1.10.0/intro/arch_overview/ext_authz_filter.html)
which calls an authorization service to check if the incoming request is
authorized or not.

This feature makes it possible to delegate authorization decisions to an
external service and also makes the request context available to the service
which can then be used to make an informed decision about the fate of the
incoming request received by Envoy.

## Goals

The tutorial shows how Envoy’s External authorization filter can be used with
OPA as an authorization service to enforce security policies over API requests
received by Envoy. The tutorial also covers examples of authoring custom
policies over the HTTP request body.

## Prerequisites

This tutorial requires Kubernetes 1.14 or later. To run the tutorial locally, we
recommend using
[minikube](https://kubernetes.io/docs/getting-started-guides/minikube) in
version `v1.0+` with Kubernetes 1.14 (which is the default).

## Steps

### 1. Start Minikube

```bash
minikube start
```

### 2. Create ConfigMap containing configuration for Envoy

The Envoy configuration below defines an external authorization filter
`envoy.ext_authz` for a gRPC authorization server.

Save the configuration as **envoy.yaml**:

```yaml
static_resources:
  listeners:
    - address:
        socket_address:
          address: 0.0.0.0
          port_value: 8000
      filter_chains:
        - filters:
            - name: envoy.http_connection_manager
              typed_config:
                "@type": type.googleapis.com/envoy.config.filter.network.http_connection_manager.v2.HttpConnectionManager
                codec_type: auto
                stat_prefix: ingress_http
                route_config:
                  name: local_route
                  virtual_hosts:
                    - name: backend
                      domains:
                        - "*"
                      routes:
                        - match:
                            prefix: "/"
                          route:
                            cluster: service
                http_filters:
                  - name: envoy.ext_authz
                    typed_config:
                      "@type": type.googleapis.com/envoy.config.filter.http.ext_authz.v2.ExtAuthz
                      with_request_body:
                        max_request_bytes: 8192
                        allow_partial_message: true
                      failure_mode_allow: false
                      grpc_service:
                        google_grpc:
                          target_uri: 127.0.0.1:9191
                          stat_prefix: ext_authz
                        timeout: 0.5s
                  - name: envoy.router
                    typed_config: {}
  clusters:
    - name: service
      connect_timeout: 0.25s
      type: strict_dns
      lb_policy: round_robin
      load_assignment:
        cluster_name: service
        endpoints:
          - lb_endpoints:
              - endpoint:
                  address:
                    socket_address:
                      address: 127.0.0.1
                      port_value: 8080
admin:
  access_log_path: "/dev/null"
  address:
    socket_address:
      address: 0.0.0.0
      port_value: 8001
```

Create the ConfigMap:

```bash
kubectl create configmap proxy-config --from-file envoy.yaml
```

### 3. Define a OPA policy

The following OPA policy restricts access to the `/people` endpoint exposed by
our sample app:

* Alice is granted a **guest** role and can perform a `GET` request to `/people`.
* Bob is granted an **admin** role and can perform a `GET` and `POST` request to `/people`.

The policy also restricts an `admin` user, in this case `bob` from creating an
employee with the same `firstname` as himself.

**policy.rego**

```live:example:module:openable
package envoy.authz

import input.attributes.request.http as http_request

default allow = false

token = {"valid": valid, "payload": payload} {
    [_, encoded] := split(http_request.headers.authorization, " ")
    [valid, _, payload] := io.jwt.decode_verify(encoded, {"secret": "secret"})
}

allow {
    is_token_valid
    action_allowed
}

is_token_valid {
  token.valid
  now := time.now_ns() / 1000000000
  token.payload.nbf <= now
  now < token.payload.exp
}

action_allowed {
  http_request.method == "GET"
  token.payload.role == "guest"
  glob.match("/people*", [], http_request.path)
}

action_allowed {
  http_request.method == "GET"
  token.payload.role == "admin"
  glob.match("/people*", [], http_request.path)
}

action_allowed {
  http_request.method == "POST"
  token.payload.role == "admin"
  glob.match("/people", [], http_request.path)
  lower(input.parsed_body.firstname) != base64url.decode(token.payload.sub)
}
```

Store the policy in Kubernetes as a Secret.

```bash
kubectl create secret generic opa-policy --from-file policy.rego
```

In the next step, OPA is configured to query for the `data.envoy.authz.allow`
decision. If the response is `true` the operation is allowed, otherwise the
operation is denied. Sample input received by OPA is shown below:

```live:example:query:hidden
data.envoy.authz.allow
```

```live:example:input
{
  "attributes": {
      "request": {
          "http": {
              "method": "GET",
                "path": "/people",
                "headers": {
                  "authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiZ3Vlc3QiLCJzdWIiOiJZV3hwWTJVPSIsIm5iZiI6MTUxNDg1MTEzOSwiZXhwIjoxNjQxMDgxNTM5fQ.K5DnnbbIOspRbpCr2IKXE9cPVatGOCBrBQobQmBmaeU"
                }
            }
        }
    }
}
```

With the input value above, the answer is:

```live:example:output
```

An example of the complete input received by OPA can be seen [here](https://github.com/open-policy-agent/opa-envoy-plugin#example-input).

> In typical deployments the policy would either be built into the OPA container
> image or it would fetched dynamically via the [Bundle
> API](https://www.openpolicyagent.org/docs/latest/bundles/). ConfigMaps are
> used in this tutorial for test purposes.

### 4. Create App Deployment with OPA and Envoy sidecars

Our deployment contains a sample Go app which provides information about
employees in a company. It exposes a `/people` endpoint to `get` and `create`
employees. More information can on the app be found
[here](https://github.com/ashutosh-narkar/go-test-server).

OPA is started with a configuration that sets the listening address of Envoy
External Authorization gRPC server and specifies the name of the policy decision
to query. More information on the configuration options can be found
[here](https://github.com/open-policy-agent/opa-envoy-plugin#configuration).

Save the deployment as **deployment.yaml**:

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: example-app
  labels:
    app: example-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: example-app
  template:
    metadata:
      labels:
        app: example-app
    spec:
      initContainers:
        - name: proxy-init
          image: openpolicyagent/proxy_init:v5
          # Configure the iptables bootstrap script to redirect traffic to the
          # Envoy proxy on port 8000, specify that Envoy will be running as user
          # 1111, and that we want to exclude port 8282 from the proxy for the
          # OPA health checks. These values must match up with the configuration
          # defined below for the "envoy" and "opa" containers.
          args: ["-p", "8000", "-u", "1111", "-w", "8282"]
          securityContext:
            capabilities:
              add:
              - NET_ADMIN
            runAsNonRoot: false
            runAsUser: 0
      containers:
        - name: app
          image: openpolicyagent/demo-test-server:v1
          ports:
            - containerPort: 8080
        - name: envoy
          image: envoyproxy/envoy:v1.10.0
          securityContext:
            runAsUser: 1111
          volumeMounts:
          - readOnly: true
            mountPath: /config
            name: proxy-config
          args:
          - "envoy"
          - "--config-path"
          - "/config/envoy.yaml"
        - name: opa
          # Note: openpolicyagent/opa:latest-envoy is created by retagging
          # the latest released image of OPA-Envoy.
          image: openpolicyagent/opa:{{< current_opa_envoy_docker_version >}}
          securityContext:
            runAsUser: 1111
          volumeMounts:
          - readOnly: true
            mountPath: /policy
            name: opa-policy
          args:
          - "run"
          - "--server"
          - "--addr=localhost:8181"
          - "--diagnostic-addr=0.0.0.0:8282"
          - "--set=plugins.envoy_ext_authz_grpc.addr=:9191"
          - "--set=plugins.envoy_ext_authz_grpc.query=data.envoy.authz.allow"
          - "--set=decision_logs.console=true"
          - "--ignore=.*"
          - "/policy/policy.rego"
          livenessProbe:
            httpGet:
              path: /health?plugins
              scheme: HTTP
              port: 8282
            initialDelaySeconds: 5
            periodSeconds: 5
          readinessProbe:
            httpGet:
              path: /health?plugins
              scheme: HTTP
              port: 8282
            initialDelaySeconds: 5
            periodSeconds: 5
      volumes:
        - name: proxy-config
          configMap:
            name: proxy-config
        - name: opa-policy
          secret:
            secretName: opa-policy
```

```bash
kubectl apply -f deployment.yaml
```

> The `proxy-init` container installs iptables rules to redirect all container
  traffic through the Envoy proxy sidecar. More information can be found
  [here](https://github.com/open-policy-agent/contrib/tree/master/envoy_iptables).


### 5. Create a Service to expose HTTP server

```bash
kubectl expose deployment example-app --type=NodePort --name=example-app-service  --port=8080
```

Set the `SERVICE_URL` environment variable to the service's IP/port.

**minikube:**

```bash
export SERVICE_PORT=$(kubectl get service example-app-service -o jsonpath='{.spec.ports[?(@.port==8080)].nodePort}')
export SERVICE_HOST=$(minikube ip)
export SERVICE_URL=$SERVICE_HOST:$SERVICE_PORT
echo $SERVICE_URL
```

**minikube (example):**

```bash
192.168.99.113:31056
```

### 6. Exercise the OPA policy

For convenience, we’ll want to store Alice's and Bob's tokens in environment variables.

```bash
export ALICE_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiZ3Vlc3QiLCJzdWIiOiJZV3hwWTJVPSIsIm5iZiI6MTUxNDg1MTEzOSwiZXhwIjoxNjQxMDgxNTM5fQ.K5DnnbbIOspRbpCr2IKXE9cPVatGOCBrBQobQmBmaeU"
export BOB_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYWRtaW4iLCJzdWIiOiJZbTlpIiwibmJmIjoxNTE0ODUxMTM5LCJleHAiOjE2NDEwODE1Mzl9.WCxNAveAVAdRCmkpIObOTaSd0AJRECY2Ch2Qdic3kU8"
```

Check that `Alice` can get employees **but cannot** create one.

```bash
curl -i -H "Authorization: Bearer "$ALICE_TOKEN"" http://$SERVICE_URL/people
curl -i -H "Authorization: Bearer "$ALICE_TOKEN"" -d '{"firstname":"Charlie", "lastname":"OPA"}' -H "Content-Type: application/json" -X POST http://$SERVICE_URL/people
```

Check that `Bob` can get employees and also create one.

```bash
curl -i -H "Authorization: Bearer "$BOB_TOKEN"" http://$SERVICE_URL/people
curl -i -H "Authorization: Bearer "$BOB_TOKEN"" -d '{"firstname":"Charlie", "lastname":"Opa"}' -H "Content-Type: application/json" -X POST http://$SERVICE_URL/people
```

Check that `Bob` **cannot** create an employee with the same firstname as himself.

```bash
curl -i  -H "Authorization: Bearer "$BOB_TOKEN"" -d '{"firstname":"Bob", "lastname":"Rego"}' -H "Content-Type: application/json" -X POST http://$SERVICE_URL/people
```

## Wrap Up

Congratulations for finishing the tutorial !

This tutorial showed how to use OPA as an External authorization service to
enforce custom policies by leveraging Envoy’s External authorization filter.

This tutorial also showed a sample OPA policy that returns a `boolean` decision
to indicate whether a request should be allowed or not.

Envoy's external authorization filter allows optional response headers and body
to be sent to the downstream client or upstream. An example of a rule that
returns an object that not only indicates if a request is allowed or not but
also provides optional response headers, body and HTTP status that can be sent
to the downstream client or upstream can be seen
[here](https://github.com/open-policy-agent/opa-envoy-plugin#example-policy-with-object-response).
