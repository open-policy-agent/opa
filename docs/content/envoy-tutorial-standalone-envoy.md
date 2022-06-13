---
title: "Tutorial: Standalone Envoy"
kind: envoy
weight: 10
---

The tutorial shows how Envoy’s External authorization filter can be used with
OPA as an authorization service to enforce security policies over API requests
received by Envoy. The tutorial also covers examples of authoring custom
policies over the HTTP request body.

## Prerequisites

This tutorial requires Kubernetes 1.20 or later. To run the tutorial locally, we
recommend using [minikube](https://minikube.sigs.k8s.io/docs/start/) in
version `v1.21+` with Kubernetes 1.20+.

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

import future.keywords

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

Then, build an OPA bundle.

```shell
opa build policy.rego
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

### 4. Publish OPA Bundle

We will now serve the OPA bundle created in the previous step using Nginx.

```bash
docker run --rm --name bundle-server -d -p 8888:80 -v ${PWD}:/usr/share/nginx/html:ro nginx:latest
```

The above command will start a Nginx server running on port `8888` on your host and act as a bundle server.

### 5. Create App Deployment with OPA and Envoy sidecars

Our deployment contains a sample Go app which provides information about
employees in a company. It exposes a `/people` endpoint to `get` and `create`
employees. More information can on the app be found
[here](https://github.com/ashutosh-narkar/go-test-server).

OPA is started with a configuration that sets the listening address of Envoy
External Authorization gRPC server and specifies the name of the policy decision
to query. OPA will also periodically download the policy bundle from the local Nginx server
configured in the previous step. More information on the configuration options can be found
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
        image: envoyproxy/envoy:v1.20.0
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
        # Note: openpolicyagent/opa:latest-envoy is created by retagging
        # the latest released image of OPA-Envoy.
        image: openpolicyagent/opa:{{< current_opa_envoy_docker_version >}}
        args:
        - "run"
        - "--server"
        - "--addr=localhost:8181"
        - "--diagnostic-addr=0.0.0.0:8282"
        - "--set=services.default.url=http://host.minikube.internal:8888"
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
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: proxy-config
        configMap:
          name: proxy-config
```

```bash
kubectl apply -f deployment.yaml
```

Check that the Pod shows `3/3` containers `READY` the `STATUS` as `Running`:

```bash
kubectl get pod

NAME                           READY   STATUS    RESTARTS   AGE
example-app-67c644b9cb-bbqgh   3/3     Running   0          8s
```

> The `proxy-init` container installs iptables rules to redirect all container
  traffic through the Envoy proxy sidecar. More information can be found
  [here](https://github.com/open-policy-agent/contrib/tree/main/envoy_iptables).


### 6. Create a Service to expose HTTP server

In a second terminal, start a [minikube tunnel](https://minikube.sigs.k8s.io/docs/handbook/accessing/#using-minikube-tunnel) to allow for use of the `LoadBalancer` service type.

```bash
minikube tunnel
```

In the first terminal, create a `LoadBalancer` service for the deployment.

```bash
kubectl expose deployment example-app --type=LoadBalancer --name=example-app-service --port=8080
```

Check that the Service shows an `EXTERNAL-IP`:

```bash
kubectl get service example-app-service

NAME                  TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)          AGE
example-app-service   LoadBalancer   10.109.64.199   10.109.64.199   8080:32170/TCP   5s
```

Set the `SERVICE_URL` environment variable to the service's IP/port.

**minikube:**

```bash
export SERVICE_HOST=$(kubectl get service example-app-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
export SERVICE_URL=$SERVICE_HOST:8080
echo $SERVICE_URL
```

**minikube (example):**

```
10.109.64.199:8080
```

### 7. Exercise the OPA policy

For convenience, we’ll want to store Alice's and Bob's tokens in environment variables.

```bash
export ALICE_TOKEN="eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiZ3Vlc3QiLCAic3ViIjogIllXeHBZMlU9In0.Uk5hgUqMuUfDLvBLnlXMD0-X53aM_Hlziqg3vhOsCc8"
export BOB_TOKEN="eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiYWRtaW4iLCAic3ViIjogIlltOWkifQ.5qsm7rRTvqFHAgiB6evX0a_hWnGbWquZC0HImVQPQo8"
```

Check that `Alice` can get employees **but cannot** create one.

```bash
curl -i -H "Authorization: Bearer $ALICE_TOKEN" http://$SERVICE_URL/people
curl -i -H "Authorization: Bearer $ALICE_TOKEN" -d '{"firstname":"Charlie", "lastname":"OPA"}' -H "Content-Type: application/json" -X POST http://$SERVICE_URL/people
```

Check that `Bob` can get employees and also create one.

```bash
curl -i -H "Authorization: Bearer $BOB_TOKEN" http://$SERVICE_URL/people
curl -i -H "Authorization: Bearer $BOB_TOKEN" -d '{"firstname":"Charlie", "lastname":"Opa"}' -H "Content-Type: application/json" -X POST http://$SERVICE_URL/people
```

Check that `Bob` **cannot** create an employee with the same firstname as himself.

```bash
curl -i  -H "Authorization: Bearer $BOB_TOKEN" -d '{"firstname":"Bob", "lastname":"Rego"}' -H "Content-Type: application/json" -X POST http://$SERVICE_URL/people
```

To remove the kubernetes resources created during this tutorial please use the following commands.
```bash
kubectl delete service example-app-service
kubectl delete deployment example-app
kubectl delete configmap proxy-config
```

To remove the bundle server run:
```bash
docker rm -f bundle-server
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
