---
title: Integrating OPA
kind: documentation
weight: 65
restrictedtoc: true
---

OPA exposes domain-agnostic APIs that your service can call to manage and
enforce policies. Read this page if you want to integrate an application,
service, or tool with OPA.

## Evaluating Policies

OPA supports different APIs for evaluating policies.

* The [REST API](../rest-api) returns decisions as JSON over HTTP.
* The [Go API (GoDoc)](https://godoc.org/github.com/open-policy-agent/opa/rego) returns
  decisions as simple Go types (`bool`, `string`, `map[string]interface{}`,
  etc.)

### Integrating with the REST API

To integrate with OPA outside of Go, we recommend you deploy OPA as a host-level
daemon or sidecar container. When your application or service needs to make
policy decisions it can query OPA locally via HTTP. Running OPA locally on the
same host as your application or service helps ensure policy decisions are fast
and highly-available.

#### Named Policy Decisions

Use the [Data API](../rest-api#data-api) to query OPA for _named_ policy decisions:

```http
POST /v1/data/<path>
Content-Type: application/json
```

```json
{
    "input": <the input document>
}
```

The `<path>` in the HTTP request identifies the policy decision to ask for. In
OPA, every rule generates a policy decision. In the example below there are two
decisions: `example/authz/allow` and `example/authz/is_admin`.

```live:authz:module:openable,read_only
package example.authz

default allow = false

allow {
    some id
    input.method = "GET"
    input.path = ["salary", id]
    input.subject.user = id
}

allow {
    is_admin
}

is_admin {
    input.subject.groups[_] = "admin"
}
```

You can request specific decisions by querying for `<package path>/<rule name>`.
For example to request the `allow` decision execute the following HTTP request:

```http
POST /v1/data/example/authz/allow
Content-Type: application/json
```

```json
{
    "input": <the input document>
}
```

The body of the request specifies the value of the `input` document to use
during policy evaluation. For example:

```http
POST /v1/data/example/authz/allow
Content-Type: application/json
```
```json
{
    "input": {
        "method": "GET",
        "path": ["salary", "bob"],
        "subject": {
            "user": "bob"
        }
    }
}
```

OPA returns an HTTP 200 response code if the policy was evaluated successfully.
Non-HTTP 200 response codes indicate configuration or runtime errors. The policy
decision is contained in the `"result"` key of the response message body. For
example, the above request returns the following response:

```http
200 OK
Content-Type: application/json
```

```json
{
    "result": true
}
```

If the requested policy decision is _undefined_ OPA returns an HTTP 200 response
without the `"result"` key. For example, the following request for `is_admin` is
undefined because there is no default value for `is_admin` and the input does
not satisfy the `is_admin` rule body:

```http
POST /v1/data/example/authz/is_admin
Content-Type: application/json
```

```json
{
    "input": {
        "subject": {
            "user": "bob",
            "groups": ["sales", "marketing"]
        }
    }
}
```

The response:

```http
200 OK
Content-Type: application/json
```

```json
{}
```

For another example of how to integrate with OPA via HTTP see the [HTTP
API Authorization](../http-api-authorization) tutorial.

### Integrating with the Go API

Use the
[github.com/open-policy-agent/opa/rego](https://godoc.org/github.com/open-policy-agent/opa/rego)
package to embed OPA as a library inside services written in Go. To get started
import the `rego` package:

```go
import "github.com/open-policy-agent/opa/rego"
```

The `rego` package exposes different options for customizing how policies are
evaluated. Through the `rego` package you can supply policies and data, enable
metrics and tracing, toggle optimizations, etc. In most cases you will:

1. Use the `rego` package to construct a prepared query.
2. Execute the prepared query to produce policy decisions.
3. Interpret and enforce the policy decisions.

Preparing queries in advance avoids parsing and compiling the policies on each
query and improves performance considerably. Prepared queries are safe to share
across multiple Go routines.

To prepare a query create a new `rego.Rego` object by calling `rego.New(...)`
and then invoke `rego.Rego#PrepareForEval`. The `rego.New(...)` call can be
parameterized with different options like the query, policy module(s), data
store, etc.

```go

module := `
package example.authz

default allow = false

allow {
    some id
    input.method = "GET"
    input.path = ["salary", id]
    input.subject.user = id
}

allow {
    is_admin
}

is_admin {
    input.subject.groups[_] = "admin"
}
`

query, err := rego.New(
    rego.Query("x = data.example.authz.allow"),
    rego.Module("example.rego", module),
    ).PrepareForEval(ctx)

if err != nil {
    // Handle error.
}
```

Using the `query` returned by `rego.Rego#PrepareForEval` call the `Eval`
function to evaluate the policy:

```go
input := map[string]interface{}{
    "method": "GET",
    "path": []interface{}{"salary", "bob"},
    "subject": map[string]interface{}{
        "user": "bob",
        "groups": []interface{}{"sales", "marketing"},
    },
}

ctx := context.TODO()
results, err := query.Eval(ctx, rego.EvalInput(input))
```

The `rego.PreparedEvalQuery#Eval` function returns a _result set_ that contains
the query results. If the result set is empty it indicates the query could not
be satisfied. Each element in the result set contains a set of _variable
bindings_ and a set of expression values. The query from above includes a single
variable `x` so we can lookup the value and interpret it to enforce the policy
decision.

```go
if err != nil {
    // Handle evaluation error.
} else if len(results) == 0 {
    // Handle undefined result.
} else if result, ok := results[0].Bindings["x"].(bool); !ok {
    // Handle unexpected result type.
} else {
    // Handle result/decision.
    // fmt.Printf("%+v", results) => [{Expressions:[true] Bindings:map[x:true]}]
}
```

For more examples of embedding OPA as a library see the
[`rego`](https://godoc.org/github.com/open-policy-agent/opa/rego#pkg-examples)
package in the Go documentation.

### WebAssembly (Wasm)

Policies can be evaluated as compiled Wasm binaries.

See [OPA Wasm docs](../wasm) for more details.

## Managing OPA

OPA supports a set of management APIs for distributing policies and collecting
telemetry information on OPA deployments.

- See the [Bundle API](../management/#bundles) for distributing policy and data to OPA.
- See the [Status API](../management/#status) for collecting status reports on bundle activation and agent health.
- See the [Decision Log API](../management/#decision-logs) for collecting a log of policy decisions made by agents.
- See the [Health API](../rest-api#health-api) for checking agent deployment readiness and health.

OPA also exports a [Prometheus API endpoint](../management/#prometheus) that can be scraped to obtain
insight into performance and errors.
