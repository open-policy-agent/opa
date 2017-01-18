---
nav_id: MAIN_DOCUMENTATION
doc_id: HOW_DOES_OPA_WORK
layout: documentation

title: How Does OPA Work?
---

{% contentfor header %}

# How Does OPA Work?

OPA is a full-featured policy engine that runs as a host-local daemon alongside your service. You can think of it as a concierge for your service who can answer detailed questions on behalf of your users to meet their specific needs.

{% endcontentfor %}

{% contentfor body %}

## Overview

OPA’s RESTful APIs use JSON over HTTP so you and your users can integrate OPA with any programming language. At a high level, integrating OPA into your service involves:

  * Deploying OPA alongside your service
  * Pushing relevant data about your service’s state into OPA’s document store
  * Offloading some or all decision-making to OPA by querying it

When your service is integrated with OPA, your users will be able author and deploy custom policies that control the behavior of your service’s policy-enabled features. Furthermore, users can publish data to OPA that is not available to your service about their own deployment context.

{% img '{{assets["request-response.svg"].logical_path}}' width:'320' class:'block-center' %}

In the future, both your service and its users will be able to register for, and react to, notifications triggered when OPA detects a policy-relevant change.

## Deployment

Unless you embed OPA as a Go library, you will deploy it alongside your service – either directly as an operating system daemon or inside a container. In this way, transactions will have low latency and availability will be determined through shared fate with your service.

When OPA starts for the first time, it will not contain any policies or data. Policies and data can be added, removed, and modified at any time. For example: by deployment automation software or your service as it is deployed, by your service during an upgrade, or by administrators as needed.

## Data and Policies

The primary unit of data in OPA is a document, which is similar to a JSON value. Documents typically correspond to single, self-contained objects and are capable of representing both primitive types (strings, numbers, booleans, and null) as well as structured types (objects, and arrays). Documents are created, read, updated, and deleted via OPA’s RESTful HTTP APIs.

{% img '{{assets["data-model-dependencies.svg"].logical_path}}' width:'720' %}

### <a name="base-documents"></a> Base Documents

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

Policies are written using OPA’s purpose-built, declarative language Rego. Rego includes rich support for traversing nested documents and transforming data using syntax inspired by dictionary and array access in languages like Python and JSONPath. For detailed information about using Rego, see [How Do I Write Policies?](/documentation/how-do-i-write-policies).

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

violations[server] :-
    server = servers[_],
    server.protocols[_] = "http",
    public_servers[server] = true

public_servers[server] :-
    server = servers[_],
    server.ports[_] = ports[i].id,
    ports[i].networks[_] = networks[j].id,
    networks[j].public = true
```

A policy file must contain a single package declaration, which defines the path to the policy module and its rules (for example, data.opa.examples.violations – see The data Document for more information about accessing nested documents). The policy name itself (in this case, “exempli-gratia”) is only used to identify policies for file management purposes; it is not used otherwise.

### <a name="virtual-documents"></a> Rules and Virtual Documents

In contrast to base documents, virtual documents embody the results of evaluating the rules included in policy modules. Virtual documents are computed when users publish new policy modules, update existing modules, run queries, and when any relevant base document is published or updated. Rules allow policy authors to write questions with yes-no answers (that is, predicates) and to generate structured values from raw data found in base documents as well as from intermediate data found in other virtual documents.

### The data Document

All documents pushed into OPA or computed by rules are nested under a built-in root document named data.

{% img '{{assets["data-model-logical.svg"].logical_path}}' width:'720' %}

```json
{
  "data": {
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

### The input Document

In some cases, to evaluate a policy, the query must specify additional documents
as arguments.

Query arguments are nested under a built-in root document named input (similar to data).

```json
{
  "input": {
      "method": "GET",
      "path": "/servers/s2",
      "user": "alice"
  }
}
```

Query arguments can be accessed hierarchically starting from the root input
node:

```ruby
allow :- input.user = "alice"
```

Just like state stored in OPA, documents supplied with the query can be aliased:

```ruby
package opa.examples

import input.method
import input.user

# allow "bob" to perform read-only operations
allow :- user = "bob", method = "GET"

# allow "alice" to perform any operation
allow :- user = "alice"
```

### Putting It All Together

Let’s take a look at some documents representing the state of a hypothetical service and a policy module that uses this data. The following documents describe a set of servers, the protocols they use, the ports they open, and the networks those ports are connected to.

```json
{
  "data": {
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
}
```

When the data is published, we can use OPA’s API to inspect base documents like servers:

```http
GET https://example.com/v1/data/servers HTTP/1.1
```

The response is an array of all servers:

```json
[
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
```

Now let’s write a policy that enumerates servers that are connected to public networks and that are using HTTP. These servers are violating a business rule that states that all public servers must use HTTPS.

```ruby
# This policy module belongs the opa.example package.
package opa.examples

# Refer to data.servers as `servers`.
import data.servers
# Refer to the data.networks as `networks`.
import data.networks
# Refer to the data.ports as `ports`.
import data.ports

# A server exists in the violations set if...
violations[server] :-
    # ...the server exists
    server = servers[_],
    # ...and any of the server’s protocols is HTTP
    server.protocols[_] = "http",
    # ...and the server is public.
    public_servers[server] = true

# A server exists in the public_servers set if...
public_servers[server] :-
    # ...the server exists
    server = servers[_],
    # ...and the server is connected to a port
    server.ports[_] = ports[i].id,
    # ...and the port is connected to a network
    ports[i].networks[_] = networks[j].id,
    # ...and the network is public.
    networks[j].public = true
```

Note that:

  * Rules consist of assertions about data stored in OPA. In this case, the assertions test for equality with, and membership of, values in the servers, networks, and ports documents.
  * Expressions can reference elements in a collection using the `[_]` and `[<variable>]` syntax. OPA knows to evaluate such queries by iterating over each element in the corresponding collection.
  * Assertions about elements in a collection are `true` if any of the elements match the expression, and are only `false` when none of the elements match. For example, `ports[i].networks[_] = networks[j].id` will be `true` whenever any element in `ports[i].networks` matches the id of any element in `networks`.
  * Expressions can reference nested documents. For `example,ports[i].networks[_]` refers to each network ID listed in each port document.
  * Expressions can reference virtual documents. For example, `public_servers[server] = true` matches only if `server` is in the list produced by the `public_servers` rule.

After publishing this policy module, data will include additional documents corresponding to the module’s package declaration (opa.examples) and the virtual documents its rules generate.

```json
{
  "data": {
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

If we use OPA’s API to inspect the violations virtual document…

```http
GET https://example.com/v1/data/opa/examples/violations HTTP/1.1
```

…the response is the subset of the servers base document that use HTTP and are connected to a public network:

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
[
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
```

## Future Features

OPA is under active development. The following features are planned but not yet implemented.

### Triggers

Your service and its users can register to be notified when the system exits the expected state so that violations can be remediated automatically.

Any rule can be used as the trigger for a notification. For example, let’s assume that you want to migrate containers to a new host if their current host shuts down. You need a rule to detect when there are containers assigned to hosts that are no longer running.

```ruby
# All production containers are running if...
containers_to_migrate[id] :-
    # ...the container exists
    container = containers[id],
    # ...and it is in production
    container.site.name = "prod",
    # ...and its host is running.
    container.host.state != "terminated"
```

This rule produces a list of containers that should be migrated. You can register to observe this rule, and when the underlying data changes, OPA will re-evaluate it and trigger a notification. You can handle the event by re-deploying the containers in the resulting list to a running host.

### Transactions

OPA APIs support transactional operations. Either all of the operations within a transaction succeed or the whole transaction fails. For example, to deploy a new virtual machine, you might:

  * Open a new transaction.
  * Query OPA for a list of hosts where the VM can be deployed.
  * Deploy the VM to host from the list.
  * Push data about the new deployment into OPA’s document store.
  * Close the transaction.

If any of steps 2–4 fail, so will the transaction.

{% endcontentfor %}
