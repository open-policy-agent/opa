---
title: "Community Spotlight — Grant Shively"
authors: ["anderseknert"]
date: 2021-05-05
slug: community-spotlight-grant-shively-12903674c28c
---

![Portrait photo of Grant Shively, GoDaddy principal software engineer](/img/blog/community-spotlight-grant-shively-12903674c28c/banner.jpg)

"Community Spotlight" is a new series of blogs where we talk to people in and around the OPA community: users, integrators and contributors. Our first "spotlight" is [Grant Shively](https://www.linkedin.com/in/grant-shively/), principal software engineer at GoDaddy.

**Okay, so let's start with an introduction.**

Sure! My name is Grant Shively and I've been working at GoDaddy for the last 13 years. For the last six years or so I've been a principal engineer on what we call our Care Platform, which is essentially our CRM system.

On the infrastructure side, we've moved from bare metal legacy servers with big monolithic ASP.NET web applications to a cloud-native, microservice architecture. Meanwhile we've transitioned much of the application platform towards Node.js and .NET Core. Recently, I've been doing a lot of work migrating things from our internal cloud to AWS.

For many years I've been involved in projects where we've needed complex authorization policies. And so we've built all these systems to deal with that, but there's never been a company-wide solution for that kind of thing.

I've been pushing for an authorization platform for the company for a few years, and last year it finally gained traction. We completed two company-wide initiatives with leaders from each of the major organizations where we all got together to talk about how we wanted to solve for authorization holistically at GoDaddy.

During the initiatives, we did a buy vs. build analysis where we evaluated different vendors, and open source projects. And that's when I discovered OPA. I watched the [Netflix talk](https://www.youtube.com/watch?v=R6tUNpRpdnY), which showed how they used OPA to solve many of the issues we are currently experiencing.

And so we did a proof of concept at the end of last year to prove the value and efficacy of the project using OPA, and we got buy-in on it, and now we're here, building out the final multi-tenant solution! We are hoping to have the next team running in production by the end of Q2, and we already have a number of teams lined up to onboard after that.

**That was a great introduction! So, OPA adoption starting from a platform team, and growing from there?**

I've definitely evangelized OPA within the company, since I'm a big fan of the project. There's someone from another team helping to contribute to this platform we're building so I guess that makes us two teams at the moment. My team is focused on the systems that support our customer service guides, while the other team is working on our domain control center. Initially, they're looking to implement this authorization platform for some of the high risk scenarios we have around domains, such as transfer of ownership and things like that.

**You said you were working mainly with .NET and Node.js?**

Yep. As a company we work in a number of languages, including Python, Go, Node.js, .NET Core, and Java… I think those are most of the blessed languages, but there are probably some other languages used for one-offs as well.

My team, we were traditionally C#, and then we brought in Node.js during our transition to microservices. With the arrival of .NET Core, we were comfortable with continuing to support C# as well. Personally, after working with both languages for a number of years, I prefer Node.js for implementing our small, single-purpose APIs.

**You've made quite a few contributions to OPA, which is written in Go. Did you have experience working with that before?**

No, I hadn't touched it before! Go is definitely approachable. And I think the OPA code base is very clean and logical to explore. You don't have to look too much to figure out where something's at, once you get used to it.

Go feels like a language that minimizes syntactic sugar. There's really only a couple of "right" ways to do things, I feel. Sometimes it's frustrating too, like with the lack of map/filter/reduce operations. Coming from more functional paradigms it feels frustrating at times.

**Agreed. In this process of finding OPA, what other alternatives did you consider? Building your own? Some commercial options?**

We looked at [Athenz](https://github.com/AthenZ/athenz). One of our senior architects had worked on that project and it looked interesting. It did some of the stuff we wanted, but not all. And then we looked at a whole bunch of products from various vendors. We briefly thought about building our own until we ran into OPA. OPA completely negated any reason for us to try and build our own because it did exactly what we needed, and in such an elegant way that it would be very difficult for us to replicate.

**Compared to the other options you considered, what was the main appeal of OPA? What was it that won you over?**

One of the things we needed was the ability for multiple engineering teams to work with policy, in a self-service manner, while at the same time leveraging globally-managed policy and data, like authentication token expiration rules and identity attributes.

Like everybody else, we have a few home-grown systems, and we definitely needed to be able to integrate with those. Also, we have an extremely distributed architecture. Hundreds of AWS accounts, and whatever solution we found was going to need to run in pretty much all of them.

I knew that there were going to be integration points that would be very difficult for most vendor products. One thing we really didn't want was some sort of centralized authorization API for the whole company. We've been bitten by such architectures in the past. Our apps are highly distributed, and we need our authorization decision points to be highly distributed too, while still being able to manage and distribute policy from a central location. And a lot of the bigger vendors had these products that just felt a little too heavy for large-scale decentralization, you know?.

**Right.**

And we really liked Rego. We've talked about this before, but a lot of the vendors are based on [XACML](https://en.wikipedia.org/wiki/XACML), which is just really tedious to work with. Compared to that, Rego was a breath of fresh air. Another factor we considered was the buy-in we saw from some of the bigger companies like Netflix, etc. Rapid adoption of a project is usually a great sign.

I'm sure I'll have more opinions on Rego once we try and onboard more teams onto our platform. Certain teams at GoDaddy have more operational experience than others, and we'd really like to build a simplified UI policy builder on top of our platform and Rego. That way, they can just fill out some simple input boxes, hit enter, and it publishes a policy for them. That kind of thing.

**Interesting!**

Yeah, I'm both looking and not looking forward to tackling that problem, haha!

**Maybe you don't remember anymore, but did you have any such moments where you got stuck while learning Rego? I know one thing that felt a little odd to me when starting out was how Rego handled undefined values. How rule evaluation just stops when they are encountered.**

Some of the syntax around iteration was a little confusing at first. I think it's one of those things that makes total sense once you understand it, but it wasn't like anything I'd worked with before.

The biggest issue though — and we still kind of have it—is understanding how a decision was made, and how to surface that in logs. We have a couple of [open](https://github.com/open-policy-agent/opa/issues/2755) [issues](https://github.com/open-policy-agent/opa/issues/2089) about that, and we have been running OPA internally with a few patches we want to upstream that help address some of this.

It's really important to our security and engineering teams that we can look at a somewhat human readable, but hopefully not super verbose, way of saying that "this authorization request was approved or denied because of these reasons".

Another related issue would be how to best propagate obligations up through layers of policy. Some obligations can come from a really low level, like policy checking the claims of a JWT, and if some property doesn't exist, then we want to propagate an obligation up from that lower level rule. Solving some of those problems has felt a little clunky.

**On the other side, what did you really like?**

I really liked that you could create custom built-in functions and other things to extend OPAs functionality. That has been really useful to us as we've built things around our requirements, like custom key signing, custom decision logging, and so on.

There have been a few cases where we needed to extend portions of the OPA code, only to find that they were hardcoded around a particular implementation, without interfaces for us to leverage. I've been very pleased with the responsiveness of and guidance from the OPA maintainers. We've been able to contribute a number of small changes to make the OPA code more flexible for our extensions.

**On the topic of flexibility, one of the things that really appealed to me was the number of [integrations](https://www.openpolicyagent.org/docs/latest/ecosystem/) available. Even if you're never going to use all of them, it really shows what the general purpose nature of OPA enables.**

Yeah, that brings to mind another consideration. Netflix talked about how they used OPA for authorization across their entire stack—infrastructure, forward facing code, back end. When I saw OPA, I knew our platform could grow to be much more than just, you know, a simple authorization engine—there are many other policy-based scenarios that don't necessarily have to do with authorization. So that may be a far future thing, but that was also very enticing about OPA.

**I think some of the flexibility really pays off at scale. Having one unified language to describe policy across the stack, having one place to go for decision logs, and so on.**

Yep. We're definitely looking at increasing the number of integrations. So many of the systems deployed today work only with Active Directory, and AD group-based authorization, so it'll be interesting to see how we can integrate with systems like that.

**About integrations. Are there any of the existing ones in the [ecosystem](https://www.openpolicyagent.org/docs/latest/ecosystem/) you are using or are planning to use?**

I'm only familiar with the [Envoy one](https://github.com/open-policy-agent/opa-envoy-plugin). Oh, and Kubernetes. What else is there? We're just starting to use Envoy, so I think there will likely be some places where we use that integration. And, we'll probably want to look into a Kubernetes integration too at some point, if [AWS EKS](https://aws.amazon.com/eks/) supports that.

Looking at the list now… Wow, there's really a bunch of them here! I'll need to look into some of these. What we're really doing a lot of is AWS integrations. Kind of in the same vein as the Envoy integration, extending OPA and all that, you know? We're doing the same thing with OPA and AWS, and I hope we'll be able to eventually open source that.

Things like key signing using the [AWS Key Management Service](https://aws.amazon.com/kms/) (KMS) or shipping decision logs directly to [AWS Kinesis](https://aws.amazon.com/kinesis/) without having to go through HTTP endpoints in between.

**I know you've mentioned [AWS Lambda](https://aws.amazon.com/lambda/) functions too.**

Yeah. A lot of our APIs are lambda-based, and we'll obviously want to use this platform we're building for that too. We're currently trying to figure out how to make OPA work as well for on-demand serverless functions as it does for traditional compute environments.

**Definitely a hot topic! I've seen some questions on that on the OPA Slack as well. What are the challenges there?**

So, the way OPA currently works is that many of the internal "plugins", like the client downloading bundles, or the one uploading decision logs-they all work on a time-based loop. So you configure them to upload or download or to do whatever they are meant to do at a certain interval, like every thirty seconds or something.

This doesn't really work for serverless functions though as they don't have continuous compute available; functions freeze after serving a response, so nothing can run in the background. What we'd need in this context is rather something based on [other types of triggers](https://github.com/open-policy-agent/opa/issues/2899). We're waiting for a feature called AWS Lambda Extensions, which should reach general availability in May. They should make it possible to use OPA really efficiently even in a lambda context.

**Anything you have found missing from OPA or would like to see on the [roadmap](https://docs.google.com/presentation/u/1/d/16QV6gvLDOV3I0_guPC3_19g6jHkEg3X9xqMYgtoCKrs/edit)?**

I mentioned it before, but yeah, the big thing would be figuring out how to do the tracing in a more efficient way. I have come to understand that the way "full" tracing is done today is apparently pretty expensive. We just need something like [rule level tracing](https://github.com/open-policy-agent/opa/issues/2089), which has been discussed a bit in GitHub issues in the past. With that in OPA proper we wouldn't have a reason to run our own modifications at all, except for plugins.

**Awesome! Finally, what are your future plans for OPA at GoDaddy?**

What we're starting out with is authorization for internal systems. Once we nail that and it's working well we're looking to expand it to authorization for customer systems, and at some point possibly our infrastructure too.

With what OPA can do, I feel like the sky's the limit in terms of what you want to throw into it. We just have to make sure we fully support the platform we're building as we continue growing. I have high hopes for it!
