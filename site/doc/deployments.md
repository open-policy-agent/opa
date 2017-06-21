# Deployments

This document helps you get OPA up and running in different deployment
environments. You should read this document if you are planning to deploy OPA.

## Docker

Docker makes OPA easy to deploy in different types of environments.

This section explains how to use the official OPA Docker images. If this is your
first time deploying OPA and you plan to use one of the Docker images, we
recommend you review this section to familiarize yourself with the basics.

OPA releases are available as images on Docker Hub.

* [openpolicyagent/opa:0.4.10](https://hub.docker.com/r/openpolicyagent/opa/)

### Running

If you start OPA outside of Docker without any arguments, it prints a list of
available commands. By default, the official OPA Docker image executes the `run`
command which starts an instance of OPA as an interactive shell. This is nice
for development, however, for deployments, we want to run OPA as a server.

The `run` command accepts a `--server` (or `-s`) flag that starts OPA as a
server. See `--help` for more information on other arguments. The most important
command line arguments for OPA's server mode are:

* `--addr` to set the listening address (default: `0.0.0.0:8181`).
* `--log-level` (or `-l`) to set the log level (default: `"info"`).
* `--log-format` to set the log format (default: `"text"`).

By default, OPA listens for normal HTTP connections on `0.0.0.0:8181`. To make
OPA listen for HTTPS connections, see [Security](../security/).

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

OPA logs to stderr and the level can be set with `--log-level/-l`. The default log level is `info` which causes OPA to log request/response headers.

```
time="2017-03-12T03:03:23Z" level=info msg="First line of log stream." addr=":8181"
time="2017-03-12T03:03:30Z" level=debug msg="Received request." client_addr="172.17.0.1:60952" req_body= req_id=1 req_method=GET req_params=map[] req_path="/v1/data"
time="2017-03-12T03:03:30Z" level=debug msg="Sent response." client_addr="172.17.0.1:60952" req_id=1 req_method=GET req_path="/v1/data" resp_bytes=13 resp_duration=0.402793 resp_status=200
```

If the log level is set to `debug` the request and response message bodies will be logged. This is useful for development however it can be expensive in production.

```
time="2017-05-16T00:09:24Z" level=info msg="Received request." client_addr="172.17.0.1:42164" req_body="{"input": "hello"}" req_id=1 req_method=POST req_params=map[] req_path="/v1/data"
time="2017-05-16T00:09:24Z" level=info msg="Sent response." client_addr="172.17.0.1:42164" req_id=1 req_method=POST req_path="/v1/data" resp_body="{"result":{"example":{"message":"world"}}}" resp_bytes=42 resp_duration=0.618689 resp_status=200
```

The default log format is text-based and intended for development. For
production, enable JSON formatting with `--log-format json`:

```
{"client_addr":"[::1]:64427","level":"debug","msg":"Received request.","req_body":"","req_id":1,"req_method":"GET","req_params":{},"req_path":"/v1/data","time":"2017-03-11T18:22:18-08:00"}
{"client_addr":"[::1]:64427","level":"debug","msg":"Sent response.","req_id":1,"req_method":"GET","req_path":"/v1/data","resp_bytes":13,"resp_duration":0.392554,"resp_status":200,"time":"2017-03-11T18:22:18-08:00"}
```

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
        - "/policies/example.rego"
        volumeMounts:
        - readOnly: true
          mountPath: /policies
          name: example-policy
```

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

```bash
curl $OPA_URL/v1/data -d @example_pod.json
```
