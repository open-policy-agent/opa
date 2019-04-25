---
title: Introduction
kind: documentation
weight: 1
---

A [**policy**](#policy) is a set of rules that governs the behavior of a
service. [**Policy enablement**](#policy-enablement) empowers users to read,
write, and manage these rules without needing specialized development or
operational expertise. When your users can implement policies without
recompiling your source code, then your service is **policy enabled**.

## What is Policy? {#policy}

All organizations have policies. Policies are essential to the long-term
success of organizations because they encode important knowledge about how to
comply with legal requirements, work within technical constraints, avoid
repeating mistakes, and so on.

In their simplest form, policies can be applied manually based on rules that
are written down or conventions that are unspoken but permeate an
organization’s culture. Policies may also be enforced with application logic or
statically configured at deploy time.

## What is Policy Enablement? {#policy-enablement}

Policy-enabled services allow policies to be specified declaratively, updated
at any time without recompiling or redeploying, and enforced automatically
(which is especially valuable when decisions need to be made faster than
humanly possible). They make deployments more adaptable to changing business
requirements, improve the ability to discover violations and conflicts,
increase the consistency of policy compliance, and mitigate the risk of human
error.

A policy-enabled service is able to answer questions by comparing relevant
input from its environment to policy statements written by administrators. For
example, a cloud computing service could answer questions such as:

* Can I add compute capacity?
* In what regions can I add compute capacity?
* Which instances are currently running in the wrong region?

## What is OPA?

OPA is a lightweight general-purpose policy engine that can be co-located with
your service. You can integrate OPA as a sidecar, host-level daemon, or
library.

Services offload policy decisions to OPA by executing *queries*. OPA evaluates
policies and data to produce query results (which are sent back to the client).
Policies are written in a high-level declarative language and can be loaded
into OPA via the filesystem or well-defined APIs.

## Why use OPA?

{{< figure src="benefits.svg" width="65" caption="Open Policy Agent: before and after" >}}
