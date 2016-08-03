---
nav_id: MAIN_DOCUMENTATION
doc_id: REST_API_VERSION_1
layout: documentation

title: REST API
---

{% contentfor header %}

# REST API

This document is the authoritative specification of the OPA REST API (v1). These APIs are the foundation for integrating with OPA using languages other than Go.

{% endcontentfor %}

{% contentfor body %}

## Policy API

The Policy API exposes CRUD endpoints for managing policy modules. Policy modules can be added, removed, or updated at any time.

The identifiers given to policy modules are only used for management purposes. They are not used outside of the Policy API.

### List Policies

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
[
  {
    "ID": "example2",
    "Module": {
      "Package": {
        "Path": [
          {
            "Type": "var",
            "Value": "data"
          },
          {
            "Type": "string",
            "Value": "opa"
          },
          {
            "Type": "string",
            "Value": "examples"
          }
        ]
      },
      "Imports": [
        {
          "Path": {
            "Type": "ref",
            "Value": [
              {
                "Type": "var",
                "Value": "data"
              },
              {
                "Type": "string",
                "Value": "servers"
              }
            ]
          }
        }
      ],
      "Rules": [
        {
          "Name": "violations",
          "Key": {
            "Type": "var",
            "Value": "server"
          },
          "Body": [
            {
              "Terms": [
                {
                  "Type": "var",
                  "Value": "="
                },
                {
                  "Type": "var",
                  "Value": "server"
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "data"
                    },
                    {
                      "Type": "string",
                      "Value": "servers"
                    },
                    {
                      "Type": "var",
                      "Value": "$0"
                    }
                  ]
                }
              ]
            },
            {
              "Terms": [
                {
                  "Type": "var",
                  "Value": "="
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "server"
                    },
                    {
                      "Type": "string",
                      "Value": "protocols"
                    },
                    {
                      "Type": "var",
                      "Value": "$1"
                    }
                  ]
                },
                {
                  "Type": "string",
                  "Value": "http"
                }
              ]
            },
            {
              "Terms": {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "opa"
                  },
                  {
                    "Type": "string",
                    "Value": "examples"
                  },
                  {
                    "Type": "string",
                    "Value": "public_servers"
                  },
                  {
                    "Type": "var",
                    "Value": "server"
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
    "ID": "example1",
    "Module": {
      "Package": {
        "Path": [
          {
            "Type": "var",
            "Value": "data"
          },
          {
            "Type": "string",
            "Value": "opa"
          },
          {
            "Type": "string",
            "Value": "examples"
          }
        ]
      },
      "Imports": [
        {
          "Path": {
            "Type": "ref",
            "Value": [
              {
                "Type": "var",
                "Value": "data"
              },
              {
                "Type": "string",
                "Value": "servers"
              }
            ]
          }
        },
        {
          "Path": {
            "Type": "ref",
            "Value": [
              {
                "Type": "var",
                "Value": "data"
              },
              {
                "Type": "string",
                "Value": "networks"
              }
            ]
          }
        },
        {
          "Path": {
            "Type": "ref",
            "Value": [
              {
                "Type": "var",
                "Value": "data"
              },
              {
                "Type": "string",
                "Value": "ports"
              }
            ]
          }
        }
      ],
      "Rules": [
        {
          "Name": "public_servers",
          "Key": {
            "Type": "var",
            "Value": "server"
          },
          "Body": [
            {
              "Terms": [
                {
                  "Type": "var",
                  "Value": "="
                },
                {
                  "Type": "var",
                  "Value": "server"
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "data"
                    },
                    {
                      "Type": "string",
                      "Value": "servers"
                    },
                    {
                      "Type": "var",
                      "Value": "$0"
                    }
                  ]
                }
              ]
            },
            {
              "Terms": [
                {
                  "Type": "var",
                  "Value": "="
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "server"
                    },
                    {
                      "Type": "string",
                      "Value": "ports"
                    },
                    {
                      "Type": "var",
                      "Value": "$1"
                    }
                  ]
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "data"
                    },
                    {
                      "Type": "string",
                      "Value": "ports"
                    },
                    {
                      "Type": "var",
                      "Value": "k"
                    },
                    {
                      "Type": "string",
                      "Value": "id"
                    }
                  ]
                }
              ]
            },
            {
              "Terms": [
                {
                  "Type": "var",
                  "Value": "="
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "data"
                    },
                    {
                      "Type": "string",
                      "Value": "ports"
                    },
                    {
                      "Type": "var",
                      "Value": "k"
                    },
                    {
                      "Type": "string",
                      "Value": "networks"
                    },
                    {
                      "Type": "var",
                      "Value": "$2"
                    }
                  ]
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "data"
                    },
                    {
                      "Type": "string",
                      "Value": "networks"
                    },
                    {
                      "Type": "var",
                      "Value": "m"
                    },
                    {
                      "Type": "string",
                      "Value": "id"
                    }
                  ]
                }
              ]
            },
            {
              "Terms": [
                {
                  "Type": "var",
                  "Value": "="
                },
                {
                  "Type": "ref",
                  "Value": [
                    {
                      "Type": "var",
                      "Value": "data"
                    },
                    {
                      "Type": "string",
                      "Value": "networks"
                    },
                    {
                      "Type": "var",
                      "Value": "m"
                    },
                    {
                      "Type": "string",
                      "Value": "public"
                    }
                  ]
                },
                {
                  "Type": "boolean",
                  "Value": true
                }
              ]
            }
          ]
        }
      ]
    }
  }
]
```

#### Status Codes

- **200** - no error
- **500** - server error

### Get a Policy

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
  "ID": "example1",
  "Module": {
    "Package": {
      "Path": [
        {
          "Type": "var",
          "Value": "data"
        },
        {
          "Type": "string",
          "Value": "opa"
        },
        {
          "Type": "string",
          "Value": "examples"
        }
      ]
    },
    "Imports": [
      {
        "Path": {
          "Type": "ref",
          "Value": [
            {
              "Type": "var",
              "Value": "data"
            },
            {
              "Type": "string",
              "Value": "servers"
            }
          ]
        }
      },
      {
        "Path": {
          "Type": "ref",
          "Value": [
            {
              "Type": "var",
              "Value": "data"
            },
            {
              "Type": "string",
              "Value": "networks"
            }
          ]
        }
      },
      {
        "Path": {
          "Type": "ref",
          "Value": [
            {
              "Type": "var",
              "Value": "data"
            },
            {
              "Type": "string",
              "Value": "ports"
            }
          ]
        }
      }
    ],
    "Rules": [
      {
        "Name": "public_servers",
        "Key": {
          "Type": "var",
          "Value": "server"
        },
        "Body": [
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "var",
                "Value": "server"
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "servers"
                  },
                  {
                    "Type": "var",
                    "Value": "$0"
                  }
                ]
              }
            ]
          },
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "server"
                  },
                  {
                    "Type": "string",
                    "Value": "ports"
                  },
                  {
                    "Type": "var",
                    "Value": "$1"
                  }
                ]
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "ports"
                  },
                  {
                    "Type": "var",
                    "Value": "k"
                  },
                  {
                    "Type": "string",
                    "Value": "id"
                  }
                ]
              }
            ]
          },
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "ports"
                  },
                  {
                    "Type": "var",
                    "Value": "k"
                  },
                  {
                    "Type": "string",
                    "Value": "networks"
                  },
                  {
                    "Type": "var",
                    "Value": "$2"
                  }
                ]
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "networks"
                  },
                  {
                    "Type": "var",
                    "Value": "m"
                  },
                  {
                    "Type": "string",
                    "Value": "id"
                  }
                ]
              }
            ]
          },
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "networks"
                  },
                  {
                    "Type": "var",
                    "Value": "m"
                  },
                  {
                    "Type": "string",
                    "Value": "public"
                  }
                ]
              },
              {
                "Type": "boolean",
                "Value": true
              }
            ]
          }
        ]
      }
    ]
  }
}
```

#### Status Codes

- **200** - no error
- **404** - not found
- **500** - server error

### Create or Update a Policy

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

public_servers[server] :-
	server = servers[_],
	server.ports[_] = ports[k].id,
	ports[k].networks[_] = networks[m].id,
	networks[m].public = true
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "ID": "example1",
  "Module": {
    "Package": {
      "Path": [
        {
          "Type": "var",
          "Value": "data"
        },
        {
          "Type": "string",
          "Value": "opa"
        },
        {
          "Type": "string",
          "Value": "examples"
        }
      ]
    },
    "Imports": [
      {
        "Path": {
          "Type": "ref",
          "Value": [
            {
              "Type": "var",
              "Value": "data"
            },
            {
              "Type": "string",
              "Value": "servers"
            }
          ]
        }
      },
      {
        "Path": {
          "Type": "ref",
          "Value": [
            {
              "Type": "var",
              "Value": "data"
            },
            {
              "Type": "string",
              "Value": "networks"
            }
          ]
        }
      },
      {
        "Path": {
          "Type": "ref",
          "Value": [
            {
              "Type": "var",
              "Value": "data"
            },
            {
              "Type": "string",
              "Value": "ports"
            }
          ]
        }
      }
    ],
    "Rules": [
      {
        "Name": "public_servers",
        "Key": {
          "Type": "var",
          "Value": "server"
        },
        "Body": [
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "var",
                "Value": "server"
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "servers"
                  },
                  {
                    "Type": "var",
                    "Value": "$0"
                  }
                ]
              }
            ]
          },
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "server"
                  },
                  {
                    "Type": "string",
                    "Value": "ports"
                  },
                  {
                    "Type": "var",
                    "Value": "$1"
                  }
                ]
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "ports"
                  },
                  {
                    "Type": "var",
                    "Value": "k"
                  },
                  {
                    "Type": "string",
                    "Value": "id"
                  }
                ]
              }
            ]
          },
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "ports"
                  },
                  {
                    "Type": "var",
                    "Value": "k"
                  },
                  {
                    "Type": "string",
                    "Value": "networks"
                  },
                  {
                    "Type": "var",
                    "Value": "$2"
                  }
                ]
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "networks"
                  },
                  {
                    "Type": "var",
                    "Value": "m"
                  },
                  {
                    "Type": "string",
                    "Value": "id"
                  }
                ]
              }
            ]
          },
          {
            "Terms": [
              {
                "Type": "var",
                "Value": "="
              },
              {
                "Type": "ref",
                "Value": [
                  {
                    "Type": "var",
                    "Value": "data"
                  },
                  {
                    "Type": "string",
                    "Value": "networks"
                  },
                  {
                    "Type": "var",
                    "Value": "m"
                  },
                  {
                    "Type": "string",
                    "Value": "public"
                  }
                ]
              },
              {
                "Type": "boolean",
                "Value": true
              }
            ]
          }
        ]
      }
    ]
  }
}
```

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

Before accepting the request, the server will parse, compile, and install the policy module. If any of these operations fail, all changes will be rolled back and the server will return 400. The error message that accompanies the response should indicate why the request failed.

### Delete a Policy

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
HTTP/1.1 204 No Content
```

#### Status Codes

- **204** - no content
- **400** - bad request
- **404** - not found
- **500** - server error

If other policy modules in the same package depend on rules in the module to be deleted, the server will return 400.

## Data API

The Data API exposes endpoints for reading and writing documents in OPA. For an introduction to the different types of documents in OPA see [How Does OPA Work?](../../how-does-opa-work/).

### Get a Document

```
GET /v1/data/{path:.+}
```

Get a document.

The path separator is used to access an object value by key or an array element by index. If the path accesses an array, OPA attempts to treat the path element as an integer.

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
[
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
```

#### Query Parameters

- **global** - Provide an input document to the query. Format is `<path>:<value>` where `<path>` is the import path of the input document and `<value>` is the JSON serialized input document. The parameter may be specified multiple times but each instance should contain a unique `<path>`.
- **pretty** - Return data with indented, human-readable formatting.

#### Status Codes

- **200** - no error
- **404** - not found
- **500** - server error

The server returns 404 in two cases:

1. The path refers to a non-existent document.
2. The path refers to a Virtual Document that is undefined at the time of the query.

In the second case, the response body will contain an object indicating the document is undefined.

#### Example Module

```ruby
package opa.examples

import example.flag

allow_request :- flag = true
```

#### Example Request with Global Parameter

```http
GET /v1/data/opa/examples/allow_request?global=example.flag:false HTTP/1.1
```

#### Example Response for Undefined Virtual Document

```http
HTTP/1.1 404 Not Found
Content-Type: application/json
```

```json
{
  "IsUndefined": true
}
```

### Patch a Document

```
PATCH /v1/data/{path:.+}
Content-Type: application/json-patch+json
```

Update a document.

The path separator is used to access an object value by key or an array element by index. If the path accesses an array, OPA attempts to treat the path element as an integer.

OPA accepts updates encoded as JSON Patch operations. The message body of the request should contain a JSON encoded array containing one or more JSON Patch operations. Each operation specifies the operation type, path, and an optional value. For more information on JSON Patch, see [RFC 6902](https://tools.ietf.org/html/rfc6902).

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

- **200** - no content
- **404** - not found
- **500** - server error

The effective path of the JSON Patch operation is obtained by joining the path portion of the URL with the path value from the operation(s) contained in the message body. In all cases, the parent of the effective path MUST refer to an existing document, otherwise the server returns 404. In the case of **remove** and **replace** operations, the effective path MUST refer to an existing document, otherwise the server returns 404.

## Query API

### Execute a Query

```
GET /v1/query
```

Execute an ad-hoc query and returns bindings for variables found in the query.

#### Example Request

```
GET /v1/query?q=data.servers[i].ports[_] = "p2", data.servers[i].name = name HTTP/1.1
```

#### Example Response

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
[
  {
    "i": 3,
    "name": "dev"
  },
  {
    "i": 0,
    "name": "app"
  }
]
```

#### Query Parameters

- **q** - The ad-hoc query to execute. OPA will parse, compile, and execute the query represented by the parameter value. The value MUST be URL encoded.
- **pretty** - Return data with indented, human-readable formatting.

#### Status Codes

- **200** - no error
- **400** - bad request
- **500** - server error

## Errors

All of the API endpoints use standard HTTP error codes to indicate success or failure of an API call. If an API call fails, the response will contain a JSON encoded object that provides more detail:

```
{
  "Code": 404,
  "Message": "storage error (code: 1): module not found: test"
}
```

{% endcontentfor %}