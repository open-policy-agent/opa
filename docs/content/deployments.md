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

greeting := msg {
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

The `edge` tag refers to the current `main` branch of OPA. Useful for testing
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

greeting := msg {
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

## CPU and Memory Requirements

For more information see the [Resource Utilization section on the Policy Performance page](../policy-performance#resource-utilization).

## Operational Readiness and Failure Modes

Depending on how you deploy OPA, it may or may not have policies available as soon as it starts up.  If OPA starts making decisions without any policies, it will return `undefined` as an answer to all policy queries.  This can be problematic because even though OPA returns a response, it has not actually returned the decision dictated by policy.

For example, without loading any policies into OPA whatsoever, a policy query will return the answer `undefined`, which via the HTTP API is represented as an empty JSON object `{}`.

```
$ opa run -s
$ curl localhost:8181/v1/data/foo/bar
{}
```

In contrast, when policies are loaded, OPA is operationally ready for policy queries, and the answer is defined, the answer is a JSON object of the form `{"result": ...}`
```
$ opa run foo.rego -s
$ curl localhost:8181/v1/data/foo/bar
{"result": 7}
```

However, it is possible that even though policies have been loaded the policy response is still `undefined` because the policy makes no decision for the given inputs.
```
$ opa run foo.rego -s
$ curl localhost:8181/v1/data/baz
{}
```

Just because OPA has returned an answer for a policy query, that does not indicate that it was operationally ready for that query.  Moreover, the operational readiness of OPA cannot be ascertained from the query response, as illustrated above.  Two issues must therefore be addressed: how to know when OPA is operationally ready for policy queries and how to make a decision before OPA is ready.

### Ensuring Operational Readiness

The relevance of the discussion above depends on how you have chosen to deploy policies into OPA.

If you deploy policies to OPA on disk (e.g. volume mounting into the OPA container on Kubernetes), then OPA will only start answering policy queries once all the policies are successfully loaded.  In this case, it is impossible for OPA to answer policy queries before it has loaded policy, so the discussion above is a non-issue.

On the other hand, if you use the [Bundle service](../management-bundles) OPA will start up without any policies and immediately start downloading a bundle.  But even before the bundle has successfully downloaded, OPA will answer policy queries if asked (which is in every case except the bootstrap case the right thing to do).  For this reason, OPA provides a `/health` [API](../rest-api/#health-api) that verifies that the server is operational and optionally that a bundle has been successfully activated.  As long as no policy queries are routed to OPA until the `/health` API verifies that OPA is operational.  The recommendation is to ensure the `/health` API indicates that OPA is operational before routing policy queries to it.

Finally, you might choose to push policies into OPA via its [REST API](../rest-api/#create-or-update-a-policy).  In this case, there is no way for OPA to know whether it has a complete policy set, and so the decision as to when to route policy queries to OPA must be handled by whatever software is pushing policies into OPA.

### Making Decisions before OPA is Ready

The mechanisms discussed above ensure that OPA is not asked to answer policy queries before it is ready to do so.  But from the perspective of the software needing decisions, until OPA is operational, the software must make a decision on its own.  Typically there are two choices:

* fail-open: if OPA does not provide a decision, then treat the decision as allowed.
* fail-closed: if OPA does not provide a decision, then treat the decision as denied.

The choices are more varied if the policy is not making an allow/deny decision, but often there is some analog to fail-open and fail-closed.  The key observation is that this logic is entirely the responsibility of the software asking OPA for a policy decision.  Despite the fact that what to do when OPA is unavailable is technically a policy question, it is one that we cannot rely on OPA to answer.  The right logic can depend on many factors including the likelihood of OPA not making a decision and the cost of allowing or denying a request incorrectly.

In Kubernetes admission control, for example, the Kubernetes admin can choose whether to fail-open or fail-closed, leaving the decision up to the user.  And often this is the correct way to build an integration because it is unlikely that there is a universal solution.  For example, running an OPA-integration in a development environment might require fail-open, but running exactly the same integration in a production environment might require fail-closed.

## Capabilities

OPA now supports a _capabilities_ check on policies. The check allows callers to restrict the [built-in](../policy-reference/#built-in-functions) functions that policies may depend on. If the policies passed to OPA require built-ins not listed in the capabilities structure, an error is returned. The capabilities check is currently supported by the `check` and `build` sub-commands and can be accessed programmatically on the `ast.Compiler` structure. The OPA repository includes a set of capabilities files for previous versions of OPA in the [capabilities](https://github.com/open-policy-agent/opa/tree/main/capabilities) folder.

For example, given the following policy:

```rego
package example

deny["missing semantic version"] {
  not valid_semantic_version_tag
}

valid_semantic_version_tag {
  semver.is_valid(input.version)
}
```

We can check whether it is compatible with different versions of OPA:

```bash
# OK!
$ opa build ./policies/example.rego --capabilities ./capabilities/v0.22.0.json

# ERROR!
$ opa build ./policies/example.rego --capabilities ./capabilities/v0.21.1.json
```

### Built-ins

The 'build' command can validate policies against a configurable set of OPA capabilities. The capabilities define the built-in functions and other language features that policies may depend on. For example, the following capabilities file only permits the policy to depend on the "plus" built-in function ('+'):

```json
{
    "builtins": [
        {
            "name": "plus",
            "infix": "+",
            "decl": {
                "type": "function",
                "args": [
                    {
                        "type": "number"
                    },
                    {
                        "type": "number"
                    }
                ],
                "result": {
                    "type": "number"
                }
            }
        }
    ]
}
```

The following command builds a directory of policies ('./policies') and validates them against `capability-built-in-plus.json`:

```bash
opa build ./policies --capabilities ./capability-built-in-plus.json
```

### Network

When passing a capabilities definition file via `--capabilities`, one can restrict which hosts remote schema definitions can be retrieved from. For example, a `capabilities.json` containing the json below would disallow fetching remote schemas from any host but "kubernetesjsonschema.dev". Setting `allow_net` to an empty array would prohibit fetching any remote schemas.

**capabilities.json**
```json
{
    "builtins": [ ... ],
    "allow_net": [ "kubernetesjsonschema.dev" ]
}
```

Not providing a capabilities file, or providing a file without an `allow_net` key, will permit fetching remote schemas from any host.

Note that the metaschemas [http://json-schema.org/draft-04/schema](http://json-schema.org/draft-04/schema), [http://json-schema.org/draft-06/schema](http://json-schema.org/draft-06/schema), and [http://json-schema.org/draft-07/schema](http://json-schema.org/draft-07/schema), are always available, even without network access.

Similarly, the `allow_net` capability restricts what hosts the `http.send` built-in function may send requests to, and what hosts the `net.lookup_ip_addr` built-in function may resolve IP addresses for.

### Future keywords

The availability of future keywords in an OPA version can also be controlled using the capabilities file:

```json
{
    "future_keywords": [ "in" ]
}
```

With these capabilities, the future import `future.keywords.in` would be available. See [the documentation
of the membership and iteration operator for details](../policy-language/#membership-and-iteration-in).

### Wasm ABI compatibility

A specific OPA version's capabilities file shows which Wasm ABI versions it is capable of evaluating:

```json
{
    "wasm_abi_versions": [
        {
            "version": 1,
            "minor_version": 1
        },
        {
            "version": 1,
            "minor_version": 2
        }
   ]
}
```

This snippet would allow for evaluating bundles containing Wasm modules of the ABI version 1.1 and 1.2.
See [the ABI version docs](../wasm/#abi-versions) for details.

### Building your own capabilities JSON

Use the following JSON structure to build more complex capability checks.

```json
{
    "builtins": [
        {
            "name": "name", // REQUIRED: Unique name of built-in function, e.g., <name>(arg1,arg2,...,argN)

            "infix": "+",  // OPTIONAL: Unique name of infix operator. Default should be unset.

            "decl": {  // REQUIRED: Built-in function type declaration.

                "type": "function", // REQUIRED: states this is a function

                "args": [ // REQUIRED: List of types to be passed in as an arguement: any, number, string, boolean, object, array, set.
                    {
                        "type": "number"
                    },
                    {
                        "type": "number"
                    }
                ],
                "result": { // REQUIRED: The expected result type.
                    "type": "number"
                }
            }
        }
    ],
    "allow_net": [ // OPTIONAL: allow_net is an array of hostnames or IP addresses, that an OPA instance is allowed to connect to.
      "mycompany.com",
      "database.safe",
    ],
    "future_keywords": [ "in" ]
}
```
