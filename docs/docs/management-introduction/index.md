---
title: "OPA Management APIs and Architecture"
sidebar_label: "Overview"
---

OPA exposes a set of APIs that enable unified, logically centralized policy
management. Read this page if you are interested in how to build a control plane
around OPA that enables policy distribution and collection of important
telemetry data like decision logs.

OPA enables low-latency, highly-available policy enforcement by providing a
lightweight engine for distributed architectures. By default, all of the policy
and data that OPA uses to make decisions is kept in-memory:

import HostLocalDiagram from './assets/HostLocalDiagram';

<HostLocalDiagram/>

OPA is designed to enable _distributed_ policy enforcement. You can run OPA next
to each and every service that needs to offload policy decision-making. By
co-locating OPA with the services that require decision-making, you ensure that
policy decisions are rendered as fast as possible and in a highly-available
manner.

import DistributedDiagram from './assets/DistributedDiagram';

<DistributedDiagram/>

To control and observe a set of OPAs, each OPA can be configured to connect to
management APIs that enable:

- Policy distribution ([Bundles](./management-bundles))
- Decision telemetry ([Decision Logs](./management-decision-logs))
- Agent telemetry ([Status](./management-status))
- Dynamic agent configuration ([Discovery](./management-discovery))

By configuring and implementing these management APIs you can unify control and
visibility over OPAs in your environments. OPA does not provide a control plane
service out-of-the-box.

import ControlPlaneDiagram from './assets/ControlPlaneDiagram';

<ControlPlaneDiagram/>
