---
nav_id: MAIN_DOCUMENTATION
doc_id: DEPLOYMENTS
layout: documentation

title: Deployments
---

{% contentfor header %}

# Deployments

This document helps you get OPA up and running in different deployment
environments. You should read this document if you are planning to deploy OPA.

{% endcontentfor %}

{% contentfor body %}

## Docker

Docker makes OPA easy to deploy in different types of environments.

This section explains how to use the official OPA Docker images. If this is your
first time deploying OPA and you plan to use one of the Docker images, we
recommend you review this section to familiarize yourself with the basics.

OPA releases are available as images on Docker Hub.

* [openpolicyagent/opa:0.4.5](https://hub.docker.com/r/openpolicyagent/opa/){: .opa-deployments--docker-hub-list--link}
{: .opa-deployments--docker-hub-list}

### Running

If you start OPA outside of Docker without any arguments, it prints a list of
available commands. By default, the official OPA Docker image executes the `run`
command which starts an instance of OPA as an interactive shell. This is nice
for development, however, for deployments, we want to run OPA as a server.

The `run` command accepts a `--server` (or `-s`) flag that starts OPA as a
server. See `--help` for more information on other arguments. The most important
command line arguments for OPA's server mode are:

* `--addr` to set the listening address (default: `0.0.0.0:8181`).
* `--v=N` to set the logging level.
* `--logtostderr=1` send logs to STDERR instead of file.

By default, OPA listens for normal HTTP connections on `0.0.0.0:8181`. To make
OPA listen for HTTPS connections, see [Security](../security/).

We can run OPA as a server using Docker:

```bash
docker run -p 8181:8181 openpolicyagent/opa \
    run --server --logtostderr=1 --v=2
```

Test that OPA is available:

```
curl -i localhost:8181/
```

#### Logging

OPA focuses on providing debugging support through well-defined APIs and
whenever possible, OPA returns detailed, structured error messages to clients.
By default, OPA only emits log messages in response to critical internal events.
When OPA is healthy, the log stream is quiet.

The `--v=N` flag controls log verbosity:

* `--v=2` enables API response logging.
* `--v=3` enables API request AND response logging.

**Response Logging**

With response logging enabled, API response metadata is logged:

```
I0308 20:20:16.107720       1 logging.go:43] 172.17.0.1:57000 GET /v1/policies 200 18 0.821549ms
```

These log messages include:

* Client IP:Port
* HTTP request method and path
* HTTP response status and message body size
* Total request processing time

**Request Logging**

With request logging enabled, the entire API request is logged:

```
I0308 20:23:11.375300       1 logging.go:42] rid=1: GET /v1/policies HTTP/1.1
I0308 20:23:11.375313       1 logging.go:42] rid=1: Host: localhost:8181
I0308 20:23:11.375316       1 logging.go:42] rid=1: Accept: */*
I0308 20:23:11.375319       1 logging.go:42] rid=1: User-Agent: curl/7.51.0
I0308 20:23:11.375321       1 logging.go:42] rid=1:
I0308 20:23:11.375324       1 logging.go:42] rid=1:
I0308 20:23:11.375728       1 logging.go:60] rid=1: 172.17.0.1:57004 GET /v1/policies 200 18 0.453044ms
```

> The format of these log messages may change in the future.

#### Volume Mounts

By default, OPA does not include any data or policies.

The simplest way to load data and policies into OPA is to provide them via the
file system as command line arguments. When running inside Docker, you can
provide files via volume mounts.

```bash
docker run -v $PWD/example:/example openpolicyagent/opa \
    run -e 'data.example.greeting' \
    /example
```

**$PWD/example/data.json**:

```json
{
    "hostOS": "$(uname)"
}
```

**$PWD/example/policy.rego**:

```ruby
package example

greeting = msg {
    concat("", ["Hello ", data.example.hostOS, "!"], msg)
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

The version information is contained in the OPA executable itself. You can check
the version with the following command:

```bash
docker run openpolicyagent/opa version
```

## Kubernetes

### Kicking the Tires

This section shows how to quickly deploy OPA on top of Kubernetes to try it out.

> These steps assume Kubernetes is deployed with
[minikube](https://github.com/kubernetes/minikube). If you are using a different
Kubernetes provider, the steps should be similar. You may need to use a
different Service configuration at the end.
{: .opa-tip}

First, create a ConfigMap containing a test policy. The policy will inspect
"pod" objects provided as input. If the pod is missing a "customer" label or the
pod includes containers that refer to images outside the acmecorp registry,
"allow" will be false.

In this case, the policy file does not contain sensitive information so it's
fine to store as a ConfigMap. If the file contained sensitive information, then
we recommend you store it as a Secret.

```ruby
package example

import input.pod

default allow = true

allow = false {
    not pod.metadata.labels.customer
}

allow = false {
    container = pod.spec.containers[_]
    not re_match("^registry.acmecorp.com/.+$", container.image)
}
```
{: .opa-collapse--ignore}

```bash
kubectl create configmap example-policy \
    --from-file example.rego
```

Next, create a ReplicationController to deploy OPA. The ConfigMap containing the
policy is volume mounted into the container. This allows OPA to load the policy
from the file system.

```yaml
kind: ReplicationController
apiVersion: v1
metadata:
  name: opa
spec:
  replicas: 1
  selector:
    app: opa
  template:
    metadata:
      labels:
        app: opa
    spec:
      volumes:
      - name: example-policy
        configMap:
          name: example-policy
      containers:
      - name: opa
        image: openpolicyagent/opa
        ports:
        - name: http
          containerPort: 8181
        args:
        - "run"
        - "--server"
        - "--v=2"
        - "--logtostderr=1"
        - "/policies/example.rego"
        volumeMounts:
        - readOnly: true
          mountPath: /policies
          name: example-policy
```
{: .opa-collapse--ignore}

```bash
kubectl create -f rc_opa.yaml
```

At this point OPA is up and running. Create a Service to expose the OPA API so
that you can query it:

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
{: .opa-collapse--ignore}

```bash
kubectl create -f svc_opa.yaml
```

Get the URL of OPA using `minikube`:

```bash
OPA_URL=$(minikube service opa --url)
```

Exercise the OPA API. Note that the container below references an image outside
our hypothetical repository:

```json
{
    "input": {
        "kind": "Pod",
        "apiVersion": "v1",
        "metadata": {
            "name": "opa",
            "labels": {
                "customer": "example.org"
            }
        },
        "spec": {
            "containers": [
                {
                    "name": "opa",
                    "image": "openpolicyagent/opa"
                }
            ]
        }
    }
}
```
{: .opa-collapse--ignore}

```bash
curl $OPA_URL/v1/data -d @example_pod.json
```

{% endcontentfor %}
