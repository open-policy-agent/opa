# OPA Concepts

OPA provides an open source policy engine that can be used to policy enable applications. Policy enabling applications decouples the policy that governs how applications behave from the implementation of those applications. This allows administrators to create and manage policy without requiring changes to the application. This eases some of the challenges of maintaining applications in complex modern deployment environments.

Because policies are defined declaratively and outside the application, administrators and other developers can more readily understand the policies that govern how the application must behave and are empowered to safely and efficiently make changes to policy.

OPA is purpose built for modern deployment environments and can be integrated with various types of applications to provide rich policy control over various functions such as:

- API authorization
- VM and container placement
- Auto-scaling and auto-healing clusters
- CI/CD pipeline gating
- Network ACLs
- Feature flags

This document describes the main concepts in OPA. After reading this document you should have a better idea of how OPA is deployed and interacted with by applications and users.

## Overview

<overview.svg>

Conceptually, OPA is a collection of algorithms that answer questions about policy. At a high level, using OPA involves:

- Writing policies a declarative language ("Opalog") and loading them into OPA.
- Pushing internal application state that is relevant ot policy into OPA.
- Executing queries via OPA's APIs to answer questions about operations governed by policy.
- Reacting to notifications delivered byt OPA when policy is violated.

## Deployment

<deployment.svg>

In order to support arbitrary langagues, runtimes, and frameworks, OPA is deployed as a host-local daemon alongside applications. OPA supports deployment directly on hosts as an operating system daemon or it can be deployed in a container.

When OPA is started for the first time it contains no policies or data. Applications or deployment automation typically include pre-defined policies that are loaded into OPA at deploy time and updated during upgrades. Administrators can manually add, remove, and change policies as needed however this is typically an exception.

## APIs

<api-interations.svg>

Data is pushed into OPA by applications via HTTP APIs. The data pushed into OPA is the internal application state relevant to policy. The application is responsible for updating the data stored in OPA when the state changes.

Applications execute queries when handling operations that are governed by policy. The answer to the query tells the application what to do with the operation, e.g, reject the API call or place the VM on a specific host.

Applications can register for asynchronous notifications when policy is violated. This allows applications to take corrective action when the system exits the expected state. For example, when one of the hosts in a deployment is shutdown unexpectedly, the state of the host is updated to "terminated". Assuming some rule asserts that a service must be deployed on a running host at all times, OPA will detect this violation and deliver a notification to the application responsible for deployment (which can handle the event by redeploying the service.)

OPA exposes transactional APIs to operate on consistent snapshots of data stored in OPA. For example, when deploying a new VM, the application would open a transaction and perform the following steps: 

1. Query OPA for a list of hosts where the VM can be deployed.
2. Choose a host from the list and deploy the VM.
3. Push new data to OPA to reflect that the VM has been deployed.
4. Close the transaction.

Lastly, policy writers can use OPA's APIs to obtain detailed explanations of query processing to understand why queries are returning specific results. This is useful for debugging queries.

## Data Model

<data-model.svg>

OPA is designed to support document-oriented models such as JSON. Documents consist of scalars (i.e, booleans, strings, numbers, and null) and collections (i.e, arrays and sets). Conceptually, there are two kinds of documents in OPA:

- **Base documents** representing application state relevant to policy.
- **Virtual documents** representing higher level information defined by policies.

**Base documents** represent the raw information relevant to policy that is pushed by applications integrated with OPA. Base documents are stored in-memory and can be written to disk for resilience.

**Virtual documents** are defined by rules inside policies. OPA computes the contents of virtual documents when callers execute queries against rules or when dependant documents are modified. Virtual documents are defined in terms of base documents and other virtual documents. Policy offers can use virtual documents to define abstractions which are useful for expressing high level policy and insulating policy from schema changes in low level data.

When defining policies, rules are written which contain expressions that reference documents. The language which rules are written in ("Opalog") does not distinguish between base and virtual documents inside expressions.

## Policies

Policies are defined in OPA's native query language: Opalog.

Opalog is a declarative language based on [Datalog](https://en.wikipedia.org/wiki/Datalog). Opalog allows policy writers to define modules which contain rules. Rules contain expressions which assert facts about the expected state of documents stored in OPA. The documents referenced in rules may be base documents pushed applications integrated with OPA or virtual documents defined by other rules.

To support document-oriented models such as JSON, Opalog has rich support for referencing nested documents (i.e, documents inside arrays or objects). The syntax for referencing nested documents is based on dictionary and array access in languages like Python as well as JSON Path.

Let's look at an example.

Assume we have the following data stored in OPA:

```
{
  "servers": [
    {"id": "s1", "name": "app", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2", "p3"]},
    {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]},
    {"id": "s3", "name": "cache", "protocols": ["memcache"], "ports": ["p3"]},
    {"id": "s4", "name": "dev", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2"]}
  ],
  "networks": [
    {"id": "n1", "public": false},
    {"id": "n2", "public": false},
    {"id": "n3", "public": true} 
  ],
  "ports": [
    {"id": "p1", "networks": ["n1"]},
    {"id": "p2", "networks": ["n3"]},
    {"id": "p3", "networks": ["n2"]},
  ]
}
```

We can write a rule which enumerates servers that expose HTTP (but not HTTPS) and are connected to public networks. These represent violations of policy.

```
violations[] = server :-
	server = db.servers[]
	server.protocols[] = "http"
	connected_to_internet[] = server

connected_to_public[] = server :-
	server = db.servers[]
	server.ports[] = db.ports[i].id
	db.ports[i].networks[] = db.networks[j].id
	db.networks[j].public = true
```

The key aspects of Opalog are illustrated by this example:

- Rules define the content of virtual documents. In this case, we create two virtual documents: `violations` and `connected_to_public`.

- Rules consist of assertions against data stored in OPA. In this case the asertions are expressions which test for equality and membership of `servers`, `networks`, and `ports` documents.

- Expressions can referenced nested documents, e.g, `ports[i].networks[]` references the network IDs stored in an array on each port document.

- Expressions can reference elements in a collection using the `[]` and `[<variable>]` syntax. When this is done, OPA knows to iterate over elements of the collection when processing queries.

If we query this rule, we receive an array of servers that expose an HTTP server and are connected to a public network:

```
GET /query?rule=violations
```

The response:

```
HTTP/1.1 200 OK
Content-Type: application/json

[
  {"id": "s1", "name": "app-server", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2", "p3"]},
  {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p2"]},
]
```

<!--
## What's Next

// Once at least one of these exist we can link to them. For now this section is hidden.

For more information on how to write policy definitions and queries, see [Opalog: OPA's Query Language](./LANGUAGE.md).

For more information on the architecture of OPA, see [OPA's Architecture](./ARCHITECTURE.md).
-->

<!--
graveyard
-->