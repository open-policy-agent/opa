---
title: "Tutorial: Standalone Envoy"
kind: envoy
weight: 10
---

The tutorial shows how Envoy’s External
[authorization filter](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html)
can be used with OPA as an authorization service to enforce security policies over API requests
received by Envoy. The tutorial also covers examples of authoring custom
policies over the HTTP request body.

## Overview

In this tutorial we'll see how to use OPA as an External
Authorization service for the Envoy proxy. We'll do this by:

* Running a local Kubernetes cluster
* Creating a simple authorization policy in Rego and serving it via the Bundle API
* Deploying a sample application with Envoy and OPA sidecars
* Run some sample requests to see the policy in action

Note that other than the HTTP client and bundle server, all components
are co-located in the same pod.

## Running a local Kubernetes cluster

To start a local Kubernetes cluster to run our demo, we'll be using
[kind](https://kind.sigs.k8s.io/). In order to use the `kind` command,
you'll need to have Docker installed on your machine. Running
`docker info` is the easiest way to check if Docker is installed and
running.

You should see output similar to the following, showing information about
the Docker client **and** server on our machine:

```shell
$ docker info
Client:
  ...

Server:
 ...
```

If the above command shows information for both the client and server,
then Docker is installed and running.

{{< info >}}
If you haven't used `kind` before, you can find installation instructions
in the [project documentation](https://kind.sigs.k8s.io/#installation-and-usage).
{{</ info >}}

Create a cluster with the following command:

```shell
$ kind create cluster --name opa-envoy --image kindest/node:v1.27.3
Creating cluster "opa-envoy" ...
 ✓ Ensuring node image (kindest/node:v1.27.3) 🖼 
 ✓ Preparing nodes 📦  
 ✓ Writing configuration 📜 
 ✓ Starting control-plane 🕹️ 
 ✓ Installing CNI 🔌 
 ✓ Installing StorageClass 💾 
...
```

Once the cluster is created, make sure your `kubectl` context is set to connect
to the new cluster:

```shell
$ kubectl cluster-info --context kind-opa-envoy
Kubernetes control plane is running at ...
CoreDNS is running at ...
...
```

Listing the cluster nodes, should show something like this:

```shell
$ kubectl get nodes
NAME                       STATUS   ROLES           AGE     VERSION
opa-envoy-control-plane   Ready    control-plane   2m35s   v1.27.3
```

## Creating & Serving our Policy Bundle

This tutorial assumes you have some Rego knowledge, in summary the policy below does the following:

* Checks that the JWT token is valid
* Checks that the action is allowed based on the token payload `role` and the request path
* Guests have read-only access to the `/people` endpoint, admins can create users too as long as the
  name is not the same as the admin's name.

```rego
# policy.rego
package envoy.authz

import future.keywords.if

import input.attributes.request.http as http_request

default allow := false

allow if {
        is_token_valid
        action_allowed
}

is_token_valid if {
        token.valid
        now := time.now_ns() / 1000000000
        token.payload.nbf <= now
        now < token.payload.exp
}

action_allowed if {
        http_request.method == "GET"
        token.payload.role == "guest"
        glob.match("/people", ["/"], http_request.path)
}

action_allowed if {
        http_request.method == "GET"
        token.payload.role == "admin"
        glob.match("/people", ["/"], http_request.path)
}

action_allowed if {
        http_request.method == "POST"
        token.payload.role == "admin"
        glob.match("/people", ["/"], http_request.path)
        lower(input.parsed_body.firstname) != base64url.decode(token.payload.sub)
}

token := {"valid": valid, "payload": payload} if {
        [_, encoded] := split(http_request.headers.authorization, " ")
        [valid, _, payload] := io.jwt.decode_verify(encoded, {"secret": "secret"})
}
```

Create a file called `policy.rego` with the above content and store it in a ConfigMap:

```shell
kubectl create configmap authz-policy --from-file policy.rego
```

Now that the policy is stored in a ConfigMap, we can spin up an HTTP server to make it
available to as a Bundle to OPA when it's making decisions for our application:

```yaml
# bundle-server.yaml
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bundle-server
  labels:
    app: bundle-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bundle-server
  template:
    metadata:
      labels:
        app: bundle-server
    spec:
      initContainers:
        - name: opa-builder
          image: openpolicyagent/opa:latest
          args:
            - "build"
            - "--bundle"
            - "/opt/policy/"
            - "--output"
            - "/opt/output/bundle.tar.gz"
          volumeMounts:
            - name: index
              mountPath: /opt/output/
            - name: policy
              mountPath: /opt/policy/
      containers:
        - name: bundle-server
          image: nginx:1.25
          ports:
            - containerPort: 80
              name: http
          volumeMounts:
            - name: index
              mountPath: /usr/share/nginx/html
      volumes:
        - name: index
          emptyDir: {}
        - name: policy
          configMap:
            name: authz-policy
---
apiVersion: v1
kind: Service
metadata:
  name: bundle-server
spec:
  selector:
    app: bundle-server
  ports:
    - protocol: TCP
      port: 80
      targetPort: http
```

Create a file called `bundle-server.yaml` with the above content and apply it to the cluster:

```shell
kubectl apply -f bundle-server.yaml
```

Once the deployment is running, we can check that the bundle is available by running:

```shell
kubectl port-forward service/bundle-server 8080:80
```

Before checking that the bundle has been generated correctly and is available to download:

```shell
$ curl -I localhost:8080/bundle.tar.gz
HTTP/1.1 200 OK
...
```

You may now exit the port-forwarding session, the bundle server will only be accessed
from inside the cluster from now on.

## Deploying an application with Envoy and OPA sidecars

In this tutorial, we are manually configuring the Envoy proxy sidecar to intermediate
HTTP traffic from clients and our application. Envoy will consult OPA to
make authorization decisions for each request by sending `CheckRequest` messages over
a gRPC connection.

We will use the following Envoy configuration to achieve this. In summary, this
configures Envoy to:

* Listen on port `8000` for HTTP traffic
* Consult OPA for authorization decisions at 127.0.0.1:9191 & deny failing requests
* Forward requests to the application at 127.0.0.1:8080 if ok.

```yaml
# envoy.yaml
static_resources:
  listeners:
  - address:
      socket_address:
        address: 0.0.0.0
        port_value: 8000
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
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
              "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
              transport_api_version: V3
              with_request_body:
                max_request_bytes: 8192
                allow_partial_message: true
              failure_mode_allow: false
              grpc_service:
                google_grpc:
                  target_uri: 127.0.0.1:9191
                  stat_prefix: ext_authz
                timeout: 0.5s
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
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
layered_runtime:
  layers:
    - name: static_layer_0
      static_layer:
        envoy:
          resource_limits:
            listener:
              example_listener_name:
                connection_limit: 10000
        overload:
          global_downstream_max_connections: 50000
```

Create a `ConfigMap` containing the above configuration by running:

```shell
kubectl create configmap proxy-config --from-file envoy.yaml
```

Our application will be configured using a `Deployment` and `Service`.
There are a few things to note:
* the pods have an `initContainer` that configures the `iptables` rules to
  redirect traffic to the Envoy proxy.
* the `demo-test-server` container is a simple user store using in-memory state.
* the `envoy` container is configured to use the `proxy-config` `ConfigMap` we
  created earlier.
* The OPA container is configured to download policy bundles from
  the in-cluster bundle server (`bundle-server.default.svc.cluster.local`).
* The OPA license key must be set. We show how to do this in the next step.

```yaml
# app.yaml
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
          image: openpolicyagent/proxy_init:v8
          # Configure the iptables bootstrap script to redirect traffic to the
          # Envoy proxy on port 8000. Envoy will be running as 1111, and port
          # 8282 will be excluded to support OPA health checks.
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
          image: envoyproxy/envoy:v1.26.3
          volumeMounts:
            - readOnly: true
              mountPath: /config
              name: proxy-config
          args:
            - "envoy"
            - "--config-path"
            - "/config/envoy.yaml"
          env:
            - name: ENVOY_UID
              value: "1111"
        - name: opa
          image: openpolicyagent/opa:latest-envoy
          args:
            - "run"
            - "--server"
            - "--addr=localhost:8181"
            - "--diagnostic-addr=0.0.0.0:8282"
            - "--set=services.default.url=http://bundle-server"
            - "--set=bundles.default.resource=bundle.tar.gz"
            - "--set=plugins.envoy_ext_authz_grpc.addr=:9191"
            - "--set=plugins.envoy_ext_authz_grpc.path=envoy/authz/allow"
            - "--set=decision_logs.console=true"
            - "--set=status.console=true"
            - "--ignore=.*"
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
            initialDelaySeconds: 1
            periodSeconds: 3
      volumes:
        - name: proxy-config
          configMap:
            name: proxy-config
---
apiVersion: v1
kind: Service
metadata:
  name: example-app
spec:
  selector:
    app: example-app
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
```

Deploy the application and Kubernetes Service to the cluster with:

```shell
kubectl apply -f app.yaml
```

Check that everything is working by listing the pod (make sure that
all three pods are running ok).

```shell
$ kubectl get pods
NAME                             READY   STATUS    RESTARTS   AGE
bundle-server-5d7bfffdb6-bgn86   1/1     Running   0          1m
example-app-74b4bc88-5d4wh       3/3     Running   0          1m
```

## See the Policy in Action

Run a shell inside the cluster to use for testing. We will use this in-cluster
shell for the rest of the tutorial.

```shell
kubectl run curl --restart=Never -it --rm --image curlimages/curl:8.1.2 -- sh
```

Set two tokens for two users, Alice and Bob with different permissions.
As defined by our policy:

```shell
export ALICE_TOKEN="eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiZ3Vlc3QiLCAic3ViIjogIllXeHBZMlU9In0.Uk5hgUqMuUfDLvBLnlXMD0-X53aM_Hlziqg3vhOsCc8"
export BOB_TOKEN="eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiYWRtaW4iLCAic3ViIjogIlltOWkifQ.5qsm7rRTvqFHAgiB6evX0a_hWnGbWquZC0HImVQPQo8"
```

### Listing People

Send a request to list people. This should succeed for both Alice and Bob.

```shell
curl -i -H "Authorization: Bearer $ALICE_TOKEN" http://example-app/people
```
```
HTTP/1.1 200 OK
content-type: application/json
date: Tue, 18 Jul 2023 15:22:25 GMT
content-length: 96
x-envoy-upstream-service-time: 14
server: envoy

[{"id":"1","firstname":"John","lastname":"Doe"},{"id":"2","firstname":"Jane","lastname":"Doe"}]
```

And for Bob:

```shell
curl -i -H "Authorization: Bearer $BOB_TOKEN" http://example-app/people
```
```
HTTP/1.1 200 OK
...omitted...
```

### Creating People

Send a request to create a new user. This should fail for Alice but not Bob:

```shell
curl -i -H "Authorization: Bearer $ALICE_TOKEN" \
  -d '{"firstname":"Foo", "lastname":"Bar"}' -H "Content-Type: application/json" \
  -X POST http://example-app/people
```
```
HTTP/1.1 403 Forbidden
date: Tue, 18 Jul 2023 15:25:28 GMT
server: envoy
content-length: 0
```
And for Bob, the request is permitted and the user is saved with an ID

```shell
curl -i -H "Authorization: Bearer $BOB_TOKEN" \
  -d '{"firstname":"Foo", "lastname":"Bar"}' -H "Content-Type: application/json" \
  -X POST http://example-app/people
```
```
HTTP/1.1 200 OK
content-type: application/json
date: Tue, 18 Jul 2023 15:28:20 GMT
content-length: 51
x-envoy-upstream-service-time: 11
server: envoy

{"id":"498081","firstname":"Foo","lastname":"Bar"}
```

### Creating People: Conflict

Our policy also blocks users from creating users with the same name, test that
functionality with this request:

```shell
curl -i -H "Authorization: Bearer $BOB_TOKEN" \
  -d '{"firstname":"Bob", "lastname":"Bar"}' -H "Content-Type: application/json" \
  -X POST http://example-app/people
```
```
HTTP/1.1 403 Forbidden
date: Tue, 18 Jul 2023 15:31:48 GMT
server: envoy
content-length: 0
```

## Shutting Down

Exit the in-cluster shell by typing `exit`.

Delete the cluster by running:

```shell
$ kind delete cluster --name opa-envoy
Deleting cluster "opa-envoy" ...
Deleted nodes: ["opa-envoy-control-plane"]
```

## Wrap Up

Congratulations on finishing the tutorial !

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
