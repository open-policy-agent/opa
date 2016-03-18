# Concepts

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

## Requirements

Before diving into the concepts behind OPA, it is useful to review the requirements placed on a policy engine to successfully policy enable applications.

1. **The policy definitions must be decoupled from the application's implementation.**

    Policy authors must be empowered to define and manage policy without requiring changes to the application which necessitate costly development, testing, and deployment cycles.

1. **The policy definitions must be accessible to administrators and other developers.**

    Policy must be readily understood by administrators and other developers. In some cases, policy definitions also need to be understood by non-technical team members. Furthermore, administrators and other developers must be empowered to safely modify policy.

1. **The engine must be highly available.**

    Applications must be able to treat policy engine downtime as an extremely rare event (which likely requires manual intervention). The decision about what to do when the policy engine is unavailable is itself a policy question. It would be counter productive to the goals of policy enablement to re-implement policy decisions in the application to deal with policy engine downtime.

1. **The engine must introduce minimal latency when evaluating policy.**

	Applications must be able to assume that the performance cost of decoupling the policy implementation from the application is negligible. If the performance cost of doing so is too high, applications will not use the engine.

1. **The engine must integrate with applications written in any programming language.**

    Applications must not have to be developed using a specific programming language in order to benefit from the engine. If the cost of integrating with the engine involves developing the application in a specific programming language, far fewer applications will be able to use the engine.

1. **The engine must easily ingest the state of world that is relevant to policy.**

    Applications must be able to leverage their respective programming language ecosystems to push data into the policy engine. Furthermore, the process for reading and writing data stored in the policy engine must be as simple as possible.

These core requirements capture the guiding principles behind the concepts and design of OPA.

## Overview

Conceptually, OPA is a collection of algorithms that answer questions about policy.
<img src="https://cdn.rawgit.com/open-policy-agent/opa/9f5f1e6fa68fd0ee627122b9e5c8809519e5bba8/docs/overview.svg" />

At a high level, using OPA involves:

- Deploying OPA on servers alongside applications.
- Writing policies a declarative language ("Opalog") and loading them into OPA.
- Pushing application state that is relevant to policy into OPA.
- Executing queries via OPA's APIs to answer questions about operations governed by policy.
- Reacting to notifications delivered by OPA when policy is violated.

## Deployment

In order to support arbitrary languages, frameworks and runtimes, OPA is deployed as a host-local daemon ("opad") alongside applications. OPA can be deployed directly on hosts as an operating system daemon or in a container.

Deploying OPA as a host-local daemon helps ensure low latency transactions and high availability (through fate sharing) of the policy engine. If OPA is deployed on a separate server, significant consideration must be given to ensure that hardware failures and network partitions do not result in downtime of the policy engine.

When OPA is started for the first time it contains no policies or data. Applications or deployment automation may load policies into OPA at deploy time and update policies during upgrades. Administrators can manually add, remove, and change policies as needed.

Applications interact with OPA via HTTP.

<img src="https://cdn.rawgit.com/open-policy-agent/opa/9f5f1e6fa68fd0ee627122b9e5c8809519e5bba8/docs/deployment.svg" />

## APIs

OPA exposes its APIs over HTTP and uses JSON as the default interchange format. The APIs are RESTful with the exception of certain streaming operations. HTTP and JSON are used because they are well supported in most application development languages and provide a low barrier to integration.

### Policy API

OPA's Policy API exposes CRUD operations on policies within OPA. Applications and administrators can use the Policy API to manage policies.

For example, the request below creates a policy named "example-policy":

```
PUT /v1/policies/example-policy
Content-Type: text/plain

package opa.examples

violations[] = server :-
	server = data.servers[]
	server.protocols[] = "http"
	server.ports[] = data.ports[i].id
	data.ports[i].networks[] = data.networks[j].id
	data.networks[j].public = true
```

Policy names are only used to identify policies for management purposes. Policy names are not used in policy definitions or when issuing requests against the Data API.

### Data API

In order to evaluate policy and compute violations, OPA needs to have access to the state of the world which is relevant to policy. OPA's Data API deals with data from applications or the environment that is relevant to policy.

Applications use the Data API to push state relevant to policy into OPA. Applications are responsible for updating the relevant data stored in OPA when state changes.

For example, an application can push a list of servers into OPA by issuing an HTTP PATCH on /v1/data/servers:

```
PATCH /v1/data/servers HTTP/1.1
Content-Type: application/json-patch+json

[
	{
	  "op": "add",
	  "path": "-",
	  "value": {"id": "s1", "name": "app", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2", "p3"]}
	},
	{
	  "op": "add",
	  "path": "-",
	  "value": {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]}
	},
	{
	  "op": "add",
	  "path": "-",
	  "value": {"id": "s3", "name": "cache", "protocols": ["memcache"], "ports": ["p3"]}
	},
	{
	  "op": "add",
	  "path": "-",
	  "value": {"id": "s4", "name": "dev", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2"]}
	}
]
```

Conceptually, all data stored in OPA exists within a single document ("data").

Applications execute queries when handling operations that are governed by policy. The query response tells the application what to do with the operation, e.g, reject the API call or place the VM on a specific host. Queries are executed via the Data API by performing a GET on the appropriate path. See the [Policies](#policies) section for examples.

Queries can also be performed against data pushed into OPA. For example, the request below queries for the "servers" added above.

```
GET /v1/data/servers HTTP/1.1
```

The response:

```
HTTP/1.1 200 OK
Content-Type: application/json

[
  {"id": "s1", "name": "app", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2", "p3"]},
  {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]},
  {"id": "s3", "name": "cache", "protocols": ["memcache"], "ports": ["p3"]},
  {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
]
```

### Triggers

Applications can register for asynchronous notifications when policy is violated. This allows applications to take corrective action when the system exits the expected state.

For example, to receive notifications when services should be migrated to a new host, the application responsible for service deployment would create a rule which asserts that running services must be placed on a host which is not in the "terminated" state. If a host was shutdown unexpectedly, the system would notify OPA and the host state (stored in OPA) would be updated. OPA will detect that a rule has been violated and deliver a notification to the service deployment application (which can handle the event by redeploying the service on a running host.)

### Transactions

OPA exposes transactional APIs to operate on consistent snapshots of data stored in OPA. For example, when deploying a new VM, the application would open a transaction and perform the following steps:

1. Query OPA for a list of hosts where the VM can be deployed.
2. Choose a host from the list and deploy the VM.
3. Push new data to OPA to reflect that the VM has been deployed.
4. Close the transaction.

Lastly, policy writers can use OPA's APIs to obtain detailed explanations of query processing to understand why queries are returning specific results. This is useful for debugging queries.

## Data Model

OPA is designed to support document-oriented models such as JSON. Documents consist of scalars (i.e, booleans, strings, numbers, and null) and collections (i.e, objects, arrays, and sets). The document model was selected for OPA because of its prevalence in modern application stacks. For example, most applications today expose APIs which produce and consume JSON and many modern applications rely on document-oriented databases or document support in existing relational databases.

Conceptually, there are two kinds of documents in OPA:

- **Base documents** representing application state relevant to policy.
- **Virtual documents** representing higher level information defined by policies.

**Base documents** represent the raw information relevant to policy that is pushed by applications integrated with OPA. Base documents are stored in-memory and can be written to disk for resilience.

**Virtual documents** are defined by rules inside policies. OPA computes the contents of virtual documents when callers execute queries against rules or when dependant documents are modified. Virtual documents are defined in terms of base documents and other virtual documents. Policy authors can use virtual documents to define abstractions which are useful for expressing high level policy and insulating policy from schema changes in low level data.

<img src="https://cdn.rawgit.com/open-policy-agent/opa/9f5f1e6fa68fd0ee627122b9e5c8809519e5bba8/docs/data-model-dependencies.svg" />

When defining policies, rules are written which contain expressions that reference documents. The language that rules are written in ("Opalog") lets you reference base documents and virtual documents in exactly the same way.

<img src="https://cdn.rawgit.com/open-policy-agent/opa/9f5f1e6fa68fd0ee627122b9e5c8809519e5bba8/docs/data-model-logical.svg" />

## <a name="policies"></a> Policies

Policies are defined in OPA's native query language: Opalog.

Opalog is a declarative language based on [Datalog](https://en.wikipedia.org/wiki/Datalog). Opalog allows policy writers to define modules which contain rules. Rules contain expressions which assert facts about the expected state of documents stored in OPA. The documents referenced in rules may be base documents pushed by applications integrated with OPA or virtual documents defined by other rules.

To support document-oriented models such as JSON, Opalog has rich support for referencing nested documents (i.e, documents inside arrays or objects). The syntax for referencing nested documents is based on dictionary and array access in languages like Python as well as JSON Path.

Let's look at an example.

Assume we have the following data stored in OPA:

```
GET /v1/data HTTP/1.1
```

The response:

```
HTTP/1.1 200 OK
Content-Type: application/json

{
  "servers": [
    {"id": "s1", "name": "app", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2", "p3"]},
    {"id": "s2", "name": "db", "protocols": ["mysql"], "ports": ["p3"]},
    {"id": "s3", "name": "cache", "protocols": ["memcache", "http"], "ports": ["p3"]},
    {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p1", "p2"]}
  ],
  "networks": [
    {"id": "n1", "public": false},
    {"id": "n2", "public": false},
    {"id": "n3", "public": true}
  ],
  "ports": [
    {"id": "p1", "networks": ["n1"]},
    {"id": "p2", "networks": ["n3"]},
    {"id": "p3", "networks": ["n2"]}
  ]
}
```

We can write a rule which enumerates servers that expose HTTP (but not HTTPS) and are connected to public networks. These represent violations of policy.

```opalog
package opa.examples                            # this policy belongs the opa.examples package

import data.servers                             # import the data.servers document to refer to it as "servers" instead of "data.servers"
import data.networks                            # same but for data.networks
import data.ports                               # same but for data.ports

violations[] = server :-                        # a server exists in the violations set if:
	server = servers[]                          # the server exists in the servers collection
	server.protocols[] = "http"                 # and the server has http in its protocols collection
	public_servers[] = server                   # and the server exists in the public_servers collection

public_servers[] = server :-                    # a server exists in the public_servers set if:
	server = servers[]                          # the server exists in the servers collection
	server.ports[] = ports[i].id                # and the server is connected to a port in the ports collection
	ports[i].networks[] = networks[j].id        # and the port is connected to a network in the networks collection
	networks[j].public = true                   # and the network is public
```

The key aspects of Opalog are illustrated by this example:

- Rules define the content of virtual documents. In this case, we create two virtual documents: `violations` and `public_servers`.

- Rules consist of assertions against data stored in OPA. In this case the assertions are expressions which test for equality and membership of `servers`, `networks`, and `ports` documents.

- Expressions can referenced nested documents, e.g, `ports[i].networks[]` references the network IDs stored in an array on each port document.

- Expressions can reference elements in a collection using the `[]` and `[<variable>]` syntax. When this is done, OPA knows to iterate over elements of the collection when processing queries.

If we query for the document produced by this rule, we receive an array of servers that expose an HTTP server and are connected to a public network:

```
GET /v1/data/opa/examples/violations HTTP/1.1
```

The response:

```
HTTP/1.1 200 OK
Content-Type: application/json

[
  {"id": "s1", "name": "app-server", "protocols": ["http", "https", "ssh"], "ports": ["p1", "p2", "p3"]},
  {"id": "s4", "name": "dev", "protocols": ["http"], "ports": ["p2"]}
]
```

<!--
## What's Next

// Once at least one of these exist we can link to them. For now this section is hidden.

For more information on how to write policy definitions and queries, see [Opalog: OPA's Query Language](./LANGUAGE.md).

For more information on the architecture of OPA, see [OPA's Architecture](./ARCHITECTURE.md).
-->
