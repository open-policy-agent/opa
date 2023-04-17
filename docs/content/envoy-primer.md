---
title: Policy Primer via Examples
kind: envoy
weight: 2
---

This page covers how to write policies for the content of the requests that are passed to OPA by Envoy's
[External Authorization
filter](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/security/ext_authz_filter.html).

## Writing Policies

Let's start with an example policy that restricts access to an endpoint based on a user's role and permissions.

```live:bool_example:module:openable
package envoy.authz
import future.keywords

import input.attributes.request.http

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
    http.method == "GET"
    token.payload.role == "guest"
    glob.match("/people/*", ["/"], http.path)
}

action_allowed if {
    http.method == "GET"
    token.payload.role == "admin"
    glob.match("/people/*", ["/"], http.path)
}

action_allowed if {
    http.method == "POST"
    token.payload.role == "admin"
    glob.match("/people", ["/"], http.path)
    lower(input.parsed_body.firstname) != base64url.decode(token.payload.sub)
}


token := {"valid": valid, "payload": payload} if {
    [_, encoded] := split(http.headers.authorization, " ")
    [valid, _, payload] := io.jwt.decode_verify(encoded, {"secret": "secret"})
}
```

The first line `package envoy.authz` declaration gives the (hierarchical) name `envoy.authz` to the rules in the
remainder of the policy. If the OPA-Envoy [configuration](../envoy-introduction#configuration) does not specify the `path`
field, `envoy/authz/allow` will be considered as the default policy decision path. `data.envoy.authz.allow` will be the
name of the policy decision to query in the default case.

The above policy uses the `io.jwt.decode_verify` builtin function to parse and verify the JWT containing
information about the user making the request. It uses other builtins like `glob.match`, `lower`, `base64url.decode` etc.
OPA has 150+ builtins detailed at [openpolicyagent.org/docs/policy-reference](../policy-reference).

The dot notation seen in multiple places in the policy for ex. `input.parsed_body.firstname` simply descends through
the hierarchy to access the requested value. The dot (.) operator never throws any errors; if the path does not exist
the value of the expression is `undefined`.

```live:bool_example:query:hidden
data.envoy.authz.allow
```

Sample input received by OPA is shown below:

```live:bool_example:input
{
  "attributes": {
    "request": {
      "http": {
        "method": "GET",
        "path": "/people/",
        "headers": {
          "authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiZ3Vlc3QiLCJzdWIiOiJZV3hwWTJVPSIsIm5iZiI6MTUxNDg1MTEzOSwiZXhwIjoxNjQxMDgxNTM5fQ.K5DnnbbIOspRbpCr2IKXE9cPVatGOCBrBQobQmBmaeU"
        }
      }
    }
  }
}
```

With the input value above, the answer is:

```live:bool_example:output
```

## Example Policy with Additional Controls

The `allow` variable in the above policy returns a `boolean` decision to indicate whether a request should be allowed or not.
If you want, you can also control the HTTP status sent to the upstream or downstream client, along with the response body, and the response headers.  To do that, you can write rules like the ones below to fill in values for variables with the following types:

* `headers` is an object whose keys are strings and values are strings. In case the request is denied, the object represents the HTTP response headers to be sent to the downstream client. If the request is allowed, the object represents additional request headers to be sent to the upstream.
* `response_headers_to_add` is an object whose keys are strings and values are strings. It defines the HTTP response headers to be sent to the downstream client when a request is allowed.
* `request_headers_to_remove` is an array of strings which describes the HTTP headers to remove from the original request before dispatching it to the upstream when a request is allowed.
* `body` is a string which represents the response body data sent to the downstream client when a request is denied.
* `status_code` is a number which represents the HTTP response status code sent to the downstream client when a request is denied.

```live:obj_example:module:openable
package envoy.authz
import future.keywords

import input.attributes.request.http

default allow := false

allow if {
    is_token_valid
    action_allowed
}

headers["x-ext-auth-allow"] := "yes"
headers["x-validated-by"] := "security-checkpoint"

request_headers_to_remove := ["one-auth-header", "another-auth-header"]

response_headers_to_add["x-foo"] := "bar"

status_code := 200 if {
  allow
} else := 401 {
  not is_token_valid
} else := 403

body := "Authentication Failed" if status_code == 401
body := "Unauthorized Request"  if status_code == 403

is_token_valid if {
    token.valid
    now := time.now_ns() / 1000000000
    token.payload.nbf <= now
    now < token.payload.exp
}

action_allowed if {
    http.method == "GET"
    token.payload.role == "guest"
    glob.match("/people/*", ["/"], http.path)
}

action_allowed if {
    http.method == "GET"
    token.payload.role == "admin"
    glob.match("/people/*", ["/"], http.path)
}

action_allowed if {
    http.method == "POST"
    token.payload.role == "admin"
    glob.match("/people", ["/"], http.path)
    lower(input.parsed_body.firstname) != base64url.decode(token.payload.sub)
}


token := {"valid": valid, "payload": payload} if {
    [_, encoded] := split(http.headers.authorization, " ")
    [valid, _, payload] := io.jwt.decode_verify(encoded, {"secret": "secret"})
}
```

```live:obj_example:query:hidden
data.envoy.authz
```

Sample input received by OPA is shown below:

```live:obj_example:input
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

With the input value above, the value of all the variables in the package are:

```live:obj_example:output
```

## Output Document

When Envoy receives a policy decision, it expects a JSON object with the following fields:
* `allowed` (required): a boolean deciding whether or not the request is allowed
* `headers` (optional): an object mapping a string header name to a string header value (e.g. key "x-ext-auth-allow" has value "yes")
* `response_headers_to_add` (optional): an object mapping a string header name to a string header value
* `request_headers_to_remove` (optional): is an array of string header names
* `http_status` (optional): a number representing the HTTP status code
* `body` (optional): the response body

To construct that output object using the policies demonstrated in the last section, you can use the following Rego snippet.  Notice that we are using partial object rules so that any variables with undefined values simply have no key in the `result` object.

```rego
result["allowed"] := allow
result["headers"] := headers
result["response_headers_to_add"] := response_headers_to_add
result["request_headers_to_remove"] := request_headers_to_remove
result["body"] := body
result["http_status"] := status_code
```

For a single user, including this snippet in your normal policy is fine, but when you have multiple teams writing policies, you will typically pull this bit of boilerplate into a wrapper package, so your teams can focus on writing the policies shown in the previous sections.


## Input Document

In OPA, `input` is a reserved, global variable whose value is the request sent by the Envoy External Authorization filter
to OPA. The OPA-Envoy plugin supports both [v2](https://www.envoyproxy.io/docs/envoy/latest/api-v2/service/auth/v2/external_auth.proto#service-auth-v2-checkrequest)
and [v3](https://www.envoyproxy.io/docs/envoy/latest/api-v3/service/auth/v3/external_auth.proto#service-auth-v3-checkrequest)
versions of the `CheckRequest` which is used to pass the request to OPA.

For v3 requests, the [specified JSON mapping for protobuf](https://developers.google.com/protocol-buffers/docs/proto3#json)
is used for making the incoming `envoy.service.auth.v3.CheckRequest` available in `input`.
It differs from the encoding used for v2 requests. In v3, all keys are lower camelcase.
Also, needless nesting of `oneof` values is removed.

For example, source address data that looks like this in v2,

```
"source": {
  "address": {
    "Address": {
      "SocketAddress": {
        "PortSpecifier": {
          "PortValue": 59052
        },
        "address": "127.0.0.1"
      }
    }
  }
}
```

becomes, in v3,

```
"source": {
  "address": {
    "socketAddress": {
      "address": "127.0.0.1",
      "portValue": 59052
    }
  }
}
```

The following table shows the rego code for common data, in v2 and v3:


| information         |  rego v2 | rego v3 |
|---------------------|----------|---------|
| `source address`      | `input.attributes.source.address.Address.SocketAddress.address` | `input.attributes.source.address.socketAddress.address`|
| `source port`         | `input.attributes.source.address.Address.SocketAddress.PortSpecifier.PortValue` | `input.attributes.source.address.socketAddress.portValue`|
| `destination address` | `input.attributes.destination.address.Address.SocketAddress.address` | `input.attributes.destination.address.socketAddress.address`|
| `destination port`    | `input.attributes.destination.address.Address.SocketAddress.PortSpecifier.PortValue` | `input.attributes.destination.address.socketAddress.portValue`|
| `dynamic metadata`   | `input.attributes.metadata_context.filter_metadata` | `input.attributes.metadataContext.filterMetadata` |

Due to those differences, it's important to know which version is used when writing policies.
Thus, this information is passed into the OPA evaluation under `input.version`, where you'll either
find, for v2,

```live:v2_sample:module:read_only
input.version == { "ext_authz": "v2", "encoding": "encoding/json" }
```

or, for v3,

```live:v3_sample:module:read_only
input.version == { "ext_authz": "v3", "encoding": "protojson" }
```

To have Envoy use the v3 version of the service, the `http_filters` entry in the Envoy configuration should look
like below (minimal version):

```yaml
http_filters:
- name: envoy.ext_authz
  typed_config:
    '@type': type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
    transport_api_version: V3
    grpc_service:
      google_grpc: # or envoy_grpc
        target_uri: "127.0.0.1:9191"
```

### Example Input

{{<detail-tag "Example v3 Input">}}
```json
{
  "attributes": {
    "source": {
      "address": {
        "socketAddress": {
          "address": "172.17.0.1",
          "portValue": 61402
        }
      }
    },
    "destination": {
      "address": {
        "socketAddress": {
          "address": "172.17.06",
          "portValue": 8000
        }
      }
    },
    "request": {
      "time": "2020-11-20T09:47:47.722473Z",
      "http": {
        "id":"13519049518330544501",
        "method": "POST",
        "headers": {
          ":authority":"192.168.99.206:30164",
          ":method":"POST",
          ":path":"/people?lang=en",
          "accept": "*/*",
          "authorization":"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYWRtaW4iLCJzdWIiOiJZbTlpIiwibmJmIjoxNTE0ODUxMTM5LCJleHAiOjE2NDEwODE1Mzl9.WCxNAveAVAdRCmkpIObOTaSd0AJRECY2Ch2Qdic3kU8",
          "content-length":"41",
          "content-type":"application/json",
          "user-agent":"curl/7.54.0",
          "x-forwarded-proto":"http",
          "x-request-id":"7bca5c86-bf55-432c-b212-8c0f1dc999ec"
        },
        "host":"192.168.99.206:30164",
        "path":"/people?lang=en",
        "protocol":"HTTP/1.1",
        "body":"{\"firstname\":\"Charlie\", \"lastname\":\"Opa\"}",
        "size":41
      }
    },
    "metadataContext": {}
  },
  "parsed_body":{"firstname": "Charlie", "lastname": "Opa"},
  "parsed_path":["people"],
  "parsed_query": {"lang": ["en"]},
  "truncated_body": false,
  "version": {
    "encoding":"protojson",
    "ext_authz":"v3"
  }
}
```
{{</detail-tag>}}

{{<detail-tag "Example v2 Input">}}
```json
{
  "attributes":{
     "source":{
        "address":{
           "Address":{
              "SocketAddress":{
                 "PortSpecifier":{
                    "PortValue":61402
                 },
                 "address":"172.17.0.1"
              }
           }
        }
     },
     "destination":{
        "address":{
           "Address":{
              "SocketAddress":{
                 "PortSpecifier":{
                    "PortValue":8000
                 },
                 "address":"172.17.0.6"
              }
           }
        }
     },
     "request":{
        "http":{
           "id":"13519049518330544501",
           "method":"POST",
           "headers":{
              ":authority":"192.168.99.206:30164",
              ":method":"POST",
              ":path":"/people?lang=en",
              "accept":"*/*",
              "authorization":"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYWRtaW4iLCJzdWIiOiJZbTlpIiwibmJmIjoxNTE0ODUxMTM5LCJleHAiOjE2NDEwODE1Mzl9.WCxNAveAVAdRCmkpIObOTaSd0AJRECY2Ch2Qdic3kU8",
              "content-length":"41",
              "content-type":"application/json",
              "user-agent":"curl/7.54.0",
              "x-forwarded-proto":"http",
              "x-request-id":"7bca5c86-bf55-432c-b212-8c0f1dc999ec"
           },
           "host":"192.168.99.206:30164",
           "path":"/people?lang=en",
           "protocol":"HTTP/1.1",
           "body":"{\"firstname\":\"Charlie\", \"lastname\":\"Opa\"}",
           "size":41
        }
     }
  },
  "parsed_body":{"firstname": "Charlie", "lastname": "Opa"},
  "parsed_path":["people"],
  "parsed_query": {"lang": ["en"]},
  "truncated_body": false,
  "version": {
    "encoding":"encoding/json",
    "ext_authz":"v2"
  }
}
```
{{</detail-tag>}}

The `parsed_path` field in the input is generated from the `path` field in the HTTP request which is included in the
Envoy External Authorization `CheckRequest` message type. This field provides the request path as a string array which
can help policy authors perform pattern matching on the HTTP request path. The below sample policy allows anyone to
access the path `/people`.

```live:parsed_path_example:module:read_only
package envoy.authz
import future.keywords

default allow := false

allow if input.parsed_path == ["people"]
```

The `parsed_query` field in the input is also generated from the `path` field in the HTTP request. This field provides
the HTTP URL query as a map of string array. The below sample policy allows anyone to access the path
`/people?lang=en&id=1&id=2`.

```live:parsed_query_example:module:read_only
package envoy.authz
import future.keywords

default allow := false

allow if {
    input.parsed_path == ["people"]
    input.parsed_query.lang == ["en"]
    input.parsed_query.id == ["1", "2"]
}
```

The `parsed_body` field in the input is generated from the `body` field in the HTTP request which is included in the
Envoy External Authorization `CheckRequest` message type. This field contains the deserialized JSON request body which
can then be used in a policy as shown below.

```live:parsed_body_example:module:read_only
package envoy.authz
import future.keywords

default allow := false

allow if {
    input.parsed_body.firstname == "Charlie"
    input.parsed_body.lastname == "Opa"
}
```

The `truncated_body` field in the input represents if the HTTP request body is truncated. The body is considered to be
truncated, if the value of the `Content-Length` header exceeds the size of the request body.

If `skip-request-body-parse: true` is specified in the OPA-Envoy [configuration](../envoy-introduction#configuration), then
the `parsed_body` and `truncated_body` fields will be omitted from the input.

## Example with JWT payload passed from Envoy

Envoy can be configured to pass validated JWT payload data into the `ext_authz` filter with `metadata_context_namespaces`
and `payload_in_metadata`.

### Example Envoy Configuration

```yaml
http_filters:
- name: envoy.filters.http.jwt_authn
  typed_config:
  "@type": type.googleapis.com/envoy.config.filter.http.jwt_authn.v2alpha.JwtAuthentication
  providers:
    example:
      payload_in_metadata: verified_jwt
      <...>
- name: envoy.ext_authz
  config:
    metadata_context_namespaces:
    - envoy.filters.http.jwt_authn
    <...>
```

### Example OPA Input

This will result in something like the following dictionary being added to `input.attributes` (some common fields have
been excluded for brevity):

```
  "metadata_context": {
    "filter_metadata": {
      "envoy.filters.http.jwt_authn": {
        "verified_jwt": {
          "email": "alice@example.com",
          "exp": 1569026124,
          "name": "Alice"
        }
      }
    }
  }
```
