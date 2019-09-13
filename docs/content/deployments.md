---
title: Deployment
kind: operations
weight: 20
---

This document helps you get OPA up and running in different deployment
environments. You should read this document if you are planning to deploy OPA.

## Docker

Docker makes OPA easy to deploy in different types of environments.

This section explains how to use the official OPA Docker images. If this is your
first time deploying OPA and you plan to use one of the Docker images, we
recommend you review this section to familiarize yourself with the basics.

OPA releases are available as images on Docker Hub.

* [openpolicyagent/opa](https://hub.docker.com/r/openpolicyagent/opa/)

### Running with Docker

If you start OPA outside of Docker without any arguments, it prints a list of
available commands. By default, the official OPA Docker image executes the `run`
command which starts an instance of OPA as an interactive shell. This is nice
for development, however, for deployments, we want to run OPA as a server.

The `run` command accepts a `--server` (or `-s`) flag that starts OPA as a
server. See `--help` for more information on other arguments. The most important
command line arguments for OPA's server mode are:

* `--addr` to set the listening address (default: `0.0.0.0:8181`).
* `--log-level` (or `-l`) to set the log level (default: `"info"`).
* `--log-format` to set the log format (default: `"json"`).

By default, OPA listens for normal HTTP connections on `0.0.0.0:8181`. To make
OPA listen for HTTPS connections, see [Security](../security).

We can run OPA as a server using Docker:

```bash
docker run -p 8181:8181 openpolicyagent/opa \
    run --server --log-level debug
```

Test that OPA is available:

```
curl -i localhost:8181/
```

#### Logging

OPA logs to stderr and the level can be set with `--log-level/-l`. The default log level is `info` which causes OPA to log request/response information.

```
{"client_addr":"[::1]:64427","level":"debug","msg":"Received request.","req_body":"","req_id":1,"req_method":"GET","req_params":{},"req_path":"/v1/data","time":"20.7.13-11T18:22:18-08:00"}
{"client_addr":"[::1]:64427","level":"debug","msg":"Sent response.","req_id":1,"req_method":"GET","req_path":"/v1/data","resp_bytes":13,"resp_duration":0.392554,"resp_status":200,"time":"20.7.13-11T18:22:18-08:00"}
```

If the log level is set to `debug` the request and response message bodies will be logged. This is useful for development however it can be expensive in production.

```
{"addrs":[":8181"],"insecure_addr":"","level":"info","msg":"First line of log stream.","time":"2019-05-08T17:25:26-07:00"}
{"level":"info","msg":"Starting decision log uploader.","plugin":"decision_logs","time":"2019-05-08T17:25:26-07:00"}
{"client_addr":"[::1]:63902","level":"info","msg":"Received request.","req_body":"","req_id":1,"req_method":"GET","req_params":{},"req_path":"/v1/data","time":"2019-05-08T17:25:41-07:00"}
{"client_addr":"[::1]:63902","level":"info","msg":"Sent response.","req_id":1,"req_method":"GET","req_path":"/v1/data","resp_body":"{\"decision_id\":\"f4b41501-2408-4a14-8269-1c1085abeda4\",\"result\":{}}","resp_bytes":66,"resp_duration":2.545972,"resp_status":200,"time":"2019-05-08T17:25:41-07:00"}
```

The default log format is json and intended for production use. For more human readable
formats use "json-pretty" or "text".

> **Note:** The `text` log format is not performance optimized or intended for production use.

#### Volume Mounts

By default, OPA does not include any data or policies.

The simplest way to load data and policies into OPA is to provide them via the
file system as command line arguments. When running inside Docker, you can
provide files via volume mounts.

```bash
docker run -v $PWD:/example openpolicyagent/opa eval -d /example 'data.example.greeting'
```

**policy.rego**:

```live:docker_hello_world:module:read_only
package example

greeting = msg {
    info := opa.runtime()
    hostname := info.env["HOSTNAME"] # Docker sets the HOSTNAME environment variable.
    msg := sprintf("hello from container %q!", [hostname])
}
```

#### More Information

For more information on OPA's command line, see `--help`:

```
docker run openpolicyagent/opa run --help
```

### Tagging

The Docker Hub repository contains tags for every release of OPA. For more
information on each release see the [GitHub
Releases](https://github.com/open-policy-agent/opa/releases) page.

The "latest" tag refers to the most recent release. The latest tag is convenient
if you want to quickly try out OPA however for production deployments, we
recommend using an explicit version tag.

Development builds are also available on Docker Hub. For each version the
`{version}-dev` tag refers the most recent development build for that version.

The `edge` tag refers to the current `master` branch of OPA. Useful for testing
unreleased features. It is not recommended to use `edge` for production deployments.

The version information is contained in the OPA executable itself. You can check
the version with the following command:

```bash
docker run openpolicyagent/opa version
```

## Kubernetes

### Kicking the Tires

This section shows how to quickly deploy OPA on top of Kubernetes to try it out.

> If you are interested in using OPA to enforce admission control policies in
> Kubernetes, see the [Kubernetes Admission Control
> Tutorial](../kubernetes-tutorial).

> These steps assume Kubernetes is deployed with
[minikube](https://github.com/kubernetes/minikube). If you are using a different
Kubernetes provider, the steps should be similar. You may need to use a
different Service configuration at the end.

First, create a ConfigMap containing a test policy.

In this case, the policy file does not contain sensitive information so it's
fine to store as a ConfigMap. If the file contained sensitive information, then
we recommend you store it as a Secret.

**example.rego**:

```live:k8s_deployment_hello_world:module:read_only
package example

greeting = msg {
    info := opa.runtime()
    hostname := info.env["HOSTNAME"] # Kubernetes sets the HOSTNAME environment variable.
    msg := sprintf("hello from pod %q!", [hostname])
}
```

```bash
kubectl create configmap example-policy --from-file example.rego
```

Next, create a Deployment to run OPA. The ConfigMap containing the policy is
volume mounted into the container. This allows OPA to load the policy from
the file system.

**deployment-opa.yaml**:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: opa
  labels:
    app: opa
spec:
  replicas: 1
  template:
    metadata:
      labels:
        app: opa
      name: opa
    spec:
      containers:
      - name: opa
        image: openpolicyagent/opa:{{< current_docker_version >}}
        ports:
        - name: http
          containerPort: 8181
        args:
        - "run"
        - "--ignore=.*"  # exclude hidden dirs created by Kubernetes
        - "--server"
        - "/policies"
        volumeMounts:
        - readOnly: true
          mountPath: /policies
          name: example-policy
      volumes:
      - name: example-policy
        configMap:
          name: example-policy
```

```bash
kubectl create -f deployment-opa.yaml
```

At this point OPA is up and running. Create a Service to expose the OPA API so
that you can query it:

**service-opa.yaml**:

```yaml
kind: Service
apiVersion: v1
metadata:
  name: opa
  labels:
    app: opa
spec:
  type: NodePort
  selector:
    app: opa
  ports:
    - name: http
      protocol: TCP
      port: 8181
      targetPort: 8181
```

```bash
kubectl create -f service-opa.yaml
```

Get the URL of OPA using `minikube`:

```bash
OPA_URL=$(minikube service opa --url)
```

Now you can query OPA's API:

```bash
curl $OPA_URL/v1/data
```

OPA will respond with the greeting from the policy (the pod hostname will differ):

```json
{
  "result": {
    "example": {
      "greeting": "hello from pod \"opa-78ccdfddd-xplxr\"!"
    }
  }
}
```

### Readiness and Liveness Probes

OPA exposes a `/health` API endpoint that you can configure Kubernetes
[Readiness and Liveness
Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-probes/)
to call. For example:

```yaml
containers:
- name: opa
  image: openpolicyagent/opa:{{< current_docker_version >}}
  ports:
  - name: http
    containerPort: 8181
  args:
  - "run"
  - "--ignore=.*"               # exclude hidden dirs created by Kubernetes
  - "--server"
  - "/policies"
  volumeMounts:
  - readOnly: true
    mountPath: /policies
    name: example-policy
  livenessProbe:
    httpGet:
      scheme: HTTP              # assumes OPA listens on localhost:8181
      port: 8181
    initialDelaySeconds: 5      # tune these periods for your environemnt
    periodSeconds: 5
  readinessProbe:
    httpGet:
      path: /health?bundle=true  # Include bundle activation in readiness
      scheme: HTTP
      port: 8181
    initialDelaySeconds: 5
    periodSeconds: 5
```

See the [Health API](/docs/{{< current_version >}}/rest-api#health-api) documentation for more detail on the `/health` API endpoint.

## HTTP Proxies

OPA uses the standard Go [net/http](https://golang.org/pkg/net/http/) package
for outbound HTTP requests that download bundles, upload decision logs, etc. In
environments where an HTTP proxy is required, you can configure OPA using the
pseudo-standard `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` environment
variables.