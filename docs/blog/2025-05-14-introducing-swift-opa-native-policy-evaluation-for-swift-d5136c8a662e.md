---
title: "Introducing Swift OPA: Native Policy Evaluation for Swift"
authors: ["shomron"]
date: 2025-05-14
slug: introducing-swift-opa-native-policy-evaluation-for-swift-d5136c8a662e
---

Exciting news — Swift OPA, a new way to integrate OPA natively within Swift applications and services, has now been released.

![Introducing the Swift OPA project — native policy evaluation for Swift](/img/blog/introducing-swift-opa-native-policy-evaluation-for-swift-d5136c8a662e/1.png)

Swift OPA builds on the robust foundation of OPA's [Intermediate Representation](/blog/i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d) (IR) plans. Introduced in 2022, the OPA toolchain gained the ability to compile Rego policy into IR, precisely defining the concrete steps necessary to evaluate the policy. Swift OPA then interprets these IR plans, allowing you to leverage OPA's rich policy natively within Swift applications and services.

Until today, integrating OPA with a Swift application or service required one of the following approaches:

- Inter-process communication to a standalone OPA instance
- Evaluating Rego policy compiled into a Web Assembly (WASM) binary

Each of these approaches represent different ways of using OPA. For most of the OPA project's history, it's been used with a client/server architecture where communication takes place over the network. The embedded approach taken by WASM, on the other hand, avoids this step to enable in-process evaluation.

With its native language support, Swift OPA works more closely to the embedded approach described: evaluation occurs within the process boundary of your service or application. When developing Swift applications, this reduces latency, as well as complexity and operational overhead by simplifying builds and deployments.

## Current Status

Swift OPA development is currently focused on expanding support for OPA's built-in functions, starting with over 80 built-ins and a roadmap to add many more in the future. A rigorous conformance test suite has also been published to GitHub. A community discussion has been initiated about how to best publish the transformed data used by the test suite, with the goal of ensuring consistent behavior across different language implementations of OPA including Swift OPA.

It's exciting to add Swift support to the OPA community, and you're invited to start exploring Swift OPA today.

## How to get involved

You can explore Swift OPA now at [https://github.com/open-policy-agent/swift-opa](https://github.com/open-policy-agent/swift-opa), and join the community on the Open Policy Agent Slack channel: [#swift-opa](https://openpolicyagent.slack.com/archives/C08PCS1KJ48). Your feedback and ideas are welcome - and there are plenty of opportunities to contribute to the project!
