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

{{< figure src="benefits.svg" width="65" caption="Open Policy Agent: before and after" >}}

OPA is a full-featured policy engine that offloads policy decisions from your
service. You can think of it as a concierge for your service who can answer
detailed questions on behalf of your users to meet their specific needs.