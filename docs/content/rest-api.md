---
title: REST API
kind: documentation
weight: 80
restrictedtoc: true
---

This document is the authoritative specification of the OPA REST API.

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

### Query Parameters

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

The Data API exposes endpoints for reading and writing documents in OPA. For an introduction to the different types of documents in OPA see [How Does OPA Work?](../#how-does-opa-work).

### Get a Document

```
GET /v1/data/{path:.+}
```

Get a document.

The path separator is used to access values inside object and array documents. If the path indexes into an array, the server will attempt to convert the array index to an integer. If the path element cannot be converted to an integer, the server will respond with 404.

#### Query Parameters

- **input** - Provide an input document. Format is a JSON value that will be used as the value for the input document.
- **pretty** - If parameter is `true`, response will formatted for humans.
- **provenance** - If parameter is `true`, response will include build/version info in addition to the result.  See [Provenance](#provenance) for more detail.
- **explain** - Return query explanation in addition to result. Values: **full**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **instrument** - Instrument query evaluation and return a superset of performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
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

The request body contains an object that specifies a value for [The input Document](../#the-input-document).

#### Request Headers

- **Content-Type: application/x-yaml**: Indicates the request body is a YAML encoded object.

#### Query Parameters

- **partial** - Use the partial evaluation (optimization) when evaluating the query.
- **pretty** - If parameter is `true`, response will formatted for humans.
- **provenance** - If parameter is `true`, response will include build/version info in addition to the result.  See [Provenance](#provenance) for more detail.
- **explain** - Return query explanation in addition to result. Values: **full**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **instrument** - Instrument query evaluation and return a superset of performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **watch** - Set a watch on the data reference if the parameter is present. See [Watches](#watches) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

The server returns 400 if either:

1. The query requires an input document and the client did not supply one.
2. The query already defines an input document and the client did supply one.

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

```live:input_exmaple:module:read_only
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
Document](../#the-input-document). The request message body
may be empty. The path separator is used to access values inside object and
array documents.

#### Request Headers

- **Content-Type: application/x-yaml**: Indicates the request body is a YAML encoded object.

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

The request message body is mapped to the [Input Document](../#the-input-document).

```http
PUT /v1/policies/example1 HTTP/1.1
Content-Type: text/plain
```

```live:system_example:module:read_only
package system

main = msg {
  msg := sprintf("hello, %v", input.user)
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
- **explain** - Return query explanation in addition to result. Values: **full**.
- **metrics** - Return query performance metrics in addition to result. See [Performance Metrics](#performance-metrics) for more detail.
- **watch** - Set a watch on the query if the parameter is present. See [Watches](#watches) for more detail.

#### Status Codes

- **200** - no error
- **400** - bad request
- **404** - watch ID not found
- **500** - server error
- **501** - streaming not implemented

For queries that have large JSON values it is recommended to use the `POST` method with the query included as the `POST` body

```
POST /v1/query HTTP/1.1
Content-Type: application/json
```

```json
{
  "query": "data.servers[i].ports[_] = \"p2\"; data.servers[i].name = name"
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
and obtain a simplified version of the policy. For more details on Partial
Evaluation in OPA, see [this post on blog.openpolicyagent.org](https://blog.openpolicyagent.org/partial-evaluation-162750eaf422).

#### Request Body

Compile API requests contain the following fields:

| Field | Type | Requried | Description |
| --- | --- | --- | --- |
| `query` | `string` | Yes | The query to partially evaluate and compile. |
| `input` | `any` | No | The input document to use during partial evaluation (default: undefined). |
| `unknowns` | `array[string]` | No | The terms to treat as unknown during partial evaluation (default: `["input"]`]). |

#### Query Parameters

- **pretty** - If parameter is `true`, response will formatted for humans.
- **explain** - Return query explanation in addition to result. Values: **full**.
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

- **full** - returns a full query trace containing every step in the query evaluation process.

By default, explanations are represented in a machine-friendly format. Set the
`pretty` parameter to request a human-friendly format for debugging purposes.

### Trace Events

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

## Performance Metrics

OPA can report detailed performance metrics at runtime. Performance metrics can
be requested on individual API calls and are returned inline with the API
response. To enable performance metric collection on an API call, specify the
`metrics=true` query parameter when executing the API call. Performance metrics
are currently supported for the following APIs:

- Data API (GET and POST)
- Policy API (all methods)
- Query API (all methods)

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
- **timer_rego_query_parse_ns**: time taken (in nanonseconds) to parse the query.
- **timer_rego_query_compile_ns**: time taken (in nanonseconds) to compile the query.
- **timer_rego_query_eval_ns**: time taken (in nanonseconds) to evaluate the query.
- **timer_rego_module_parse_ns**: time taken (in nanoseconds) to parse the input policy module.
- **timer_rego_module_compile_ns**: time taken (in nanoseconds) to compile the loaded policy modules.
- **timer_server_handler_ns**: time take (in nanoseconds) to handle the API request.

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

## Watches

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


## Health API

The `/health` API endpoint executes a simple built-in policy query to verify
that the server is operational. Optionally it can account for bundle activation as well
(useful for "ready" checks at startup).

#### Query Parameters
`bundle` - Boolean parameter to account for bundle activation status in response.

#### Status Codes
- **200** - OPA service is healthy. If `bundle=true` then all configured bundles have
            been activated.
- **500** - OPA service is not healthy. If `bundle=true` this can mean any of the configured
            bundles have not yet been activated.

> *Note*: The bundle activation check is only for initial startup. Subsequent downloads
  will not affect the health check. The [Status](../management/#status)
  API should be used for more fine-grained bundle status monitoring.

#### Example Request
```http
GET /health HTTP/1.1
```

#### Example Request (bundle activation)
```http
GET /health?bundle=true HTTP/1.1
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
{}
```
