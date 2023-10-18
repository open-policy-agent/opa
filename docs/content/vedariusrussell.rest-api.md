---
title: REST API
kind: documentation
weight: 80
restrictedtoc: true
---

This document is the authoritative specification of the OPA REST API. The API can be broken down into the following
groups:

* [Policy API](#policy-api) - manage policy loaded into the OPA instance.
* [Data API](#data-api) - evaluate rules and retrieve data.
* [Query API](#query-api) - execute adhoc queries.
* [Compile API](#compile-api) - access Rego's [Partial Evaluation](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422) functionality.
* [Health API](#health-api) - access instance operational health information.
* [Config API](#config-api) - view instance configuration.
* [Status API](#status-api) - view instance [status](../management-status) state.

The REST API is a very common way to integrate with OPA.
{{<
  ecosystem_feature_link
  key="rest-api-integration"
  singular_intro="There is currently 1 project"
  singular_link="listed in the OPA Ecosystem"
  singular_outro="which is built on the REST API."
  plural_intro="There are"
  plural_link="COUNT OPA Ecosystem projects"
  plural_outro="- many of which are open source - built on the REST API which might serve as inspiration."
>}}
You may also want to review the [integration documentation](../integration) for other options
to build on OPA by embedding functionality directly into your application.

##  Policy API

The Policy API exposes CRUD endpoints for managing policy modules. Policy modules can be added, removed, and modified at any time.

The identifiers given to policy modules are only used for management purposes. They are not used outside of the Policy API.

### List Policies

```http
GET /v1/policies HTTP/1.1
```

List policy modules.


#### Status Codes

- **200** - no error
- **500** - server error

#### Example Request

```http
GET /v1/policies HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "id": "example2",
      "raw": "package opa.examples\n\nimport data.servers\n\nviolations[server] {\n\tserver = servers[_]\n\tserver.protocols[_] = \"http\"\n\tpublic_servers[server]\n}\n",
      "ast": {
        "package": {
          "path": [
            {
              "type": "var",
              "value": "data"
            },
            {
              "type": "string",
              "value": "opa"
            },
            {
              "type": "string",
              "value": "examples"
            }
          ]
        },
        "rules": [
          {
            "head": {
              "name": "violations",
              "key": {
                "type": "var",
                "value": "server"
              }
            },
            "body": [
              {
                "index": 0,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "var",
                    "value": "server"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "servers"
                      },
                      {
                        "type": "var",
                        "value": "$0"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 1,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "server"
                      },
                      {
                        "type": "string",
                        "value": "protocols"
                      },
                      {
                        "type": "var",
                        "value": "$1"
                      }
                    ]
                  },
                  {
                    "type": "string",
                    "value": "http"
                  }
                ]
              },
              {
                "index": 2,
                "terms": {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "opa"
                    },
                    {
                      "type": "string",
                      "value": "examples"
                    },
                    {
                      "type": "string",
                      "value": "public_servers"
                    },
                    {
                      "type": "var",
                      "value": "server"
                    }
                  ]
                }
              }
            ]
          }
        ]
      }
    },
    {
      "id": "example1",
      "raw": "package opa.examples\n\nimport data.servers\nimport data.networks\nimport data.ports\n\npublic_servers[server] {\n\tserver = servers[_]\n\tserver.ports[_] = ports[k].id\n\tports[k].networks[_] = networks[m].id\n\tnetworks[m].public = true\n}\n",
      "ast": {
        "package": {
          "path": [
            {
              "type": "var",
              "value": "data"
            },
            {
              "type": "string",
              "value": "opa"
            },
            {
              "type": "string",
              "value": "examples"
            }
          ]
        },
        "rules": [
          {
            "head": {
              "name": "public_servers",
              "key": {
                "type": "var",
                "value": "server"
              }
            },
            "body": [
              {
                "index": 0,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "var",
                    "value": "server"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "servers"
                      },
                      {
                        "type": "var",
                        "value": "$0"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 1,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "server"
                      },
                      {
                        "type": "string",
                        "value": "ports"
                      },
                      {
                        "type": "var",
                        "value": "$1"
                      }
                    ]
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "ports"
                      },
                      {
                        "type": "var",
                        "value": "k"
                      },
                      {
                        "type": "string",
                        "value": "id"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 2,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "ports"
                      },
                      {
                        "type": "var",
                        "value": "k"
                      },
                      {
                        "type": "string",
                        "value": "networks"
                      },
                      {
                        "type": "var",
                        "value": "$2"
                      }
                    ]
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "networks"
                      },
                      {
                        "type": "var",
                        "value": "m"
                      },
                      {
                        "type": "string",
                        "value": "id"
                      }
                    ]
                  }
                ]
              },
              {
                "index": 3,
                "terms": [
                  {
                    "type": "string",
                    "value": "eq"
                  },
                  {
                    "type": "ref",
                    "value": [
                      {
                        "type": "var",
                        "value": "data"
                      },
                      {
                        "type": "string",
                        "value": "networks"
                      },
                      {
                        "type": "var",
                        "value": "m"
                      },
                      {
                        "type": "string",
                        "value": "public"
                      }
                    ]
                  },
                  {
                    "type": "boolean",
                    "value": true
                  }
                ]
              }
            ]
          }
        ]
      }
    }
  ]
}

```

### Get a Policy

```
GET /v1/policies/<id>
```

Get a policy module.

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **404** - not found
- **500** - server error

#### Example Request

```http
GET /v1/policies/example1 HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": {
    "id": "example1",
    "raw": "package opa.examples\n\nimport data.servers\nimport data.networks\nimport data.ports\n\npublic_servers[server] {\n\tserver = servers[_]\n\tserver.ports[_] = ports[k].id\n\tports[k].networks[_] = networks[m].id\n\tnetworks[m].public = true\n}\n",
    "ast": {
      "package": {
        "path": [
          {
            "type": "var",
            "value": "data"
          },
          {
            "type": "string",
            "value": "opa"
          },
          {
            "type": "string",
            "value": "examples"
          }
        ]
      },
      "rules": [
        {
          "head": {
            "name": "public_servers",
            "key": {
              "type": "var",
              "value": "server"
            }
          },
          "body": [
            {
              "index": 0,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "var",
                  "value": "server"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "servers"
                    },
                    {
                      "type": "var",
                      "value": "$0"
                    }
                  ]
                }
              ]
            },
            {
              "index": 1,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "server"
                    },
                    {
                      "type": "string",
                      "value": "ports"
                    },
                    {
                      "type": "var",
                      "value": "$1"
                    }
                  ]
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "ports"
                    },
                    {
                      "type": "var",
                      "value": "k"
                    },
                    {
                      "type": "string",
                      "value": "id"
                    }
                  ]
                }
              ]
            },
            {
              "index": 2,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "ports"
                    },
                    {
                      "type": "var",
                      "value": "k"
                    },
                    {
                      "type": "string",
                      "value": "networks"
                    },
                    {
                      "type": "var",
                      "value": "$2"
                    }
                  ]
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "networks"
                    },
                    {
                      "type": "var",
                      "value": "m"
                    },
                    {
                      "type": "string",
                      "value": "id"
                    }
                  ]
                }
              ]
            },
            {
              "index": 3,
              "terms": [
                {
                  "type": "string",
                  "value": "eq"
                },
                {
                  "type": "ref",
                  "value": [
                    {
                      "type": "var",
                      "value": "data"
                    },
                    {
                      "type": "string",
                      "value": "networks"
                    },
                    {
                      "type": "var",
                      "value": "m"
                    },
                    {
                      "type": "string",
                      "value": "public"
                    }
                  ]
                },
                {
                  "type": "boolean",
                  "value": true
                }
              ]
            }
          ]
        }
      ]
    }
  }
}
```

### Create or Update a Policy

```
PUT /v1/policies/<id>
Content-Type: text/plain
```

Create or update a policy module.

If the policy module does not exist, it is created. If the policy module already exists, it is replaced.

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **metrics** - Return compiler performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

Before accepting the request, the server will parse, compile, and install the policy module. If the policy module is invalid, one of these steps will fail and the server will respond with 400. The error message in the response will be set to indicate the source of the error.

#### Example Request

```http
PUT /v1/policies/example1 HTTP/1.1
Content-Type: text/plain
```

```live:put_example:module:read_only
package opa.examples

import data.servers
import data.networks
import data.ports

public_servers[server] {
  some k, m
	server := servers[_]
	server.ports[_] == ports[k].id
	ports[k].networks[_] == networks[m].id
	networks[m].public == true
}
```

> cURL's `-d/--data` flag removes newline characters from input files. Use the `--data-binary` flag instead.

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{}
```

### Delete a Policy

```
DELETE /v1/policies/<id>
```

Delete a policy module.

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **metrics** - Return compiler performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - not found
- **500** - server error

If other policy modules in the same package depend on rules in the policy module to be deleted, the server will return 400.

#### Example Request

```http
DELETE /v1/policies/example2 HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{}
```

## Data API

The Data API exposes endpoints for reading and writing documents in OPA. For an explanation to the different types of documents in OPA see [How Does OPA Work?](../philosophy#how-does-opa-work)

### Get a Document

```
GET /v1/data/{path:.+}
```

Get a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

#### Query Parameters

- **input** - Provide an input document. Format is a JSON value that will be used as the value for the input document.
- **pretty** - If parameter is `true`, response will be formatted for humans.
- **provenance** - If parameter is `true`, response will include build/version info in addition to the result.  See [Provenance](#provenance) for more detail.
- **explain** - Return query explanation in addition to result. Values: **notes**, **fails**, **full**, **debug**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **instrument** - Instrument query evaluation and return a superset of performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **strict-builtin-errors** - Treat built-in function call errors as fatal and return an error immediately.

#### Request Headers

- **Accept-Encoding: gzip**: Indicates the server should respond with a gzip encoded body. The server will send the compressed response only if its length is above `server.encoding.gzip.min_length` value. See the configuration section

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if the input document is invalid (i.e. malformed JSON).

The server returns 200 if the path refers to an undefined document. In this
case, the response will not contain a `result` property.

#### Response Message

- **result** - The base or virtual document referred to by the URL path. If the
  path is undefined, this key will be omitted.
- **metrics** - If query metrics are enabled, this field contains query
  performance metrics collected during the parse, compile, and evaluation steps.
* **decision_id** - If decision logging is enabled, this field contains a string
  that uniquely identifies the decision. The identifier will be included in the
  decision log event for this decision. Callers can use the identifier for
  correlation purposes.

#### Example Request

```http
GET /v1/data/opa/examples/public_servers HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "id": "s1",
      "name": "app",
      "ports": [
        "p1",
        "p2",
        "p3"
      ],
      "protocols": [
        "https",
        "ssh"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "ports": [
        "p1",
        "p2"
      ],
      "protocols": [
        "http"
      ]
    }
  ]
}
```

### Get a Document (with Input)

```
POST /v1/data/{path:.+}
Content-Type: application/json
```

```json
{
  "input": ...
}
```

Get a document that requires input.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

The request body contains an object that specifies a value for [The input Document](../philosophy/#the-opa-document-model).

#### Request Headers

- **Content-Type: application/x-yaml**: Indicates the request body is a YAML encoded object.
- **Content-Encoding: gzip**: Indicates the request body is a gzip encoded object.
- **Accept-Encoding: gzip**: Indicates the server should respond with a gzip encoded body. The server will send the compressed response only if its length is above `server.encoding.gzip.min_length` value. See the configuration section

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **provenance** - If parameter is `true`, response will include build/version info in addition to the result.  See [Provenance](#provenance) for more detail.
- **explain** - Return query explanation in addition to result. Values: **notes**, **fails**, **full**, **debug**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **instrument** - Instrument query evaluation and return a superset of performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **strict-builtin-errors** - Treat built-in function call errors as fatal and return an error immediately.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if the input document is invalid (i.e. malformed JSON).

The server returns 200 if the path refers to an undefined document. In this
case, the response will not contain a `result` property.

#### Response Message

- **result** - The base or virtual document referred to by the URL path. If the
  path is undefined, this key will be omitted.
- **metrics** - If query metrics are enabled, this field contains query
  performance metrics collected during the parse, compile, and evaluation steps.
* **decision_id** - If decision logging is enabled, this field contains a string
  that uniquely identifies the decision. The identifier will be included in the
  decision log event for this decision. Callers can use the identifier for
  correlation purposes.

The examples below assume the following policy:

```live:input_example:module:read_only
package opa.examples

import input.example.flag

allow_request { flag == true }
```

#### Example Request

```http
POST /v1/data/opa/examples/allow_request HTTP/1.1
Content-Type: application/json
```

```json
{
  "input": {
    "example": {
      "flag": true
    }
  }
}
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": true
}
```

#### Example Request

```http
POST /v1/data/opa/examples/allow_request HTTP/1.1
Content-Type: application/json
```

```json
{
  "input": {
    "example": {
      "flag": false
    }
  }
}
```

#### Example Response

```http
HTTP/1.1 200 OK
```

```json
{}
```

### Get a Document (Webhook)

```
POST /v0/data/{path:.+}
Content-Type: application/json
```

Get a document from a webhook.

Use this API if you are enforcing policy decisions via webhooks that have pre-defined
request/response formats. Note, the API path prefix is `/v0` instead of `/v1`.

The request message body defines the content of the [The input
Document](../philosophy/#the-opa-document-model). The request message body
may be empty. The path separator is used to access values inside object and
array documents.

#### Request Headers

- **Content-Type: application/x-yaml**: Indicates the request body is a YAML encoded object.
- **Content-Encoding: gzip**: Indicates the request body is a gzip encoded object.
- **Accept-Encoding: gzip**: Indicates the server should respond with a gzip encoded body. The server will send the compressed response only if its length is above `server.encoding.gzip.min_length` value. See the configuration section

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - not found
- **500** - server error

If the requested document is missing or undefined, the server will return 404 and the message body will contain an error object.

The examples below assume the following policy:

```live:webhook_example:module:read_only
package opa.examples

import input.example.flag

allow_request { flag == true }
```

#### Example Request

```http
POST /v0/data/opa/examples/allow_request HTTP/1.1
Content-Type: application/json
```

```json
{
  "example": {
    "flag": true
  }
}
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
true
```

### Create or Overwrite a Document

```
PUT /v1/data/{path:.+}
Content-Type: application/json
```

Create or overwrite a document.

If the path does not refer to an existing document, the server will attempt to create all of the necessary containing documents. This behavior is similar in principle to the Unix command `mkdir -p`.

The server will respect the `If-None-Match` header if it is set to `*`. In this case, the server will not overwrite an existing document located at the path.

#### Query Parameters

- **metrics** - Return performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **204** - no content (success)
- **304** - not modified
- **400** - bad request
- **404** - write conflict
- **500** - server error

If the path refers to a virtual document or a conflicting base document the server will respond with 404. A base document conflict will occur if the parent portion of the path refers to a non-object document.

#### Example Request To Initialize Document With If-None-Match

```http
PUT /v1/data/us-west/servers HTTP/1.1
Content-Type: application/json
If-None-Match: *
```

```json
{}
```

#### Example Response If Document Already Exists

```http
HTTP/1.1 304 Not Modified
```

#### Example Response If Document Does Not Exist

```http
HTTP/1.1 204 No Content
```

### Patch a Document

```
PATCH /v1/data/{path:.+}
Content-Type: application/json-patch+json
```

Update a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

The server accepts updates encoded as JSON Patch operations. The message body of the request should contain a JSON encoded array containing one or more JSON Patch operations. Each operation specifies the operation type, path, and an optional value. For more information on JSON Patch, see [RFC 6902](https://tools.ietf.org/html/rfc6902).

#### Status Codes

- **204** - no content (success)
- **400** - bad request
- **404** - not found
- **500** - server error

The effective path of the JSON Patch operation is obtained by joining the path portion of the URL with the path value from the operation(s) contained in the message body. In all cases, the parent of the effective path MUST refer to an existing document, otherwise the server returns 404. In the case of **remove** and **replace** operations, the effective path MUST refer to an existing document, otherwise the server returns 404.

#### Example Request

```http
PATCH /v1/data/servers HTTP/1.1
Content-Type: application/json-patch+json
```

```json
[
    {"op": "add",
     "path": "-",
     "value": {
         "id": "s5",
         "name": "job",
         "protocols": ["amqp"],
         "ports": ["p3"]
     }}
]
```

#### Example Response

```http
HTTP/1.1 204 No Content
```

### Delete a Document

```
DELETE /v1/data/{path:.+}
```

Delete a document.

The server processes the DELETE method as if the client had sent a PATCH request containing a single remove operation.

#### Query Parameters

- **metrics** - Return performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **204** - no content (success)
- **404** - not found
- **500** - server error

If the path refers to a non-existent document, the server returns 404.

#### Example Request

```http
DELETE /v1/data/servers HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 204 No Content
```

## Query API

### Execute a Simple Query

```
POST /
Content-Type: application/json
```

Execute a simple query.

OPA serves POST requests without a URL path by querying for the document at
path `/data/system/main`. The content of that document defines the response
entirely.

#### Request Headers

- **Content-Type: application/x-yaml**: Indicates the request body is a YAML encoded object.

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - not found
- **500** - server error

If the default decision (defaulting to `/system/main`) is undefined, the server returns 404.

The policy example below shows how to define a rule that will
produce a value for the `/data/system/main` document. You can configure OPA
to use a different URL path to serve these queries. See the [Configuration Reference](../configuration)
for more information.

The request message body is mapped to the [Input Document](../philosophy/#the-opa-document-model).

```http
PUT /v1/policies/example1 HTTP/1.1
Content-Type: text/plain
```

```live:system_example:module:read_only
package system

main = msg {
  msg := sprintf("hello, %v", [input.user])
}
```

#### Example Request

```http
POST /
Content-Type: application/json
```

```json
{
  "user": ["alice"]
}
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
"hello, alice"
```

### Execute an Ad-hoc Query

Execute an ad-hoc query and return bindings for variables found in the query.

```
GET /v1/query
```

#### Query Parameters

- **q** - The ad-hoc query to execute. OPA will parse, compile, and execute the query represented by the parameter value. The value MUST be URL encoded. Only used in GET method. For POST method the query is sent as part of the request body and this parameter is not used.
- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **notes**, **fails**, **full**, **debug**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error
- **501** - streaming not implemented

For queries that have large JSON values it is recommended to use the `POST` method with the query included as the `POST` body:

```
POST /v1/query HTTP/1.1
Content-Type: application/json
```

```json
{
  "query": "input.servers[i].ports[_] = \"p2\"; input.servers[i].name = name",
  "input": {
    "servers": [ ... ],
  }
}
```

#### Example Request

```
GET /v1/query?q=data.servers[i].ports[_] = "p2"; data.servers[i].name = name HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "i": 3,
      "name": "dev"
    },
    {
      "i": 0,
      "name": "app"
    }
  ]
}
```

## Compile API

### Partially Evaluate a Query

```http
POST /v1/compile
Content-Type: application/json
```

Partially evaluate a query.

The [Compile API](#compile-api) allows you to partially evaluate Rego queries
and obtain a simplified version of the policy. This is most useful when building
integrations where policy logic is to be translated and evaluated in another
environment. For example, 
[this post](https://blog.openpolicyagent.org/write-policy-in-opa-enforce-policy-in-sql-d9d24db93bf4)
on the OPA blog shows how SQL can be generated based on Compile API output. 
For more details on Partial Evaluation in OPA, please refer to
[this blog post](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422).

#### Request Body

Compile API requests contain the following fields:

| Field | Type | Required | Description |
| --- | --- | --- | --- |
| `query` | `string` | Yes | The query to partially evaluate and compile. |
| `input` | `any` | No | The input document to use during partial evaluation (default: undefined). |
| `options`  | `object[string, any]`           | No | Additional options to use during partial evaluation. Only `disableInlining` option is supported. (default: undefined). |
| `unknowns` | `array[string]` | No | The terms to treat as unknown during partial evaluation (default: `["input"]`]). |

### Request Headers

- **Content-Encoding: gzip**: Indicates the request body is a gzip encoded object.
- **Accept-Encoding: gzip**: Indicates the server should respond with a gzip encoded body. The server will send the compressed response only if its length is above `server.encoding.gzip.min_length` value

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **notes**, **fails**, **full**, **debug**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **instrument** - Instrument query evaluation and return a superset of performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The example below assumes that OPA has been given the following policy:

```live:compile_example:module:read_only
package example

allow {
  input.subject.clearance_level >= data.reports[_].clearance_level
}
```

#### Example Request

```http
POST /v1/compile HTTP/1.1
Content-Type: application/json
```

```json
{
  "query": "data.example.allow == true",
  "input": {
    "subject": {
      "clearance_level": 4
    }
  },
  "options": {
    "disableInlining": []
  },
  "unknowns": [
    "data.reports"
  ]
}
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": {
    "queries": [
      [
        {
          "index": 0,
          "terms": [
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "gte"
                }
              ]
            },
            {
              "type": "number",
              "value": 4
            },
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "data"
                },
                {
                  "type": "string",
                  "value": "reports"
                },
                {
                  "type": "var",
                  "value": "i1"
                },
                {
                  "type": "string",
                  "value": "clearance_level"
                }
              ]
            }
          ]
        }
      ]
    ]
  }
}
```

#### Unconditional Results from Partial Evaluation

When you partially evaluate a query with the Compile API, OPA returns a new set of queries and supporting policies. However, in some cases, the result of Partial Evaluation is a conclusive, unconditional answer.

For example, if you extend to policy above to include a "break glass" condition, the decision may be to allow all requests regardless of clearance level.

```live:compile_unconditional_example:module:read_only
package example

allow {
  input.subject.clearance_level >= data.reports[_].clearance_level
}

allow {
  data.break_glass = true
}
```

In this case, if `data.break_glass` is `true` then the query
`data.example.allow == true` will _always_ be true. If the query is
always true, the `"queries"` value in the result will contain an empty
array. The empty array indicates that your query can be satisfied
without any further evaluation.

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": {
    "queries": [
      [],
      [
        {
          "index": 0,
          "terms": [
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "gte"
                }
              ]
            },
            {
              "type": "number",
              "value": 4
            },
            {
              "type": "ref",
              "value": [
                {
                  "type": "var",
                  "value": "data"
                },
                {
                  "type": "string",
                  "value": "reports"
                },
                {
                  "type": "var",
                  "value": "$02"
                },
                {
                  "type": "string",
                  "value": "clearance_level"
                }
              ]
            }
          ]
        }
      ]
    ]
  }
}

```

It is also possible for queries to _never_ be true. For example, the
original policy could be extended to require that users be granted an
exception:

```live:compile_unconditional_false_example:module:read_only
package example

allow {
  input.subject.clearance_level >= data.reports[_].clearance_level
  exceptions[input.subject.name]
}

exceptions["bob"]
exceptions["alice"]
```

In this case, if we execute query on behalf of a user that does not
have an exception (e.g., `"eve"`), the OPA response will not contain a
`queries` field at all. This indicates there are NO conditions that
could make the query true.

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": {}
}
```

The following table summarizes the behavior for partial evaluation results.

| Example Query | Unknowns | Result | Description |
| --- | --- | --- | ---
| `input.x > 0` | `["input"]` | `{"result": {"queries": [[input.x > 0]]}}` | The query is partially evaluated and remaining conditions are returned. |
| `input.x > 0` | Not specified. | `{"result": {"queries": [[input.x > 0]]}}` | If the set of unknowns is not specified, it defaults to `["input"]`. |
| `input.x > 0` | `[]` | `{"result": {}}` | The query is false/undefined because there are no unknowns. |
| `1 > 0` | N/A | `{"result": {"queries": [[]]}}` | The query is always true. |
| `1 < 0` | N/A | `{"result": {}}` | The query is always false. |

> The partially evaluated queries are represented as strings in the table above. The actual API response contains the JSON AST representation.


## Health API

The `/health` API endpoint executes a simple built-in policy query to verify
that the server is operational. Optionally it can account for bundle activation as well
(useful for "ready" checks at startup).

#### Query Parameters
* `bundles` - Boolean parameter to account for bundle activation status in response. This includes any discovery bundles or bundles defined in the loaded discovery configuration.
* `plugins` - Boolean parameter to account for plugin status in response.
* `exclude-plugin` - String parameter to exclude a plugin from status checks. Can be added multiple times. Does nothing if `plugins` is not true. This parameter is useful for special use cases where a plugin depends on the server being fully initialized before it can fully initialize itself.

#### Status Codes
- **200** - OPA service is healthy. If the `bundles` option is specified then all configured bundles have
            been activated. If the `plugins` option is specified then all plugins are in an OK state.
- **500** - OPA service is not healthy. If the `bundles` option is specified this can mean any of the configured
            bundles have not yet been activated. If the `plugins` option is specified then at least one
            plugin is in a non-OK state.

{{< info >}}
The bundle activation check is only for initial bundle activation. Subsequent
downloads will not affect the health check. The [Status](../management-status)
API should be used for more fine-grained bundle status monitoring.
{{< /info >}}

#### Example Request
```http
GET /health HTTP/1.1
```

#### Example Request (bundle activation)
```http
GET /health?bundles HTTP/1.1
```

#### Example Request (plugin status)
```http
GET /health?plugins HTTP/1.1
```

#### Example Request (plugin status with exclude)
```http
GET /health?plugins&exclude-plugin=decision-logs&exclude-plugin=status HTTP/1.1
```

#### Healthy Response
```http
HTTP/1.1 200 OK
Content-Type: application/json
```
```json
{}
```

#### Unhealthy Response
```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json
```
```json
{
  "error": "not all plugins in OK state"
}
```

Other error messages include:

- `"unable to perform evaluation"`
- `"not all configured bundles have been activated"`

### Custom Health Checks

The Health API includes support for "all or nothing" checks that verify
configured bundles have activated and plugins are operational. In some cases,
health checks may need to perform fine-grained checks on plugin state or other
internal components. To support these cases, use the policy-based Health API.

By convention, the `/health/live` and `/health/ready` API endpoints allow you to
use Rego to evaluate the current state of the server and its plugins to
determine "liveness" (when OPA is capable of receiving traffic) and "readiness"
(when OPA is ready to receive traffic). Policy for the `live` and `ready` rules
is defined under package `system.health`.

> The "liveness" and "readiness" check convention comes from
> [Kubernetes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
> but they are just conventions. You can implement your own check endpoints
> under the `system.health` package as needed. Any rules implemented inside of
> `system.health` will be exposed at `/health/<rule-name>`.

#### Policy Examples

Here is a basic health policy for liveness and readiness. In this example, OPA is live once it is
able to process the `live` rule. OPA is ready once all plugins have entered the OK state at least once.

```live:health_policy_example:module:read_only
package system.health

# opa is live if it can process this rule
default live = true

# by default, opa is not ready
default ready = false

# opa is ready once all plugins have reported OK at least once
ready {
  input.plugins_ready
}
```

Note that once `input.plugins_ready` is true, it stays true. If you want to fail the ready check when
specific a plugin leaves the OK state, try this:

```live:health_policy_example_2:module:read_only
package system.health

default live = true

default ready = false

# opa is ready once all plugins have reported OK at least once AND
# the bundle plugin is currently in an OK state
ready {
  input.plugins_ready
  input.plugin_state.bundle == "OK"
}
```

See the following section for all the inputs available to use in health policy.

#### Policy Inputs

- `input.plugins_ready`: Will be false until all registered plugins have started
and are reporting an `OK` state, at which point it will be true. Once true, it will stay true
until the process ends.
- `input.plugin_state.<plugin_name>`: Shows the current state of a plugin, where `<plugin_name>`
is replaced with the name of the plugin, e.g. `bundle`, `status`.

#### Status Codes

- **200** - OPA service is healthy.
- **500** - OPA service is not healthy because policy has not evaluated to true, or is missing.

#### Example Requests

```http
GET /health/ready HTTP/1.1
```

```http
GET /health/live HTTP/1.1
```

#### Healthy Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{}
```

#### Unhealthy Response

```http
HTTP/1.1 500 Internal Server Error
Content-Type: application/json
```

```json
{
  "error": "health policy was not true at data.system.health.<rule_name>"
}
```

Other error messages include:

- `"health policy was undefined at data.system.health.<rule_name>"`


##  Config API

The `/config` API endpoint returns OPA's active configuration. When the discovery feature is enabled, this API can be
used to fetch the discovered configuration in the last evaluated discovery bundle. The `credentials` field in the
[Services](../configuration#services) configuration and the `private_key` and `key` fields in the [Keys](../configuration#keys)
configuration will be omitted from the API response.

### Get Config

```
GET /v1/config HTTP/1.1
```

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **500** - server error

#### Example Request
```http
GET /v1/config HTTP/1.1
```

#### Example Response
```http
HTTP/1.1 200 OK
Content-Type: application/json
```
```json
{
  "result": {
    "services": {
      "acmecorp": {
        "url": "https://example.com/control-plane-api/v1"
      }
    },
    "labels": {
      "id": "test-id",
      "version": "0.27.0"
    },
    "keys": {
      "global_key": {
        "scope": "read"
      }
    },
    "decision_logs": {
      "service": "acmecorp"
    },
    "status": {
      "service": "acmecorp"
    },
    "bundles": {
      "authz": {
        "service": "acmecorp"
      }
    },
    "default_authorization_decision": "/system/authz/allow",
    "default_decision": "/system/main"
  }
}
```

## Status API

The `/status` endpoint exposes a pull-based API for accessing OPA
[Status](../management-status) information. Normally this information is pushed
by OPA to a remote service via HTTP, console, or custom plugins. However, in
some cases, callers may wish to poll OPA and fetch the information.

### Get Status

```
GET /v1/status HTTP/1.1
```

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **500** - server error

#### Example Request
```http
GET /v1/status HTTP/1.1
```

#### Example Response
```http
HTTP/1.1 200 OK
Content-Type: application/json
```
```json
{
  "result": {
    "labels": {
      "id": "7da62ac6-42e0-4b3c-b6d5-199239ad436e",
      "version": "99.9.9-dev"
    },
    "bundles": {
      "play": {
        "name": "play",
        "active_revision": "b3BlbnBvbGljeWFnZW50Lm9yZw==",
        "last_successful_activation": "2021-12-08T01:36:14.201927Z",
        "last_successful_download": "2021-12-08T01:36:14.20038Z",
        "last_successful_request": "2021-12-08T01:36:23.131346Z",
        "last_request": "2021-12-08T01:36:23.131346Z",
        "metrics": {
          "timer_bundle_request_ns": 168273779
        }
      }
    },
    "metrics": {
      "prometheus": {
<------------------8<------------------>
      }
    },
    "plugins": {
      "bundle": {
        "state": "OK"
      },
      "decision_logs": {
        "state": "OK"
      },
      "discovery": {
        "state": "OK"
      },
      "status": {
        "state": "OK"
      }
    }
  }
}
```

## Authentication

The API is secured via [HTTPS, Authentication, and Authorization](../security).

###  Bearer Tokens

When OPA is started with the ``--authentication=token`` command line flag,
clients MUST provide a Bearer token in the HTTP Authorization header:

```http
GET /v1/data/exempli-gratia HTTP/1.1
Authorization: Bearer my-secret-token
```

Bearer tokens must be represented with a valid HTTP header value character
sequence.

OPA will extract the Bearer token value (which is set to ``my-secret-token``
above) and provide it to the authorization component inside OPA that will (i)
validate the token and (ii) execute the authorization policy configured by the
admin.

## Errors

All of the API endpoints use standard HTTP status codes to indicate success or
failure of an API call. If an API call fails, the response will contain a JSON
encoded object that provides more detail. The `errors` and `location` fields are
optional:

```
{
  "code": "invalid_parameter",
  "message": "error(s) occurred while compiling module(s)",
  "errors": [
    {
      "code": "rego_unsafe_var_error",
      "message": "var x is unsafe",
      "location": {
        "file": "example",
        "row": 3,
        "col": 1
      }
    }
  ]
}
```

### Method not Allowed

OPA will respond with a 405 Error (Method Not Allowed) if the method used to access the URL is not supported. For example, if a client uses the *HEAD* method to access any path within "/v1/data/{path:.*}", a 405 will be returned.

## Explanations

OPA supports query explanations that describe (in detail) the steps taken to
produce query results.

Explanations can be requested for:

- [Data API](#data-api) GET queries
- [Query API](#query-api) queries

Explanations are requested by setting the `explain` query parameter to one of
the following values:

- **off** - do not return any trace.
- **full** - returns a full query trace containing every step in the query evaluation process.
- **debug** - returns a full query trace including debug info.
- **notes** - returns only note events and their context.
- **fails** - returns only fail events and their context.

By default, explanations are represented in a machine-friendly format. Set the
`pretty` parameter to request a human-friendly format for debugging purposes.

### Trace Events

When the `explain` query parameter is set to anything except `off`, the response contains an array of Trace Event objects.

Trace Event objects contain the following fields:

- **op** - identifies the kind of Trace Event. Values: **"Enter"**, **"Exit"**, **"Eval"**, **"Fail"**, **"Redo"**.
- **query_id** - uniquely identifies the query that the Trace Event was emitted for.
- **parent_id** - identifies the parent query.
- **type** - indicates the type of the **node** field. Values: **"expr"**, **"rule"**, **"body"**.
- **node** - contains the AST element associated with the evaluation step.
- **locals** - contains the term bindings from the query at the time when the Trace Event was emitted.

#### Query IDs

Queries often reference rules or contain comprehensions. In both cases, query
evaluation involves evaluation of one or more other queries, e.g., the body of
the rule or comprehension.

Trace Events from different queries can be distinguished by the **query_id**
field.

Trace Events from related queries can be identified by the **parent_id** field.

For example, if query A references a rule R, Trace Events emitted as part of
evaluating rule R's body will have the **parent_id** field set to query A's
**query_id**.

#### Types of Events

Each Trace Event represents a step in the query evaluation process. Trace Events
are emitted at the following points:

- **enter** - before a body or rule is evaluated.
- **exit** - after a body or rule has evaluated successfully.
- **eval** - before an expression is evaluated.
- **fail** - after an expression has evaluated to false.
- **redo** - before evaluation restarts from a body, rule, or expression.

By default, OPA searches for all sets of term bindings that make all expressions
in the query evaluate to true. Because there may be multiple answers, the search
can *restart* when OPA determines the query is true or false. When the search
restarts, a **Redo** Trace Event is emitted.

#### Example Trace Event

```json
{
  "op": "eval",
  "query_id": 20,
  "parent_id": 0,
  "type": "expr",
  "node": {
    "index": 1,
    "terms": [
      {
        "type": "var",
        "value": "eq"
      },
      {
        "type": "var",
        "value": "x"
      },
      {
        "type": "var",
        "value": "y"
      }
    ]
  },
  "locals": [
    {
      "key": {
        "type": "var",
        "value": "x"
      },
      "value": {
        "type": "string",
        "value": "hello"
      }
    }
  ]
}
```

## Performance Metrics

OPA can report detailed performance metrics at runtime. Performance metrics can
be requested on individual API calls and are returned inline with the API
response. To enable performance metric collection on an API call, specify the
`metrics=true` query parameter when executing the API call. Performance metrics
are currently supported for the following APIs:

- Policy API (PUT and DELETE)
- Data API (GET, POST, PUT, and DELETE)
- Query API (all methods)
- Compile API (POST)

For example:

```http
POST /v1/data/example?metrics=true HTTP/1.1
```

Response:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "metrics": {
    "timer_rego_query_compile_ns": 69994,
    "timer_rego_query_eval_ns": 48425,
    "timer_rego_query_parse_ns": 4096
  },
  "result": {
    "some_strings": [
      "hello",
      "world"
    ]
  }
}
```

OPA currently supports the following query performance metrics:

- **timer_rego_input_parse_ns**: time taken (in nanoseconds) to parse the input
- **timer_rego_query_parse_ns**: time taken (in nanoseconds) to parse the query.
- **timer_rego_query_compile_ns**: time taken (in nanoseconds) to compile the query.
- **timer_rego_query_eval_ns**: time taken (in nanoseconds) to evaluate the query.
- **timer_rego_module_parse_ns**: time taken (in nanoseconds) to parse the input policy module.
- **timer_rego_module_compile_ns**: time taken (in nanoseconds) to compile the loaded policy modules.
- **timer_server_handler_ns**: time take (in nanoseconds) to handle the API request.
- **counter_server_query_cache_hit**: number of cache hits for the query.

The `counter_server_query_cache_hit` counter gives an indication about whether OPA creates a new Rego query
or it uses a pre-processed query which holds some prepared state to serve the API request. A pre-processed query will be
faster to evaluate since OPA will not have to re-parse or compile it. Hence, when the query is served from the cache
`timer_rego_query_parse_ns` and `timer_rego_query_compile_ns` timers will be omitted from the reported performance metrics.

OPA also supports query instrumentation. To enable query instrumentation,
specify the `instrument=true` query parameter when executing the API call.
Query instrumentation can help diagnose performance problems, however, it can
add significant overhead to query evaluation. We recommend leaving query
instrumentation off unless you are debugging a performance problem.

When instrumentation is enabled there are several additional performance metrics
for the compilation stages. They follow the format of `timer_compile_stage_*_ns`
and `timer_query_compile_stage_*_ns` for the query and module compilation stages.

## Provenance

OPA can report provenance information at runtime. Provenance information can
be requested on individual API calls and are returned inline with the API
response. To obtain provenance information on an API call, specify the
`provenance=true` query parameter when executing the API call. Provenance information
is currently supported for the following APIs:

- Data API (GET and POST)

For example:

```http
POST /v1/data/example?provenance=true HTTP/1.1
```

Response:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "provenance": {
    "build_commit": "1955fc4d",
    "build_host": "foo.com",
    "build_timestamp": "2019-04-29T23:42:04Z",
    "bundles": {
      "authz": {
        "revision": "ID-b1298a6c-6ad8-11e9-a26f-d38b5ceadad5"
      }
    },
    "version": "0.10.8-dev"
  },
  "result": true
}
```

OPA currently supports the following query provenance information:

- **version**: The version of this OPA instance.
- **build_commit**: The git commit id of this OPA build.
- **build_timestamp**: The timestamp when this instance was built.
- **build_host**: The hostname where this instance was built.
- **revision**: (Deprecated) The _revision_ string included in a .manifest file (if present) within
  a bundle. Omitted when `bundles` are configured.
- **bundles**: A set of key-value pairs describing each bundle activated on the server. Includes
  the `revision` field which is the _revision_ string included in a .manifest file (if present)
  within a bundle

## Ecosystem Projects

OPA's REST API has already been used by many projects in the OPA Ecosystem to support a variety of use cases. 

{{< ecosystem_feature_embed key="rest-api-integration" topic="built on the OPA REST API" >}} 
