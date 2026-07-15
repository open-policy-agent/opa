---
title: "OPA v0.15.1: Rego on WebAssembly"
authors: ["tsandall"]
date: 2019-11-19
slug: opa-v0-15-1-rego-on-webassembly-81c226c51be4
---

![OPA v0.15.1 release banner for Rego on WebAssembly](/img/blog/opa-v0-15-1-rego-on-webassembly-81c226c51be4/banner.webp)

We're excited to announce that with OPA v0.15.1 you can compile any Rego policy into WebAssembly. This release is the culmination of many incremental improvements to the WebAssembly compiler in OPA that we demonstrated with a proof-of-concept at KubeCon 2018 in Seattle.

## What is WebAssembly?

From [WebAssembly.org](https://webassembly.org):

> "WebAssembly (abbreviated Wasm) is a binary instruction format for a stack-based virtual machine. Wasm is designed as a portable target for compilation of high-level languages like C/C++/Rust, enabling deployment on the web for client and server applications."

## What does Wasm have to do with policy enforcement?

One of the principles behind OPA is that policy decision-making should be decoupled from policy enforcement. For people building software products, decoupling lets them avoid undifferentiated heavy lifting and get to market faster because they do not have to worry about implementing a policy or authorization engine from scratch. For operators running large-scale software systems, decoupling addresses fragmentation from siloed policy implementations enabling unified control and visibility across the stack.

Cloud native projects have embraced the idea as they matured. Today, projects like Kubernetes and Envoy have strong support for externalized admission control and authorization checks. However, it's not enough to just offer a plugin model for policy decisions. You need a component that can respond to policy queries with answers. This is where OPA comes in. OPA provides a lightweight general-purpose policy engine that can be embedded throughout your infrastructure. When your software needs to make decisions it can query OPA and OPA will reply with the answer based on the rules and data that you distribute to it.

While OPA is designed to be as lightweight as possible (e.g., by keeping all rules and data in-memory, not introducing any external runtime dependencies, etc.) the fact that the policy evaluation engine is written in Go means that library embedding can be difficult for software not written in Go. Moreover, requiring OPA be installed as a daemon inside (or near) the enforcement point comes with it's own set of challenges and may not always be feasible (e.g., if you are not the owner of the platform.)

This is where Wasm is relevant because it provides a safe, portable, efficient standards-based execution environment for arbitrary code and logic. In the past year Wasm-based extension mechanisms gained popularity: CDN companies like [Cloudflare](https://blog.cloudflare.com/webassembly-on-cloudflare-workers/) and [Fastly](https://www.fastly.com/press/press-releases/fastly-expands-serverless-capabilities-launch-compute-edge) allow you to execute software at the edge using Wasm, service proxies like [Envoy have integrated Wasm runtimes](https://github.com/envoyproxy/envoy/issues/4272), and even databases like [Postgres can be extended with Wasm built-in functions](https://github.com/wasmerio/postgres-ext-wasm).

![OPA ecosystem diagram with WebAssembly integration points highlighted](/img/blog/opa-v0-15-1-rego-on-webassembly-81c226c51be4/2.webp)

With Wasm-based extension mechanisms becoming the norm and programming language support for Wasm runtimes maturing, Wasm will naturally become a standard mechanism for offloading policy evaluation from your software. At the same time, Wasm is only a low-level instruction format. The only data types at your disposal are 32/64-bit integer and floating-point numbers. This makes it impractical to use Wasm directly for any kind of policy specification. Enter OPA.

## How does OPA work with Wasm?

OPA includes a compiler that accepts Rego policies as input and generates an executable Wasm program as output. This Wasm program can be loaded into any standard Wasm runtime and executed when policy decisions are needed. With the v0.15.1 release we have reached an important milestone in the compiler: with the exception of built-in functions, OPA can now compile any Rego policy into Wasm.

In the latest OPA release there are two ways of compiling Rego policies into Wasm:

- On the command-line using the `opa build` CLI tool.
- In Go by integrating with the github.com/open-policy-agent/opa/rego package.

![Diagram of the Wasm compile-time and runtime workflow](/img/blog/opa-v0-15-1-rego-on-webassembly-81c226c51be4/3.webp)

The compilation process takes a Rego query and zero or more Rego files and generates an execution plan that is compiled into a Wasm module binary. The module binary can then be loaded into any Wasm runtime and executed with different input and data values to obtain decisions. The resulting Wasm module binary sizes are not too significant. The baseline size for a policy with a single statement is ~30KB on disk. A policy containing 300,000 statements consumes ~20MB on disk.

The initial benchmarks comparing policy evaluation time in Wasm versus the existing Go interpreter implemented in OPA are promising. For example, a relatively simple policy that searches over a large number of data items to decide whether an operation should be allowed (or not) evaluates ~20x faster with the Wasm compiled version. This is to be expected because the overhead of the interpreter in OPA has been completely removed.

```rego
package example

default allow = false

allow {
  rule := data.rules[_]
  rule.action == input.action
  rule.resource == input.resource
  rule.identity == input.identity
}
```

Benchmark results:

```
# of data.rules | Existing Go interpreter | Wasm compiled policy
----------------+-------------------------+---------------------
100             | 0.354ms                 | 0.0173ms
1,000           | 3.1ms                   | 0.145ms
10,000          | 30ms                    | 1.5ms
100,000         | 316ms                   | 15ms
```

If you are interested in more details about Wasm support in OPA see [this page in the documentation](https://www.openpolicyagent.org/docs/latest/wasm/) and check out [this example on GitHub](https://github.com/open-policy-agent/npm-opa-wasm/).

## What's Next?

We have reached an important milestone for OPA with Wasm and we are excited about Wasm's potential as it relates to policy enforcement. However, there is still a lot left to do.

The main gap (at the moment) is lack of support for the 50+ built-in functions from OPA proper. While you can implement these built-in functions yourself in the host language (e.g., NodeJS) and import them into the Wasm module, we want to make this smoother. Another area that needs work are the management APIs. The OPA daemon and Go library expose APIs that let you control policy distribution, decision logging, and more. In a Wasm-enabled enforcement point these functions do not exist yet.

In the coming months we are going to build out new integrations that leverage Wasm, create SDKs for languages/runtimes other than Go and NodeJS, and use this experience to harden and optimize the new implementation.

If you would like to get involved feel free to file issues on [GitHub](https://github.com/open-policy-agent/opa) or [join us on Slack](https://slack.openpolicyagent.org/).
