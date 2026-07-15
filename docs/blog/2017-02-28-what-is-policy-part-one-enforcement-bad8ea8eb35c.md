---
title: "What is Policy? Part One: Enforcement"
authors: ["tsandall"]
date: 2017-02-28
slug: what-is-policy-part-one-enforcement-bad8ea8eb35c
---

_Welcome to the Open Policy Agent project. If you're interested in topics like policy, enforcement, remediation, and compliance we'd love to hear from you! Join us on [Slack](http://slack-inviter-1327627577.us-west-2.elb.amazonaws.com) or check out the project on [GitHub](https://github.com/open-policy-agent/opa)._

This is the first in a two-part series about policy where we introduce definitions, concepts, and challenges in policy enforcement. In future series we'll examine the state of policy in the cloud-native ecosystem.

The word **policy** means different things to different people in different contexts. In the context of software systems, policies are the rules that govern how the system behaves.

Computers and humans use policy to answer questions such as:

- Is this user allowed to change the config of that service?
- Is this VM allowed to accept TCP connections from that VM?
- Which host should this container be deployed on?
- Which workloads are running in the wrong geographic region?

![Diagram showing where policy decisions occur across a Scheduler, Host, Container, App, sshd, Network, and DB](/img/blog/what-is-policy-part-one-enforcement-bad8ea8eb35c/1.webp)

We define policy to avoid repeating mistakes and ensure important requirements are met around cost, technology, security, legislation, internal conventions, and so on.

Understanding where requirements are met and where they aren't is by itself quite valuable (more on this in a later post), but eventually we need to **enforce** policy to ensure that systems behave the way policy says they should.

We enforce policy differently depending on the organization, the technology the policy applies to, and of course, the policy in question. In some cases, we rely on in-person communication whereas in other cases we embed special-purpose components into our systems that enforce policy automatically.

![Diagram of the policy maturity spectrum from tribal knowledge and wiki to hard-coded, config, and policy engines](/img/blog/what-is-policy-part-one-enforcement-bad8ea8eb35c/2.webp)

At one end of the spectrum, policies are **tribal knowledge**. They are not recorded anywhere, so if we to want to modify the system, or even understand how it's expected to behave, we ask someone.

Eventually, we tire of answering (or asking) the same question so we record the answer **on the wiki** or **in the docs**. These docs always fall out of date eventually.

When our companies grow rapidly, introduce automation, or are faced with frequent policy violations, we usually start **hard-coding** policy into our software. If policy could be defined once and forgotten, we would stop there. In reality, policy evolves. With hard-coded solutions we have to read the code to understand (let alone modify) policy. As a result policy becomes less accessible and more expensive to maintain.

Once the pain of hard-coding becomes evident, we make our policies **configurable** in software (e.g., including config parameters or even going as far as defining custom [DSLs](https://en.wikipedia.org/wiki/Domain-specific_language).) However, our [abstractions often leak](https://en.wikipedia.org/wiki/Leaky_abstraction) or cannot be adapted for future requirements. As a result, we spend more time and money on development, re-education, and upgrades. It turns out that predicting future requirements around cost, technology, internal conventions, and so on, is HARD.

The eventual conclusion of this progression is the **policy engine**: a tool for codifying and enforcing policies that is flexible enough to encompass a wide range of future requirements around cost, technology, internal conventions, and the like. It balances the desire to have programmatic enforcement (like hard-coding and configuration) with the need to update policy frequently and inexpensively (like tribal knowledge and the wiki).

In our next post we'll look at policy engines, declarative languages, and the decoupling of policy decisions from enforcement. Thanks for reading!
