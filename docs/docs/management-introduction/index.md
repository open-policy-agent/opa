---
title: "Overview & Architecture"
---

OPA exposes a set of APIs that enable unified, logically centralized policy
management. Read this page if you are interested in how to build a control plane
around OPA that enables policy distribution and collection of important
telemetry data like decision logs.

OPA enables low-latency, highly-available policy enforcement by providing a
lightweight engine for distributed architectures. By default, all of the policy
and data that OPA uses to make decisions is kept in-memory:

<!--- source: https://docs.google.com/drawings/d/1-dwGFRjv_nFydo-8tOK-C-PbWyjvRObYhePC7XaLUFw/edit?usp=sharing --->

![Host-local Architecture](integration.svg)

OPA is designed to enable _distributed_ policy enforcement. You can run OPA next
to each and every service that needs to offload policy decision-making. By
colocating OPA with the services that require decision-making, you ensure that
policy decisions are rendered as fast as possible and in a highly-available
manner.

<!--- source: https://docs.google.com/drawings/d/1wFef9_Smy0gNvJj4l8n05WCTqhmzdadiyspyRGFvHuw/edit?usp=sharing --->

![Distributed Policy Enforcement](distributed.svg)

To control and observe a set of OPAs, each OPA can be configured to connect to
management APIs that enable:

- Policy distribution ([Bundles](./management-bundles))
- Decision telemetry ([Decision Logs](./management-decision-logs))
- Agent telemetry ([Status](./management-status))
- Dynamic agent configuration ([Discovery](./management-discovery))

By configuring and implementing these management APIs you can unify control and
visibility over OPAs in your environments. OPA does not provide a control plane
service out-of-the-box today.

<!--- source: https://docs.google.com/drawings/d/1-08mHgUN5oy2phLJ6MOr7j3e0iguxg_X__3VH321iLc/edit?usp=sharing --->

![Control Plane](control-plane.svg)
