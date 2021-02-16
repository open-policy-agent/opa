---
title: Performance
kind: envoy
weight: 100
---

This page provides some performance benchmarks that give an idea of the overhead of using the OPA-Envoy plugin.

### Test Setup

The setup uses the same example Go application that's described in the [standalone Envoy tutorial](../envoy-tutorial-standalone-envoy#steps). Below
are some more details about the setup:

* Platform: Minikube
* Kubernetes Version: 1.18.6
* Envoy Version: 1.17.0
* OPA-Envoy Version: 0.26.0-envoy

### Benchmarks

The benchmark result below provides the percentile distribution of the latency observed by sending *100 requests/sec*
to the sample application. Each request makes a `GET` call to the `/people` endpoint exposed by the application.

The graph shows the latency distribution when the load test is performed under the following conditions:

* **App Only**

In this case, the graph documents the latency distribution observed when requests are
sent directly to the application ie. no Envoy and OPA in the request path. This scenario is depicted by the
`blue` curve.

* **App and Envoy**

In this case, the distribution is with [Envoy External Authorization API](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html) disabled. This means
OPA is not included in the request path but Envoy is. This scenario is depicted by the `red` curve.

* **App, Envoy and OPA (NOP policy)**

In the case, we will see the latency observed with [Envoy External Authorization API](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html) enabled. This means
Envoy will make a call to OPA on every incoming request. The graph explores the effect of loading the below NOP policy into
OPA. This scenario is depicted by the `green` curve.

```live:nop_example:module:read_only
package envoy.authz

default allow = true
```

* **App, Envoy and OPA (RBAC policy)**

In the case, we will see the latency observed with [Envoy External Authorization API](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html) enabled and
explore the effect of loading the following RBAC policy into OPA. This scenario is depicted by the `yellow` curve.

```live:rbac_example:module:read_only
package envoy.authz

import input.attributes.request.http as http_request

default allow = false

allow {
    roles_for_user[r]
    required_roles[r]
}

roles_for_user[r] {
    r := user_roles[user_name][_]
}

required_roles[r] {
    perm := role_perms[r][_]
    perm.method = http_request.method
    perm.path = http_request.path
}

user_name = parsed {
    [_, encoded] := split(http_request.headers.authorization, " ")
    [parsed, _] := split(base64url.decode(encoded), ":")
}

user_roles = {
    "alice": ["guest"],
    "bob": ["admin"]
}

role_perms = {
    "guest": [
        {"method": "GET",  "path": "/people"},
    ],
    "admin": [
        {"method": "GET",  "path": "/people"},
        {"method": "POST",  "path": "/people"},
    ],
}
```

{{< figure src="hist-google-grpc-100.png" width="250" >}}

The above four scenarios are replicated to measure the latency distribution now by sending *1000 requests/sec*
to the sample application. The following graph captures this result.

{{< figure src="hist-google-grpc-1000.png" width="250" >}}

#### OPA Benchmarks

The table below captures the `gRPC Server Handler` and `OPA Evaluation` time with [Envoy External Authorization API](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html) enabled and the
`RBAC` policy described above loaded into OPA. All values are in microseconds.

##### OPA Evaluation

`OPA Evaluation` is the time taken to evaluate the policy.

| Number of Requests per sec | 75% | 90% | 95% | 99% | 99.9% | 99.99% | Mean | Median |
|--------------------------|---|---|---|---|-----|------|----|------|
| `100` | `419.568` | `686.746` | `962.673` | `4048.899` | `14549.446` | `14680.476` | `467.001` | `311.939` |
| `1000` | `272.289` | `441.121` | `765.384` | `2766.152` | `63938.739` | `65609.013` | `380.009` | `207.277` |
| `2000` | `278.970` | `720.716` | `1830.884` | `4104.182` | `35013.074` | `35686.142` | `450.875` | `178.829` |
| `3000` | `266.105` | `693.839` | `1824.983` | `5069.019` | `368469.802` | `375877.246` | `971.173` | `175.948` |
| `4000` | `373.699` | `1087.224` | `2279.981` | `4735.961` | `95769.559` | `96310.587` | `665.828` | `218.180` |
| `5000` | `303.871` | `1188.718` | `2321.216` | `6116.459` | `317098.375` | `325740.476` | `865.961` | `188.054` |

##### gRPC Server Handler

`gRPC Server Handler` is the total time taken to prepare the input for the policy, evaluate the policy (`OPA Evaluation`)
and prepare the result.

| Number of Requests per sec | 75% | 90% | 95% | 99% | 99.9% | 99.99% | Mean | Median |
|--------------------------|---|---|---|---|-----|------|----|------|
| `100` | `825.112` | `1170.699` | `1882.797` | `6559.087` | `15583.934` | `15651.395` | `862.647` | `613.916` |
| `1000` | `536.859` | `957.586` | `1928.785` | `4606.781` | `139058.276` | `141515.222` | `884.912` | `397.676` |
| `2000` | `564.386` | `1784.671` | `2794.505` | `43412.251` | `271882.085` | `272075.761` | `2008.655` | `351.330` |
| `3000` | `538.376` | `2292.657` | `3014.675` | `32718.355` | `364730.469` | `370538.309` | `1799.534` | `322.755` |
| `4000` | `708.905` | `2397.769` | `4134.862` | `316881.804` | `636688.855` | `637773.152` | `7054.173` | `400.242` |
| `5000` | `620.252` | `2197.613` | `3548.392` | `176699.779` | `556518.400` | `558795.978` | `4581.492` | `339.063` |

##### Resource Utilization

The following table records the CPU and memory usage for the OPA-Envoy container. These metrics were obtained using the
`kubectl top` command. No resource limits were specified for the OPA-Envoy container.

| Number of Requests per sec | CPU(cores) | Memory(bytes) |
|--------------------------|---|---|
| `100` | `253m` | `21Mi` |
| `1000` | `563m` | `52Mi` |
| `2000` | `906m` | `121Mi` |
| `3000` | `779m` | `117Mi` |
| `4000` | `920m` | `159Mi` |
| `5000` | `828m` | `116Mi` |

In the analysis so far, the gRPC client used in Envoy's External authorization filter configuration is the [Google C++ gRPC client](https://github.com/grpc/grpc).
The following graph displays the latency distribution for the same four conditions described previously (ie. *App Only*,
*App and Envoy*, *App, Envoy and OPA (NOP policy)* and *App, Envoy and OPA (RBAC policy)*) by sending *100 requests/sec*
to the sample application but now using Envoy’s in-built gRPC client.

{{< figure src="hist-envoy-grpc-100.png" width="250" >}}

The below graph captures the latency distribution when *1000 requests/sec* are sent to the sample application and
Envoy’s in-built gRPC client is used.

{{< figure src="hist-envoy-grpc-1000.png" width="250" >}}

The above graphs show that there is extra latency added when the OPA-Envoy plugin is used as an external authorization service.
For example, in the previous graph, the latency for the *App, Envoy and OPA (NOP policy)* condition between the 90th and 99th percentile
is at least double than that for *App and Envoy*.

The following graphs show the latency distribution for the *App, Envoy and OPA (NOP policy)* and *App, Envoy and OPA (RBAC policy)*
condition and plot the latencies seen by using the Google C++ gRPC client and Envoy’s in-built gRPC client in the
External authorization filter configuration. The first graph is when *100 requests/sec* are sent to the application
while the second one for *1000 requests/sec*.

{{< figure src="hist-google-vs-envoy-grpc-100.png" width="250" >}}

{{< figure src="hist-google-vs-envoy-grpc-1000.png" width="250" >}}
