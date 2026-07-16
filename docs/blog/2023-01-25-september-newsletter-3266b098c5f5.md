---
title: "OPA Newsletter: September 2022"
sidebar_label: "September Newsletter"
authors: ["peteroneilljr"]
date: 2023-01-25
slug: september-newsletter-3266b098c5f5
---

_September Edition!_

Happy September Everyone! This month's edition is coming in a little late, but don't worry, it's still packed with great information.

Don't forget to register for Cloud Native Policy Day with OPA! More info at bottom.

[Register Today!](https://www.styra.com/cloud-native-policy-day-with-opa-2022/)

## Community Updates

The Rego Playground now has a "Format" button! 🎉

This button auto-formats your policy code in the editor, as well as your input/data JSON documents.

## Ecosystem Updates

### [Open Policy Agent v0.44.0](https://github.com/open-policy-agent/opa/releases/tag/v0.44.0)

- security fixes, which mitigate CVE-2022-36085 in OPA itself, and CVE-2022-27664 and CVE-2022-32190 in our Go build tooling.
- [Linear performance scaling for sets up into the 500k key range and beyond](https://github.com/open-policy-agent/opa/pull/4999)
- [The union builtin is now about 15-30% faster than the equivalent operation in pure Rego.](https://github.com/open-policy-agent/opa/issues/4979)
- [This release introduces two new builtins: strings.any_prefix_match, and strings.any_suffix_match.](https://www.openpolicyagent.org/docs/v0.42.0/policy-reference/#builtin-strings-stringsany_prefix_match)

### [NPM-OPA-WASM v1.8.0](https://github.com/open-policy-agent/npm-opa-wasm/releases/tag/1.8.0)

- [New Feature: add loadPolicySync by @elliots in #255](https://github.com/open-policy-agent/npm-opa-wasm/pull/255)

We will discuss these new features in the September 20th Office Hours. Sign up today and send in your questions.

[Join OPA Office Hours](https://calendly.com/peter-styra/opa-office-hours)

## Community Tools

## Goast

Go AST (Abstract Syntax Tree) based static analysis tool with Rego.

[Like on GitHub](https://github.com/m-mizutani/goast)

## Java App with OPA Policies

Motivation for this code and application was to try to understand and implement the Hexagonal Architecture — also called Port and Adapter Architecture.

[Test it out](https://github.com/uwegeercken/artikel)

## OPA Support for Go Fiber

Open Policy Agent support for Fiber.

Note: Requires Go 1.16 and above

[Try it](https://github.com/gofiber/contrib/tree/main/opafiber)

## Blogs

Read up on how the OPA community is using OPA.

- [Control User Access and Permissions in CVAT with Open Policy Agent](https://medium.com/@nikman/control-user-access-and-permissions-in-cvat-with-open-policy-agent-a2abbd09774d)
- [What Exposed OPA Servers Can Tell You About Your Applications](https://www.trendmicro.com/en_us/research/22/h/what-exposed-opa-servers-can-tell-you-about-your-applications-.html)
- [Using XACML with OPA and Rego: The Best of Both Worlds](https://www.styra.com/blog/using-xacml-with-opa-and-rego-the-best-of-both-worlds/)
- [Authorize REST API with OPA (Japanese)](https://christina04.hatenablog.com/entry/opa-rest-api-authorization)
- [Controlling Kafka Data Flows using Open Policy Agent](https://opencredo.com/blogs/controlling-kafka-data-flows-using-open-policy-agent/)
- [Introduction of Open Policy Agent / Rego to realize Policy as Code](https://tech.isid.co.jp/entry/2021/12/05/Policy_as_Code%E3%82%92%E5%AE%9F%E7%8F%BE%E3%81%99%E3%82%8B_Open_Policy_Agent_/_Rego_%E3%81%AE%E7%B4%B9%E4%BB%8B) (Japanese)
- [Collaborating on Access Control Policies with Open Policy Agent](https://zendesk.engineering/collaborating-on-access-control-policies-with-open-policy-agent-fddbc3058359)

## Events 📆

### Cloud Native Policy Day with OPA, Oct 25th

Cloud Native Policy Day with OPA hosted by Styra, the creators of Open Policy Agent, will bring together the OPA community for a day of sharing and discussing policy-as-code best practices, key learnings and creative use cases for OPA. Project maintainers will be on hand to field 1:1 questions and provide live-coding demos — and you'll see proven real-world implementations from various OPA adopters during each of the sessions.

Whether you're looking to start down your policy journey, or are an OPA adopter with Rego skills to share, join the community for sharing, learning and socializing.

Attendees are invited to come for the full day with lunch provided or to stop by just for the sessions that interest them most. To register for the event, add Cloud Native Policy Day with OPA from the co-located event list selections when registering for KubeCon + CloudNativeCon NA 2022 or add it to your existing registration by selecting "modify" on your confirmation page or clicking the "modify" link in your confirmation email.

[Register Today!](https://events.linuxfoundation.org/kubecon-cloudnativecon-north-america/register/)

## Let us know how we did

The OPA monthly newsletter is built for the OPA community, let us know what you liked or what you wanted to see more of.
