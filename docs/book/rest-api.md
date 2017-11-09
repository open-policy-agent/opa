# REST API

This document is the authoritative specification of the OPA REST API.

## <a name="policy-api"/> Policy API

The Policy API exposes CRUD endpoints for managing policy modules. Policy modules can be added, removed, and modified at any time.

The identifiers given to policy modules are only used for management purposes. They are not used outside of the Policy API.

### <a name="list-policies"/>List Policies

```
GET /v1/policies
```

List policy modules.

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

#### Status Codes

- **200** - no error
- **500** - server error

### <a name="get-a-policy"/>Get a Policy

```
GET /v1/policies/<id>
```

Get a policy module.

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

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **404** - not found
- **500** - server error

### <a name="create-or-update-a-policy"/>Create or Update a Policy

```
PUT /v1/policies/<id>
Content-Type: text/plain
```

Create or update a policy module.

If the policy module does not exist, it is created. If the policy module already exists, it is replaced.

#### Example Request

```http
PUT /v1/policies/example1 HTTP/1.1
Content-Type: text/plain
```

```ruby
package opa.examples

import data.servers
import data.networks
import data.ports

public_servers[server] {
	server = servers[_]
	server.ports[_] = ports[k].id
	ports[k].networks[_] = networks[m].id
	networks[m].public = true
}
```

> cURL's `-d/--data` flag removes newline characters from input files. Use the `--data-binary` flag instead.
{: .opa-tip}

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{}
```

### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **metrics** - Return compiler performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

Before accepting the request, the server will parse, compile, and install the policy module. If the policy module is invalid, one of these steps will fail and the server will respond with 400. The error message in the response will be set to indicate the source of the error.

### <a name="delete-a-policy"/>Delete a Policy

```
DELETE /v1/policies/<id>
```

Delete a policy module.

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

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **metrics** - Return compiler performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **204** - no content
- **400** - bad request
- **404** - not found
- **500** - server error

If other policy modules in the same package depend on rules in the policy module to be deleted, the server will return 400.

## <a name="data-api"></a> Data API

The Data API exposes endpoints for reading and writing documents in OPA. For an introduction to the different types of documents in OPA see [How Does OPA Work?](/how-does-opa-work.md).

### <a name="get-a-document"/>Get a Document

```
GET /v1/data/{path:.+}
```

Get a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

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

#### Example Watch Request

If we make the following GET request:

```http
GET /v1/data/servers?watch&pretty=true HTTP/1.1
```

Followed by these PATCH requests:

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

```http
PATCH /v1/data/servers HTTP/1.1
Content-Type: application/json-patch+json
```

```json
[
    {
     "op": "remove",
     "path": "1"
    }
]
```

#### Example Watch Response

The response below represents the response _after_ the chunked encoding has been processed.
It is not complete, as further changes to `/data/servers` would cause more notifications to be streamed.

```http
HTTP/1.1 200 OK
Content-Type: application/json
Transfer-Encoding: chunked
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
      "id": "s2",
      "name": "db",
      "ports": [
        "p3"
      ],
      "protocols": [
        "mysql"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "ports": [
        "p3"
      ],
      "protocols": [
        "memcache",
        "http"
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
      "id": "s2",
      "name": "db",
      "ports": [
        "p3"
      ],
      "protocols": [
        "mysql"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "ports": [
        "p3"
      ],
      "protocols": [
        "memcache",
        "http"
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
    },
    {
      "id": "s5",
      "name": "job",
      "ports": [
        "p3"
      ],
      "protocols": [
        "amqp"
      ]
    }
  ]
}

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
      "id": "s3",
      "name": "cache",
      "ports": [
        "p3"
      ],
      "protocols": [
        "memcache",
        "http"
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
    },
    {
      "id": "s5",
      "name": "job",
      "ports": [
        "p3"
      ],
      "protocols": [
        "amqp"
      ]
    }
  ]
}
```

#### Query Parameters

- **input** - Provide an input document. Format is a JSON value that will be used as the value for the input document.
- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **watch** - Set a watch on the data reference if the parameter is present. See [Watches](#watches) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if either:

- The query requires the input document and the caller does not provide it.
- The caller provides the input document but the query already defines it programmatically.

The server returns 200 if the path refers to an undefined document. In this
case, the response will not contain a `result` property.

### <a name="get-a-document-with-input"></a> Get a Document (with Input)

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

The request body contains an object that specifies a value for [The input Document](/how-does-opa-work.md#the-input-document).

The examples below assume the following policy:

```ruby
package opa.examples

import input.example.flag

allow_request { flag = true }
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

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if either:

1. The query requires an input document and the client did not supply one.
2. The query already defines an input document and the client did supply one.

The server returns 200 if the path refers to an undefined document. In this
case, the response will not contain a `result` property.

### <a name="get-a-document-webhook"/>Get a Document (Webhook)

```
POST /v0/data/{path:.+}
Content-Type: application/json
```

Get a document from a webhook.

Use this API if you are enforcing policy decisions via webhooks that have pre-defined
request/response formats. Note, the API path prefix is `/v0` instead of `/v1`.

The request message body defines the content of the [The input
Document](/how-does-opa-work.md#the-input-document). The request message body
may be empty. The path separator is used to access values inside object and
array documents.

The examples below assume the following policy:

```ruby
package opa.examples

import input.example.flag

allow_request { flag = true }
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

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - not found
- **500** - server error

If the requested document is missing or undefined, the server will return 404 and the message body will contain an error object.

### <a name="create-or-overwrite a document"/>Create or Overwrite a Document

```
PUT /v1/data/{path:.+}
Content-Type: application/json
```

Create or overwrite a document.

If the path does not refer to an existing document, the server will attempt to create all of the necessary containing documents. This behavior is similar in principle to the Unix command `mkdir -p`.

The server will respect the `If-None-Match` header if it is set to `*`. In this case, the server will not overwrite an existing document located at the path.

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

#### Status Codes

- **204** - no content (success)
- **304** - not modified
- **404** - write conflict

If the path refers to a virtual document or a conflicting base document the server will respond with 404. A base document conflict will occur if the parent portion of the path refers to a non-object document.

### <a name="patch-a-document"/>Patch a Document

```
PATCH /v1/data/{path:.+}
Content-Type: application/json-patch+json
```

Update a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

The server accepts updates encoded as JSON Patch operations. The message body of the request should contain a JSON encoded array containing one or more JSON Patch operations. Each operation specifies the operation type, path, and an optional value. For more information on JSON Patch, see [RFC 6902](https://tools.ietf.org/html/rfc6902).

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

#### Status Codes

- **204** - no content (success)
- **404** - not found
- **500** - server error

The effective path of the JSON Patch operation is obtained by joining the path portion of the URL with the path value from the operation(s) contained in the message body. In all cases, the parent of the effective path MUST refer to an existing document, otherwise the server returns 404. In the case of **remove** and **replace** operations, the effective path MUST refer to an existing document, otherwise the server returns 404.

## <a name="query-api"></a> Query API

### <a name="execute-a-simple-query"/>Execute a Simple Query

```
POST /
Content-Type: application/json
```

Execute a simple query.

OPA serves POST requests without a URL path by querying for the document at
path `/data/system/main`. The content of that document defines the response
entirely. The example below shows how to define a rule that will produce a
value for the `/data/system/main` document.

The request message body is mapped to the [Input Document](/how-does-opa-work.md#the-input-document).

```ruby
package system

main = msg {
  sprintf("hello, %v", input.user, msg)
}
```

#### Example Request

```http
POST /
Content-Type: application/json
```

```json
{
  "user": "alice"
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

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - not found
- **500** - server error

If the `/data/system/main` document is undefined (e.g., because the administrator has not defined one) the server returns 404.

### <a name="execute-an-ad-hoc-query"/>Execute an Ad-hoc Query

```
GET /v1/query
```

Execute an ad-hoc query and return bindings for variables found in the query.

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

#### Query Parameters

- **q** - The ad-hoc query to execute. OPA will parse, compile, and execute the query represented by the parameter value. The value MUST be URL encoded.
- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **watch** - Set a watch on the query if the parameter is present. See [Watches](#watches) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - watch ID not found
- **500** - server error
- **501** - streaming not implemented

## <a name="authentication"></a> Authentication

The API is secured via [HTTPS, Authentication, and
Authorization](/documentation/references/security).

### <a name="bearer-tokens"/> Bearer Tokens

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

## <a name="errors"></a> Errors

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

## <a name="explanations"></a> Explanations

OPA supports query explanations that describe (in detail) the steps taken to
produce query results.

Explanations can be requested for:

- [Data API](#data-api) GET queries
- [Query API](#query-api) queries

Explanations are requested by setting the `explain` query parameter to one of
the following values:

- **full** - returns a full query trace containing every step in the query evaluation process.

By default, explanations are represented in a machine-friendly format. Set the
`pretty` parameter to request a human-friendly format for debugging purposes.

### <a name="trace-events"/>Trace Events

When the `explain` query parameter is set to **full** , the response contains an array of Trace Event objects.

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

## <a name="performance-metrics"></a> Performance Metrics

OPA can report detailed performance metrics at runtime. Currently, performance metrics can be requested on individual API calls and are returned inline with the API response. To enable performance metric collection on an API call, specify the `metrics=true` query parameter when executing the API call. Performance metrics are currently supported for the following APIs:

- Data API GET
- Data API POST
- Query API

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

- **timer_rego_query_parse_ns**: time taken (in nanonseconds) to parse the query.
- **timer_rego_query_compile_ns**: time taken (in nanonseconds) to compile the query.
- **timer_rego_query_eval_ns**: time taken (in nanonseconds) to evaluate the query.
- **timer_rego_module_parse_ns**: time taken (in nanoseconds) to parse the input policy module.
- **timer_rego_module_compile_ns**: time taken (in nanoseconds) to compile the loaded policy modules.

## <a name="diagnostics"></a> Diagnostics

The OPA server has the capability to log and return diagnostics on past requests to the user. By default, the server will not log any diagnostics at all. In order to have the server start logging diagnostic information, the document located at `data.system.diagnostics.config` (referred to as config below) must be defined. The config must evaluate to an object that is structured as follows:

```
{
    "mode": <"all"/"on"/"off">,
}
```

An example of a policy that defines a simple diagnostics config that only logs GET requests:

```
package system.diagnostics

default config = {"mode": "off"}

config = {"mode": "on"} {
    input.method = "GET"
}
```

If the config document does not conform to the structure above, then diagnostics are automatically disabled.

Whenever a data GET/POST, a POST to `/` or query GET request is received by the server, it evaluates the config document to determine whether or not diagnostics should be logged. The table below describes the behavior of diagnostics depending on the values of the config fields.

| Field | Value | Behavior |
| --- | --- | --- |
| `mode`    | "off" | No diagnostics are recorded. |
| `mode`    | "on"  | Enables collection of inexpensive values. This includes the query, input, result, and performance metrics. |
| `mode`    | "all" | All diagnostics are collected. This includes a full trace of the query evaluation. |

In order to allow the config document to make dynamic decisions about whether or not to record diagnostics for a given request, the server will provide a special input document when evaluating it. The input document will contain information from the HTTP request that was issued to the server, of the form:

```
{
    "method": <HTTP request method>,
    "path": <Full HTTP request path>,
    "body": <HTTP request body (null if not present or invalid JSON)>,

    # The query parameters in the HTTP request.
    "params": {
        "param0": ["value0", "value1", ...],
        "param1": [...],
        ...
    },

    # The header fields in the HTTP request.
    "header": {
        "field0": ["value0", "value1", ...],
        "field1": [...],
        ...
    }
}
```

As an example, the request `GET /v1/data/servers?watch&pretty=true HTTP/1.1` would result in the input document below (the headers will likely depend on the client used to send the request):

```
{
    "method": "GET",
    "path": "/v1/data/servers",
    "body": null,
    "params": {
      "watch": [""],
      "pretty": ["true"],
    },

    "header": { ... }
}
```

Diagnostics may be fetched from the server using the Data GET endpoint. When the server sees a GET request to `/v1/data/system/diagnostics`, it will not evaluate the document located there, but instead return a list of the diagnostics stored on the server (ordered from oldest to newest). The server will honor the `explainMode` and `pretty` parameters from the GET request, but the others will be ignored. The response is of the form:

```
{
  "result": [
    {
      "timestamp": ...,
      "query": ...,
      "input": ...,
      "result": ...,
      "error": ...,
      "explanation": ...,
      "metrics": ...
    },
    ...
  ]
}
```

| Field         | Always Present | Description                                                                                                                                                                                                |
| ------------  | -------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `timestamp`   | Yes            | Nanoseconds since the Unix Epoch time                                                                                                                                                                      |
| `query`       | Yes            | The query, if the request for the record was a query request. Otherwise the data path from the original request.                                                                                           |
| `input`       | No             | Input provided to the `query` at evaluation time.                                                                                                                                                          |
| `result`      | No             | Result of evaluating `query`. See the [Data](#data-api) and [Query](#query-api) APIs for detailed descriptions of formats.                                                                                 |
| `error`       | No             | [Error](#errors) encountered while evaluating `query`.                                                                                                                                                     |
| `metrics`     | No             | [Performance Metrics](#performance-metrics) for `query`.                                                                                                                                                   |
| `explanation` | No             | [Explanation](#explanations) of how `result` was found. |

The server will only store a finite number of diagnostics. If the server's diagnostics storage becomes full, it will delete the oldest diagnostic to make room for the new one. The size of the storage may be configured when the server is started.

## <a name="watches"></a> Watches

OPA can set watches on queries and report whenever the result of evaluating the query has changed. When a watch is set on a query, the requesting connection will be maintained as the query results are streamed back in HTTP Chunked Encoding format. A notification reflecting a certain change to the query results will be delivered _at most once_. That is, if a watch is set on `data.x`, and then multiple writes are made to `data.x`, say `1`, `2` and `3`, only the notification reflecting `data.x=3` is always seen eventually (assuming the watch is not ended, there are no connection problems, etc). The notifications reflecting `data.x=1` and `data.x=2` _might_ be seen. However, the notifications sent are guaranteed to be in order (`data.x=2` will always come after `data.x=1`, if it comes).

The notification stream will not be delimited by any value; the client is expected to be able to parse JSON values from the stream one by one, recognizing where one ends and the next begins.

The notification stream is a stream of JSON objects, each of which has the following structure:
```
{
    "result": <result>,
    "error": <error>,
    "metrics": <metrics>,
    "explanation": <explanation>,
}
```

The `error` field is optional; it is omitted when no errors occur. If it is present, it is an [Error](#errors) describing any errors encountered while evaluating the query the watch was set on. If the policies on the server are changed such that the query no longer compiles, the contents of the error's `message` field will start with the text "watch invalidated:" and will be followed with the reason for invalidation. The watch will be ended by the server and the stream closed.

The `metrics` field represents [Performance Metrics](#performance-metrics) for the evaluation of the query. It will only be present if metrics were requested in the API call that started the watch.

The `explanation` field represents an [Explanation](#explanations) of how the query answer was found. It will only be present if explanations were requested in the API call that started the watch.

If there are no errors, the `result` field will be a JSON array of the results of evaluating the query. The format of a result is:
```
{
    "expressions": [
        {
            "value": true,
            "text": "a = data.x",
            "location":{"row":1,"col":1}
        },
        ...
    ],
    "bindings": {...}
}
```

The `expressions` field is an array of the results of evaluating each of the expressions in the query. `value` is the expression's value, `text` is the actual expression, and `location` describes the location of the expression within the query. The values above are examples.

The `bindings` field is a JSON object mapping `string`s to JSON values that describe the variable bindings resulting from evaluating the query.

If the watch was set on a data reference instead of a query, the `result` field will simply be the value of the document requested, instead of an array of values.
