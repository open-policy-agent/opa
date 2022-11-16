---
title: "Tutorial: Gloo Edge"
kind: envoy
weight: 11
---

[Gloo Edge](https://docs.solo.io/gloo-edge/latest/) is an Envoy based API Gateway that provides a Kubernetes CRD to manage Envoy configuration for performing traffic management and routing.

Gloo Edge allows creation of a [Custom External Auth Service](https://docs.solo.io/gloo-edge/master/guides/security/auth/custom_auth/) that implements the Envoy spec for an [External Authorization Server](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html).

The purpose of this tutorial is to show how OPA could be used with Gloo Edge to apply security policies for upstream services.

## Prerequisites

This tutorial requires Kubernetes 1.14 or later. To run the tutorial locally, we recommend using [minikube](https://minikube.sigs.k8s.io/docs/start/) in version v1.0+ with Kubernetes 1.14+.

The tutorial also requires [Helm](https://helm.sh/docs/intro/install/) to install Gloo Edge on a Kubernetes cluster.

## Steps

### 1. Start Minikube

```bash
minikube start
```

### 2. Setup and Configure Gloo Edge

```bash
helm repo add gloo https://storage.googleapis.com/solo-public-helm
helm upgrade --install --namespace gloo-system --create-namespace gloo gloo/gloo
kubectl config set-context $(kubectl config current-context) --namespace=gloo-system
```

Ensure all the pods are running using `kubectl get pod` command.

### 3. Create Virtual Service and Upstream

[Virtual Services](https://docs.solo.io/gloo-edge/latest/introduction/architecture/concepts/#virtual-services) define a set of route rules, security configuration, rate limiting, transformations, and other core routing capabilities supported by Gloo Edge.

[Upstreams](https://docs.solo.io/gloo-edge/latest/introduction/architecture/concepts/#upstreams) define destinations for routes.

Save the configuration as **vs.yaml**.

```yaml
apiVersion: gloo.solo.io/v1
kind: Upstream
metadata:
  name: httpbin
spec:
  static:
    hosts:
      - addr: httpbin.org
        port: 80
---
apiVersion: gateway.solo.io/v1
kind: VirtualService
metadata:
  name: httpbin
spec:
  virtualHost:
    domains:
      - '*'
    routes:
      - matchers:
         - prefix: /
        routeAction:
          single:
            upstream:
              name: httpbin
              namespace: gloo-system
        options:
          autoHostRewrite: true
```

```bash
kubectl apply -f vs.yaml
```

### 4. Test Gloo

For simplification port-forwarding will be used. Open another terminal and execute.

```bash
kubectl port-forward deployment/gateway-proxy 8080:8080
```

The `VirtualService` created in the previous step forwards requests to http://httpbin.org

Let's test that Gloo works properly by running the below command in the first terminal.

```bash
curl -XGET -Is localhost:8080/get | head -n 1
HTTP/1.1 200 OK

curl http -XPOST -Is localhost:8080/post | head -n1
HTTP/1.1 200 OK
```

### 5. Define an OPA Policy

The following OPA policy will work as follows:

* Alice is granted a **guest** role and can perform `GET` requests.
* Bob is granted an **admin** role and can perform `GET` and `POST` requests.

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
}

action_allowed if {
	http_request.method == "GET"
	token.payload.role == "admin"
}

action_allowed if {
	http_request.method == "POST"
	token.payload.role == "admin"
}

token := {"valid": valid, "payload": payload} if {
	[_, encoded] := split(http_request.headers.authorization, " ")
	[valid, _, payload] := io.jwt.decode_verify(encoded, {"secret": "secret"})
}
```

A sample input can be seen below using Alice's token, Alice should be able to `GET` but not `POST`

```live:example:input
{
  "attributes": {
    "request": {
      "http": {
        "method": "GET",
        "headers": {
          "authorization": "Bearer eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiZ3Vlc3QiLCAic3ViIjogIllXeHBZMlU9In0.Uk5hgUqMuUfDLvBLnlXMD0-X53aM_Hlziqg3vhOsCc8"
        }
      }
    }
  }
}
```

With the input value above, the answer is:

```live:example:output
```

Next we build an OPA bundle.

```bash
opa build policy.rego
```

And now we serve the OPA bundle created above using Nginx.

```bash
docker run --rm --name bundle-server -d -p 8888:80 -v ${PWD}:/usr/share/nginx/html:ro nginx:latest
```

### 6. Setup OPA-Envoy

Create a deployment as shown below and save it in **deployments.yaml**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: opa
  labels:
    app: opa
spec:
  replicas: 1
  selector:
    matchLabels:
      app: opa
  template:
    metadata:
      labels:
        app: opa
    spec:
      containers:
      - name: opa
        image: openpolicyagent/opa:{{< current_opa_envoy_docker_version >}}
        volumeMounts:
          - readOnly: true
            mountPath: /policy
            name: opa-policy
        args:
          - "run"
          - "--server"
          - "--addr=0.0.0.0:8181"
          - "--set=services.default.url=http://host.minikube.internal:8888"
          - "--set=bundles.default.resource=bundle.tar.gz"
          - "--set=plugins.envoy_ext_authz_grpc.addr=0.0.0.0:9191"
          - "--set=plugins.envoy_ext_authz_grpc.path=envoy/authz/allow"
          - "--set=decision_logs.console=true"
          - "--set=status.console=true"
          - "--ignore=.*"
      volumes:
      - name: opa-policy
```

```bash
kubectl apply -f deployments.yaml
```

Ensure all pods are running using `kubectl get pod` command.

Next, define a Kubernetes `service` for OPA-Envoy. This is required to create a DNS record and thereby create a Gloo `Upstream` object.

**service.yaml**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: opa
spec:
  selector:
    app: opa
  ports:
    - name: grpc
      protocol: TCP
      port: 9191
      targetPort: 9191
```
**Note**: Since the name of the service port is `grpc`, `Gloo` will understand that traffic should be routed using HTTP2 protocol.

`kubectl apply -f service.yaml`

### 7. Configure Gloo Edge to use OPA

To use OPA as a custom auth server, we need to add the `extauth` attribute as described below:

**gloo.yaml**

```yaml
global:
  extensions:
    extAuth:
      extauthzServerRef:
        name: gloo-system-opa-9191
        namespace: gloo-system
```

To apply it, run the following command:

```bash
helm upgrade --install --namespace gloo-system --create-namespace -f gloo.yaml gloo gloo/gloo
```

Configure Gloo Edge routes to perform authorization via configured extauth before regular processing.

**vs-patch.yaml**

```yaml
spec:
  virtualHost:
    options:
      extauth:
        customAuth: {}
```

Then apply the patch to our `VirtualService` as shown below:

```bash
kubectl patch vs httpbin --type=merge --patch "$(cat vs-patch.yaml)"
```

### 8. Exercise the OPA Policy

Before we exercise the policy, for convenience sake, we will want to store Alice and Bob's tokens in environment variables as such:

```bash
export ALICE_TOKEN="eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiZ3Vlc3QiLCAic3ViIjogIllXeHBZMlU9In0.Uk5hgUqMuUfDLvBLnlXMD0-X53aM_Hlziqg3vhOsCc8"
export BOB_TOKEN="eyJhbGciOiAiSFMyNTYiLCAidHlwIjogIkpXVCJ9.eyJleHAiOiAyMjQxMDgxNTM5LCAibmJmIjogMTUxNDg1MTEzOSwgInJvbGUiOiAiYWRtaW4iLCAic3ViIjogIlltOWkifQ.5qsm7rRTvqFHAgiB6evX0a_hWnGbWquZC0HImVQPQo8"
```

Now let's verify that OPA only allows **Alice** to perform `GET` requests.

```bash
curl -XGET -Is -H "Authorization: Bearer $ALICE_TOKEN" localhost:8080/get
HTTP/1.1 200 OK
```
And with a `POST` request, we get:

```bash
curl http -XPOST -Is -H "Authorization: Bearer $ALICE_TOKEN" localhost:8080/post
HTTP/1.1 403 Forbidden
```

And for **Bob**, we should be able to `GET` and `POST`:

```bash
curl -XGET -Is -H "Authorization: Bearer $BOB_TOKEN" localhost:8080/get    
HTTP/1.1 200 OK
```

And the `POST`:

```bash
curl http -XPOST -Is -H "Authorization: Bearer $BOB_TOKEN" localhost:8080/post
HTTP/1.1 200 OK
```

Check OPA's decision logs to view the inputs received by OPA from Gloo Edge and the results generated by OPA.

```bash
kubectl logs deployment/opa -n gloo-system
```

## Wrap Up

Congratulations for finishing the tutorial!

This tutorial showed how you can use OPA with [Gloo Edge](https://docs.solo.io/gloo-edge/latest/) to apply security policies for upstream services and how to create and test a policy that would allow `GET` or `POST` requests based on your user role.