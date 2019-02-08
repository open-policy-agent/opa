---
title: How Does OPA Work?
kind: documentation
weight: 1
---

OPA is a full-featured policy engine that offloads policy decisions from your service. You can think of it as a concierge for your service who can answer detailed questions on behalf of your users to meet their specific needs.

## Overview

OPA’s RESTful APIs use JSON over HTTP so you and your users can integrate OPA with any programming language. At a high level, integrating OPA into your service involves:

  * Deploying OPA alongside your service
  * Pushing relevant data about your service’s state into OPA’s document store
  * Offloading some or all decision-making to OPA by querying it

When your service is integrated with OPA, your users will be able author and deploy custom policies that control the behavior of your service’s policy-enabled features. Furthermore, users can publish data to OPA that is not available to your service about their own deployment context.

{{< figure src="/img/request-response.svg" width="50" caption="OPA's query and decision model" >}}

In the future, both your service and its users will be able to register for, and react to, notifications triggered when OPA detects a policy-relevant change.

## Deployment

Unless you embed OPA as a Go library, you will deploy it alongside your service – either directly as an operating system daemon or inside a container. In this way, transactions will have low latency and availability will be determined through shared fate with your service.

When OPA starts for the first time, it will not contain any policies or data. Policies and data can be added, removed, and modified at any time. For example: by deployment automation software or your service as it is deployed, by your service during an upgrade, or by administrators as needed.

## Data and Policies

The primary unit of data in OPA is a document, which is similar to a JSON value. Documents typically correspond to single, self-contained objects and are capable of representing both primitive types (strings, numbers, booleans, and null) as well as structured types (objects, and arrays). Documents are created, read, updated, and deleted via OPA’s [RESTful HTTP APIs](../rest-api).

{{< figure src="/img/data-model-dependencies.svg" width="70" caption="OPA data model dependencies" >}}

### Base Documents

So-called base documents contain static, structured data stored in memory and optionally saved to disk for resiliency. Your service will publish and update base documents in order to describe its current state, and your users can do the same to include relevant data about the state of their own deployment context.

Base documents are published and updated using OPA’s Data API. For example, the following request publishes a list of servers to OPA:

```http
PATCH https://example.com/v1/data/servers HTTP/1.1
Content-Type: application/json-patch+json
```

```json
[
  {
    "op": "add",
    "path": "-",
    "value": {
      "id": "s1",
      "name": "app",
      "protocols": [
        "http",
        "https",
        "ssh"
      ],
      "ports": [
        "p1",
        "p2",
        "p3"
      ]
    }
  },
  {
    "op": "add",
    "path": "-",
    "value": {
      "id": "s2",
      "name": "db",
      "protocols": [
        "mysql"
      ],
      "ports": [
        "p3"
      ]
    }
  },
  {
    "op": "add",
    "path": "-",
    "value": {
      "id": "s3",
      "name": "cache",
      "protocols": [
        "memcache"
      ],
      "ports": [
        "p3"
      ]
    }
  },
  {
    "op": "add",
    "path": "-",
    "value": {
      "id": "s4",
      "name": "dev",
      "protocols": [
        "http",
        "https",
        "ssh"
      ],
      "ports": [
        "p1",
        "p2"
      ]
    }
  }
]
```

### Policies

Policies are written using OPA’s purpose-built, declarative language Rego. Rego includes rich support for traversing nested documents and transforming data using syntax inspired by dictionary and array access in languages like Python and JSONPath. For detailed information about using Rego, see [How Do I Write Policies?](/how-do-i-write-policies.md).

Each Rego file defines a policy module using a collection of rules that describe the expected state of your service. Both your service and its users can publish and update policy modules using OPA’s Policy API. For example, the following request creates a policy with two rules (violations and public_servers) named “exempli-gratia”:

```http
PUT https://example.com/v1/policies/exempli-gratia HTTP/1.1
Content-Type: text/plain
```

```ruby
package opa.examples

import data.servers
import data.networks
import data.ports

violations[server] {
    server := servers[_]
    server.protocols[_] == "http"
    public_servers[server]
}

public_servers[server] {
    server := servers[_]
    server.ports[_] == ports[i].id
    ports[i].networks[_] == networks[j].id
    networks[j].public == true
}
```

A policy file must contain a single package declaration, which defines the path to the policy module and its rules (for example, data.opa.examples.violations – see The data Document for more information about accessing nested documents). The policy name itself (in this case, “exempli-gratia”) is only used to identify policies for file management purposes; it is not used otherwise.

### Rules and Virtual Documents

In contrast to base documents, virtual documents embody the results of evaluating the rules included in policy modules. Virtual documents are computed when users publish new policy modules, update existing modules, run queries, and when any relevant base document is published or updated. Rules allow policy authors to write questions with yes-no answers (that is, predicates) and to generate structured values from raw data found in base documents as well as from intermediate data found in other virtual documents.

### The `data` Document

All documents pushed into OPA or computed by rules are nested under a built-in root document named data.

{{< figure src="/img/data-model-logical.svg" width="70" caption="OPA document structure" >}}

Example `data` document:

```json
{
  "servers": [...],
  "ports": [...],
  "networks": [...],
  "opa": {
    "examples": {
      "violations": [...],
      "public_servers": [...]
    }
  }
}
```

As a result, any document, base or virtual, can be accessed hierarchically starting from the root data node – either as an identifier:

```ruby
import data.servers                            # Base document
import data.opa.examples.violations            # Virtual document
```

or as a URI component in an HTTP request:

```http
GET https://example.com/v1/data/servers HTTP/1.1
```

```http
GET https://example.com/v1/data/opa/examples/violations HTTP/1.1
```

> Since the `data` document includes both base and virtual documents,
> it is possible to query for both at the same time. The easiest way
> to illustrate this is to query for _all_ of `data` at once. Note,
> OPA does NOT allow base and virtual documents to overlap. For
> example, if you try to load a rule that defines a virtual document
> at path a/b/c (which is already defined by a base document), OPA
> will return an error. Similarly, if you try to load a base document
> into a path that is already defined by a virtual document, OPA will
> also return an error.

### The `input` Document

In some cases, policies require input values. In addition to the built-in
`data` document, OPA also has a built-in `input` document. When you query
OPA, you can set the value of the `input` document.

Example `input` document:

```json
{
  "method": "GET",
  "path": "/servers/s2",
  "user": "alice"
}
```

The `input` document can be referenced just like the `data` document.

```ruby
# Let 'bob' perform read-only operations.
allow {
  input.user == "bob"
  input.method == "GET"
}

# Let 'alice' perform any operation.
allow {
  input.user == "alice"
}
```

## Putting It All Together

Let’s take a look at some documents representing the state of a hypothetical service and a policy module that uses this data. The following documents describe a set of servers, the protocols they use, the ports they open, and the networks those ports are connected to.

Example `data` document:

```json
{
  "servers": [
    {
      "id": "s1",
      "name": "app",
      "protocols": [
        "https",
        "ssh"
      ],
      "ports": [
        "p1",
        "p2",
        "p3"
      ]
    },
    {
      "id": "s2",
      "name": "db",
      "protocols": [
        "mysql"
      ],
      "ports": [
        "p3"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "protocols": [
        "memcache",
        "http"
      ],
      "ports": [
        "p3"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "protocols": [
        "http"
      ],
      "ports": [
        "p1",
        "p2"
      ]
    }
  ],
  "networks": [
    {
      "id": "n1",
      "public": false
    },
    {
      "id": "n2",
      "public": false
    },
    {
      "id": "n3",
      "public": true
    }
  ],
  "ports": [
    {
      "id": "p1",
      "networks": [
        "n1"
      ]
    },
    {
      "id": "p2",
      "networks": [
        "n3"
      ]
    },
    {
      "id": "p3",
      "networks": [
        "n2"
      ]
    }
  ]
}
```

When the data is published, we can use OPA’s API to inspect base documents like servers:

```http
GET https://example.com/v1/data/servers HTTP/1.1
```

The response is an object that contains the array of servers:

```json
{
  "result": [
    {
      "id": "s1",
      "name": "app",
      "protocols": [
        "https",
        "ssh"
      ],
      "ports": [
        "p1",
        "p2",
        "p3"
      ]
    },
    {
      "id": "s2",
      "name": "db",
      "protocols": [
        "mysql"
      ],
      "ports": [
        "p3"
      ]
    },
    {
      "id": "s3",
      "name": "cache",
      "protocols": [
        "memcache",
        "http"
      ],
      "ports": [
        "p3"
      ]
    },
    {
      "id": "s4",
      "name": "dev",
      "protocols": [
        "http"
      ],
      "ports": [
        "p1",
        "p2"
      ]
    }
  ]
}
```

Now let’s write a policy that enumerates servers that are connected to public networks and that are using HTTP. These servers are violating a business rule that states that all public servers must use HTTPS.

```ruby
# This policy module belongs to the opa.examples package.
package opa.examples

# Refer to data.servers as `servers`.
import data.servers
# Refer to the data.networks as `networks`.
import data.networks
# Refer to the data.ports as `ports`.
import data.ports

# A server exists in the violations set if...
violations[server] {
    # ...the server exists
    server := servers[_]
    # ...and any of the server’s protocols is HTTP
    server.protocols[_] == "http"
    # ...and the server is public.
    public_servers[server]
}

# A server exists in the public_servers set if...
public_servers[server] {
    # ...the server exists
    server := servers[_]
    # ...and the server is connected to a port
    server.ports[_] == ports[i].id
    # ...and the port is connected to a network
    ports[i].networks[_] == networks[j].id
    # ...and the network is public.
    networks[j].public == true
}
```

Note that:

  * Rules consist of assertions about data stored in OPA. In this case, the assertions test for equality with, and membership of, values in the servers, networks, and ports documents.
  * Expressions can reference elements in a collection using the `[_]` and `[<variable>]` syntax. OPA knows to evaluate such queries by iterating over each element in the corresponding collection.
  * Assertions about elements in a collection are `true` if any of the elements match the expression, and are only `false` when none of the elements match. For example, `ports[i].networks[_] == networks[j].id` will be `true` whenever any element in `ports[i].networks` matches the id of any element in `networks`.
  * Expressions can reference nested documents. For example, `ports[i].networks[_]` refers to each network ID listed in each port document.
  * Expressions can reference virtual documents. For example, `public_servers[server] == true` matches only if `server` is in the list produced by the `public_servers` rule.


After publishing this policy module, the `data` document will include
additional documents corresponding to the module’s package declaration
(opa.examples) and the virtual documents its rules generate.

```http
GET https://example.com/v1/data HTTP/1.1
```

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": {
    "servers": [...],
    "networks": [...],
    "ports": [...],
    "opa": {
      "examples": {
        "violations": [
          {
            "id": "s4",
            "name": "dev",
            "protocols": [
              "http"
            ],
            "ports": [
              "p1",
              "p2"
            ]
          }
        ],
        "public_servers": [
          {
            "id": "s1",
            "name": "app",
            "protocols": [
              "https",
              "ssh"
            ],
            "ports": [
              "p1",
              "p2",
              "p3"
            ]
          },
          {
            "id": "s4",
            "name": "dev",
            "protocols": [
              "http"
            ],
            "ports": [
              "p1",
              "p2"
            ]
          }
        ]
      }
    }
  }
}
```

In this case, we are only interested in the set of servers that violate the
policy. We can use OPA's API to query for just those servers.

```http
GET https://example.com/v1/data/opa/examples/violations HTTP/1.1
```

...the response is the subset of the servers base document that use HTTP and are connected to a public network:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "result": [
    {
      "id": "s4",
      "name": "dev",
      "protocols": [
        "http"
      ],
      "ports": [
        "p1",
        "p2"
      ]
    }
  ]
}
```
