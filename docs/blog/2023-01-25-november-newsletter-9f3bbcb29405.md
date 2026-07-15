---
title: "OPA Newsletter: November 2022"
sidebar_label: "November Newsletter"
authors: ["peteroneilljr"]
date: 2023-01-25
slug: november-newsletter-9f3bbcb29405
---

_November Edition!_

November has arrived and we are looking forward to the holiday season!

Thanks to all of the community members that stopped by the booth at Kubecon, it was a pleasure meeting you!

## User Survey

We are looking for input from the community to see how everyone is using OPA. Take 5 minutes to fill out this 7 question survey to help out the community!

[Take the Survey](https://bit.ly/3UaIhWa)

## Ecosystem Updates

### [Open Policy Agent v0.46.1](https://github.com/open-policy-agent/opa/releases/tag/v0.46.1)

- New language feature: refs in rule heads
- Entrypoint annotations in rule metadata
- New Built-in Function: graphql.schema_is_valid
- New Built-in Function: net.cidr_is_valid

### [Gatekeeper 3.10.0](https://github.com/open-policy-agent/gatekeeper/releases/tag/v3.10.0)

- Kubernetes v1.25+, removal of Pod Security Policies and migration to Pod Security Admission 🔐
- Mutation is promoted to stable 🦠
- Introducing Validation of Workload Resources as alpha 🚀
- Performance improvements 🏃

## Contributor Shout Outs

Thanks to all of the contributors that participated in these releases, the OPA community wouldn't be here without you!

- @mattfarina
- @jaspervdj
- @ricardomaraschini
- @byronic
- @philipaconrad
- @pjbgf
- @caldwecr
- @hzliangbin
- @peterchenadded
- @phantlantis
- @ericjkao
- @TheLunaticScripter
- @humbertoc-silva
- @Juneezee
- @vinhph0906
- @aholmis
- @Joffref
- @olegroom
- @iamatwork
- @fredallen-wk
- @bartandacc
- @max0ne
- @OpenSourceZombie
- @JAORMX
- @Boojapho
- @ethanrange
- @stp-bsh
- @qa-ship-it
- @salaxander
- @boatmisser
- @gracedo
- @meons
- @mariusblarsen

## Community Tools

### circle-policy-agent

The policy-agent is essentially a CircleCI-flavored wrapper library around the Open Policy Agent (OPA), which will allow the users to write the policy documents in CircleCI terminology.

[Star on GitHub](https://github.com/CircleCI-Public/circle-policy-agent)

### custom-opa-spicedb

This experiment adds support for querying relations from Authzed / SpiceDB via GRPC to check resource level permissions as custom builtin commands for Open Policy Agent.

[Star on GitHub](https://github.com/thomasdarimont/custom-opa-spicedb)

## Videos 🎥

### Policy as Code with Open Policy Agent — Anders Eknert, Styra

Should user Alice be allowed to read credit reports? Should a cloud instance be deployable without basic security configuration in place? Should service X be allowed to query the database? Policy defines the rules of our systems, but how do we ensure our policies are enforced consistently in increasingly distributed and diverse tech stacks? In this talk we'll explore the benefits of decoupling policy from our applications, deployment pipelines and platforms, and how Open Policy Agent (OPA) can help unify the way we work with policy across the stack.

### Securing kubernetes with opa and gatekeeper

Starts at 3:23:20 as part of the Kubehuddle Edinburgh event.

## Blogs

- [I have a plan! Exploring the OPA Intermediate Representation (IR) format](/blog/i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d)
- [5 Application Authorization Best Practices for Better Cybersecurity](https://thenewstack.io/5-application-authorization-best-practices-for-better-cybersecurity/)
- [Intro to sets in Rego](https://qjuanp.dev/post/introduction-sets-rego-open-policy-agent)
- [OPA into WASM](https://inspektor.cloud/blog/evaluating-open-policy-agent-in-rust-using-wasm/)
- [Opa for k8s](https://dev.to/thenjdevopsguy/open-policy-agent-opa-for-kubernetes-5895)
- [Spring Security Authorization with OPA](https://www.baeldung.com/spring-security-authorization-opa)
- [Programming Your Policies: Justin Cormack at QCon San Francisco 2022](https://www.infoq.com/news/2022/10/programming-policy-code/)

## Let us know how we did

The OPA monthly newsletter is built for the OPA community, let us know what you liked or what you wanted to see more of. Reach out using one of the links below.
