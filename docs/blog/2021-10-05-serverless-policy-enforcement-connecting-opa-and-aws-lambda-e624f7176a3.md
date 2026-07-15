---
title: "Serverless Policy Enforcement: Connecting OPA and AWS Lambda"
authors: ["gshively11"]
date: 2021-10-05
slug: serverless-policy-enforcement-connecting-opa-and-aws-lambda-e624f7176a3
---

![OPA logo connected to AWS Lambda serverless icon](/img/blog/serverless-policy-enforcement-connecting-opa-and-aws-lambda-e624f7176a3/banner.webp)

[Open Policy Agent](https://www.openpolicyagent.org/) (OPA) provides policy-based control for cloud native environments. It's commonly used alongside massive projects like Kubernetes and Envoy, and has dozens of other integrations and related projects in its [ecosystem](https://www.openpolicyagent.org/docs/latest/ecosystem/). Recent updates to the project aim to better integrate OPA with serverless architectures and other infrastructure with intermittent compute.

[AWS Lambda](https://docs.aws.amazon.com/lambda/latest/dg/welcome.html) is one such serverless solution. When a Lambda function is invoked, its execution environment only stays up until the function responds, at which point the runtime is frozen and all active processes/threads are paused. They are resumed once the next invocation is received, and the cycle repeats. OPA's plugin architecture wasn't designed to handle this freezing process, which resulted in unexpected behavior for things like bundle retrieval and log shipping.

## Why is Manual Better than Automatic?

However, thanks to a great new feature released in Open Policy Agent [v0.32.0](https://github.com/open-policy-agent/opa/releases/tag/v0.32.0), plugins can now be triggered manually instead of automatically. Now, you may ask: "Wait, why is manual better than automatic?" Well, in many cases it's not, but for environments like AWS Lambda, it's not only useful, but critical to the stability and functionality of the OPA process.

First, let's briefly review the default behavior of plugins in OPA. Most of the standard plugins — Discovery, Bundles, and Decision Logs — operate by using a [loop](https://github.com/open-policy-agent/opa/blob/v0.32.0/plugins/logs/plugin.go#L671) running in a [goroutine](https://github.com/open-policy-agent/opa/blob/v0.32.0/plugins/logs/plugin.go#L513) that listens to various signals, one of which is a [timer](https://github.com/open-policy-agent/opa/blob/v0.32.0/plugins/logs/plugin.go#L698). When the timer delay elapses, the plugins will do stuff in the background, e.g. ship logs, check for new bundles to download, etc. OPA calls this a "periodic" trigger.

For most use cases, periodic triggers are exactly what we need. We don't really care precisely when a plugin is doing something in the background, just that it happens roughly within the interval period we've specified. The non-deterministic nature of periodic plugin triggers works well for a majority of applications. But what happens when that non-determinism becomes a problem? What if we need control over exactly when a plugin both starts and finishes a run of its primary task? Well, now we can get that control, thanks to the addition of [Manual Trigger Support](https://github.com/open-policy-agent/opa/pull/3668).

## Introducing Manual Trigger support

Plugins now support an optional "manual" trigger mode that can be set directly in a [plugin's configuration](https://www.openpolicyagent.org/docs/latest/configuration/#bundles). Additionally, if manual triggers are set on the Discovery plugin, all other plugins will inherit that setting. When a plugin's trigger is set to manual, the plugin's background loop will either pause until the [Trigger func](https://github.com/open-policy-agent/opa/blob/v0.32.0/plugins/logs/plugin.go#L639) is called, or it will never start, depending on the plugin. The Trigger func will run a plugin's primary task and return only when the task is complete, or the provided context ends.

Now that we understand what manual triggers do, let's look at how we can use them in AWS Lambda. Lambda's on-demand compute is excellent for saving on compute costs, but it presents a challenge for code that has indeterminate start and end points, i.e., OPA plugins with periodic triggers. We don't want to interrupt the OPA process by freezing the execution environment when it's in the middle of downloading a bundle or shipping logs. And we don't want to wait around for a background loop to kick off those behaviors. Manual triggers give us the deterministic start and end points we need to properly coordinate OPA plugins in Lambda.

## Try it for Yourself

For those that are interested in running OPA in AWS Lambda, GoDaddy has recently [open-sourced a small OPA plugin](https://github.com/godaddy/opa-lambda-extension-plugin) to make this easier, which uses manual triggers to operate OPA as a [Lambda Extension](https://docs.aws.amazon.com/lambda/latest/dg/runtimes-extensions-api.html). Lambda Extensions are deeply integrated into the lifecycle of a Lambda function, allowing you to perform background tasks without impacting the response time of your functions. OPA and Lambda Extensions are a great match, allowing you to plug in OPA's policy enforcement to your serverless infrastructure without complex installation or configuration management.
