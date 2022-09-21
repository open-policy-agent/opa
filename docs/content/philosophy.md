---
title: Philosophy
kind: documentation
weight: 1
---

A [**policy**](#policy) is a set of rules that governs the behavior of a
software service.  That policy could describe rate-limits, names of trusted
servers, the clusters an application should be deployed to, permitted network
routes, or accounts a user can withdraw money from.

Authorization is a special kind of policy that often dictates which people or
machines can run which actions on which resources. Authorization is sometimes
confused with Authentication: how people or machines prove they are who they say
they are.  Authorization and more generally policy often utilize the results of
authentication (the username, user attributes, groups, claims), but makes
decisions based on far more information than just who the user is.  Generalizing
away from authorization back to policy makes the distinction even clearer
because some policy decisions have nothing to do with users, e.g. policy simply
describes invariants that must hold in a software system (e.g. all binaries must
come from a trusted source).

Today policy is often a hard-coded feature of the software service it actually
governs.  Open Policy Agent lets you [**decouple policy**](#policy-decoupling)
from that software service so that the people responsible for policy can read,
write, analyze, version, distribute, and in general manage policy separate from
the service itself. OPA also gives you a unified toolset to decouple policy from
any software service you like, and to write context-aware policies using any
context that you like.  In short, OPA helps you decouple any policy using any
context from any software system.


## What is Policy? {#policy}

All organizations have policies. Policies are essential to the long-term success
of organizations because they encode important knowledge about how to comply
with legal requirements, work within technical constraints, avoid repeating
mistakes, and so on.

In their simplest form, policies can be applied manually based on rules that are
written down or conventions that are unspoken but permeate an organization’s
culture. Policies may also be enforced with application logic or statically
configured at deploy time.

## What is Policy Decoupling? {#policy-decoupling}

Software services should allow policies to be specified declaratively, updated
at any time without recompiling or redeploying, and enforced automatically
(which is especially valuable when decisions need to be made faster than humanly
possible).

Decoupling policy helps you build such software services at scale, makes them
adaptable to changing business requirements, improves the ability to discover
violations and conflicts, increases the consistency of policy compliance, and
mitigates the risk of human error.  The policies you write can adapt more easily
to the external environment--to factors that the developer could never have
imagined at the time the software service was designed.

For example, a cloud computing service could answer questions such as:

* Can I add compute capacity?
* In what regions can I add compute capacity?
* Which instances are currently running in the wrong region?

## What is OPA?

OPA is a lightweight general-purpose policy engine that can be co-located with
your service. You can integrate OPA as a sidecar, host-level daemon, or library.

Services offload policy decisions to OPA by executing *queries*. OPA evaluates
policies and data to produce query results (which are sent back to the client).
Policies are written in a high-level declarative language and can be loaded
dynamically into OPA remotely via APIs or through the local filesystem.

## Why use OPA?

OPA is a full-featured policy engine that offloads policy decisions from your
software. You can think of it as a concierge for your software who can answer
detailed questions on behalf of your users to meet their specific needs.
OPA provides the building blocks for enabling better control and visibility over
policy in your systems.

Without OPA, you need to implement policy management for your software from scratch.
Required components such as the policy language (syntax _and_ semantics) and the
evaluation engine need to be carefully designed, implemented, tested, documented,
and then maintained to ensure correct behaviour and a positive user experience
for your customers. On top of that you must carefully consider security, tooling,
management, and more. That's a lot of work.

## How Does OPA Work?

See the [Introduction](..) for an overview of how OPA works and how to get started.

## The OPA Document Model

OPA policies (written in Rego) make decisions based on hierarchical structured data.
Sometimes we refer to this data as a document, set of attributes, piece of context,
or even just "JSON" [1]. Importantly, OPA policies can make decisions based on _arbitrary_
structured data. OPA itself is not tied to any particular domain model. Similarly,
OPA policies can represent decisions as arbitrary structured data (e.g., booleans,
strings, maps, maps of lists of maps, etc.)

Data can be loaded into OPA from outside world using push or pull interfaces that operate
synchronously or asynchronously with respect to policy evaluation. We refer to all data
loaded into OPA from the outside world as **base documents** [2]. These base documents
almost always contribute to your policy decision-making logic. However, your policies can
also make decisions based on each other. Policies almost always consist of multiple rules
that refer to other rules (possibly authored by different groups). In OPA, we refer
to the values generated by rules (a.k.a., decisions) as **virtual documents**. The term
"virtual" in this case just means the document is _computed_ by the policy, i.e.,
it's not loaded into OPA from the outside world.

Base and virtual documents can represent the exact same kind of information, e.g., numbers,
strings, lists, maps, and so on. Moreover, with Rego, you can refer to both base and virtual
documents using the exact same dot/bracket-style reference syntax. Consistency across the
types of values that can be represented and the way those values are referenced means that
_policy authors only need to learn one way of modeling and referring to information
that drives policy decision-making_. Additionally, since there is no conceptual difference
in the types of values or the way you refer to those values in base and virtual documents,
Rego lets you refer to _both_ base and virtual documents through a global variable
called `data`. Similarly, OPA lets you query for both base and virtual documents via the
`/v1/data` HTTP API [3]. This is why queries for just `data` (or `data.foo` or `data.foo.bar`, etc.)
return the combination of base and virtual documents located under that path.

Since base documents come from outside of OPA, their location under `data` is controlled
by the software doing the loading. On the other hand, the location of virtual
documents under `data` is controlled by policies themselves using the `package` directive
in the language.

Base documents can be pushed or pulled into OPA _asynchronously_ by replicating data
into OPA when the state of the world changes. This can happen periodically or when some
event (like a database change notification) occurs. Base documents loaded asynchronously
are always accessed under the `data` global variable. On the other hand, base documents can
also be pushed or pulled into OPA _synchronously_ when your software queries OPA for policy
decisions. We refer to base documents pushed synchronously as "input". Policies can
access these inputs under the `input` global variable. To pull base documents during
policy evaluation, OPA exposes (and can be extended with custom) built-in functions like
`http.send`. Built-in function return values can be assigned to local variables and
surfaced in virtual documents. Data loaded synchronously is kept outside of `data` to
avoid naming conflicts.

The following table summarizes the different models for loading base documents into OPA,
how they can be referenced inside of policies, and the actual mechanism(s) for loading.

| Model | How to access in Rego | How to integrate with OPA |
| --- | --- | --- |
| Asynchronous Push | The `data` global variable | Invoke OPA's API(s), e.g., `PUT /v1/data` |
| Asynchronous Pull | The `data` global variable | Configure OPA's [Bundle](../management-bundles) feature |
| Synchronous Push | The `input` global variable | Provide data in policy query, e.g., inside the body of `POST /v1/data` |
| Synchronous Pull | The [built-in functions](../policy-reference), e.g., `http.send` | N/A |

Data loaded asynchronously into OPA is cached in-memory so that it can be read efficiently
during policy evaluation. Similarly, policies are also cached in-memory to ensure
high-performance and and high-availability. Data _pulled_ synchronously can also be
cached in-memory. For more information on loading external data into OPA, including tradeoffs,
see the [External Data](../external-data) page.

The following diagram illustrates the base and virtual document model described above for a
hypothetical policy that renders authorization decisions (named `data.acme.allow`) based on:

* API request information pushed synchronously located under `input`.
* Entitlements data pulled asynchronously and located under `data.entitlements`.
* Resource data pulled synchronously during policy evaluation using the `http.send` built-in function.

The entitlements and resource information is _abstracted_ by rules that generate
virtual documents named `data.iam.user_has_role` and `data.acme.user_is_assigned` respectively.

<!--- source: https://docs.google.com/drawings/d/1KerjlOGRmsZvs2tqfhLh2CGGkNRFH0GWioBsHLHAuIg/edit --->

{{< figure src="data-model.svg" width="65" caption="Hypothetical Policy Document Model" >}}

> [1] OPA has excellent support for loading JSON and YAML because they are prevalent
> in modern systems; however, OPA is not tied to any particular data format. OPA
> uses its own internal representation for structures like maps and lists (a.k.a.,
> objects and arrays in JSON.)

> [2] The term "document" comes from the document-oriented database world. Document
> is just a generic term to refer to data or information encoded in some standard
> format like JSON, YAML, XML, etc. Document-oriented data does not have to adhere
> to a strict schema like data in the relational world. Documents are often deeply
> nested, hierarchical data structures containing several levels of embedded
> maps and lists.

> [3] Internally, HTTP requests like `GET /v1/data` or `GET /v1/data/foo/bar` are turned
> into Rego queries that are almost identical to the HTTP path (e.g., `data` or `data.foo.bar`)
