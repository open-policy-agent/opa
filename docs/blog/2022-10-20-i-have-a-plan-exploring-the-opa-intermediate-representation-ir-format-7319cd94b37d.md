---
title: "I have a plan! Exploring the OPA Intermediate Representation (IR) format"
authors: ["anderseknert"]
date: 2022-10-20
slug: i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d
---

![Diagram illustrating the OPA intermediate representation format](/img/blog/i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d/banner.webp)

It isn't an overstatement to say that the versatility of Open Policy Agent (OPA) is a key factor in its success. As a general purpose policy engine, OPA needs to handle inputs from a disparate set of systems — Terraform, Kubernetes, CI/CD pipelines or custom applications, to name a few — and deliver decisions in a format understood by that particular system. Providing an agnostic approach to input and output data — anything that is or can be modeled hierarchically in a JSON or YAML document is potentially subject to policy — allows [integrating](https://www.openpolicyagent.org/docs/latest/ecosystem/) OPA with all kinds of systems, applications and tech stacks.

But how does the data get passed between client application and OPA? Running OPA as a standalone service, and querying OPA for decisions over its [REST API](https://www.openpolicyagent.org/docs/latest/rest-api/), is by far the most common way to integrate OPA, and for good reasons! Running OPA as a separate component provides a nice, unified interface for communication, which commonly involves writing only a few lines of code in most modern programming languages. Additionally, many technologies allow extending the built-in functionality for e.g. authorization by utilizing _webhooks_, which are commonly REST requests with a JSON-encoded payload. Perfect integration point for OPA!

There are however some scenarios where the standalone REST model proves to be challenging:

- Resource-constrained environments like embedded systems. While OPA is fairly light-weight, some environments simply don't have the resources required to run a standalone OPA service, or networking capabilities for querying.
- Distributed deployments with tight latency budgets, where every millisecond counts.
- Environments constrained by other limitations on what type of software can be run, like web browsers.

To accommodate these requirements, OPA provides a few [alternatives](https://www.openpolicyagent.org/docs/latest/integration/) to the standalone service model:

- Applications written in Go may integrate directly with the OPA Go API, or through the high-level Go SDK, alleviating the need for OPA to run as a separate service.
- Policy may be compiled into Wasm modules, which can then be evaluated in any Wasm runtime. While the most famous Wasm runtime may be that included in web browsers, most programming languages today offer integrations with a Wasm runtime, allowing (at least the policy evaluation parts of) OPA to run "inside" of the application rather than outside of it. Additionally, OPA itself ships with a Wasm runtime, which makes it possible to have OPA pull down bundles including Wasm compiled policy for faster evaluation, and potentially other benefits.

## Wasm

Rego policies compiled to Wasm modules offers a flexible, highly performant alternative to "regular" policy evaluation, with runtimes available for a [wide array](https://github.com/appcypher/awesome-wasm-runtimes) of languages, frameworks and platforms. As such, it should be considered an option for any OPA integration where the standalone server model falls short of the requirements. However, as ubiquitous as Wasm runtimes may be, they are not available _everywhere_. Embedded environments, exotic architectures or specialized hardware all constitute examples of environments where we're unlikely to encounter a Wasm runtime. But even with one available, Wasm _itself_ is not without limitations, even by design!

With the goals of providing a safe, _sandboxed_ environment, originally targeting web browsers, Wasm has several restrictions on what can and can't be done in the confines of the runtime. Interacting with the host system, or for that matter, other host systems — whether through system calls, network requests, or file system operations, is generally prohibited. While the WebAssembly System Interface (WASI) aims to offer an API for this exact purpose, and could potentially be used for certain features of Rego (like the http.send built-in function) in the future, relying on WASI means that a policy evaluated in one runtime might not work in another, as currently only a certain subset of the WASI API is implemented in any given runtime, and Wasm runtimes like those provided by web browsers likely have no interest in supporting interactions with the host system _at all_. Last, while some great progress has been made around WASI recently, it is still nowhere near the maturity of Wasm.

## Intermediate Representation (IR)

OPA v0.37.0, released early 2022, brought two major enhancements to OPA: [compiler strict mode](https://www.openpolicyagent.org/docs/latest/strict/) and [delta bundles](https://www.openpolicyagent.org/docs/latest/management-bundles/#delta-bundles). While those two features might have stolen the show of the release, the [changelog](https://github.com/open-policy-agent/opa/releases/tag/v0.37.0) additionally provides us with this:

> The compile package and the opa build command support a new output format: "plan". It represents a _query plan_, steps needed to take to evaluate a query (with policies). The plan format is a JSON encoding of the intermediate representation (IR) used for compiling queries and policies into Wasm.

Interesting! Now, what does it mean? As alluded to in the last sentence, the low-level building blocks, or the _evaluation plan_, that eventually becomes Wasm, is now made available for consumption by other implementations. What would another implementation look like? That's up to you! While OPA may provide us with the low-level, step-by-step plan, for the evaluation of a query, it'll be on us to parse and evaluate that plan. Ever wanted to have your policy decisions served right inside of your Python app? Doable. Can't run the OPA server on your tiny microcontroller? You no longer need to. No runtime, no restrictions. What's the catch?

## Bring Your Own OPA

Using your programming language of choice to implement the full set of [instructions](https://www.openpolicyagent.org/docs/latest/ir/) included in the intermediate representation format isn't something you'd pull off in just a few hours. A robust implementation is likely going to necessitate quite some effort, and even if you decide to invest the days — or possibly, weeks — required for a greenfield implementation, implementing the IR instructions is only half of the story. OPA provides an impressive number of [built-in functions](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions), requiring corresponding implementations in the platform you choose to target. You probably won't need every single built-in to accommodate your use case though, so starting with the ones known to be relevant for you is likely a smart idea. Rego modules compiled to Wasm, on the other hand, ship with [native implementations](https://github.com/open-policy-agent/opa/tree/main/wasm) of many of the built-ins. If you plan to build an IR compiler or evaluator in C or C++, leveraging those would give you a head start!

Another aspect to consider is the management features provided by OPA. Similarly to Wasm, the scope of the IR format is limited to policy _evaluation_. Fetching bundles from remote endpoints, sending decision logs, or providing metrics and status reports is left as an exercise to the implementation. However, just like with the built-in functions, you likely won't need to support the full set of management capabilities shipped with OPA, but can pick and choose the parts that make sense to you. More interestingly, you're free to implement your _own_ management features. Rather than pulling bundles from an S3 bucket, why not stream your permissions data from a Kafka topic? Or build a direct integration against that tool your organization uses for health checks, and so on. As laborious as a custom implementation may be, it opens up for some very interesting opportunities!

We're getting ahead of ourselves though. Before we dash off to write our own, next generation, OPA implementation in whatever the hottest programming language is these days, we should probably start by getting familiar with the IR format, and how to make sense of evaluation _plans_.

## Making Plans

Let's create a simple policy, and build a plan from that. The below policy contains two rules — `is_admin` to check if the "admin" role is included in the list of roles provided in the **input** for a user, and `allow`, which in this case simply is true if `is_admin` is true, but presumably would be extended to include more checks in future iterations of our policy.

```rego
package policy

import future.keywords.if

import future.keywords.in

allow if is_admin

is_admin if "admin" in input.user.roles
```

Simple enough, right? Let's see what a plan might look like! In order to build one, we'll use the aptly named `opa build` command. This command is used to build [bundles](https://www.openpolicyagent.org/docs/latest/management-bundles/), and the `--target` flag allows us to say that rather than just copying Rego and data files into the bundle, we want OPA to compile either a `plan`, or a `wasm` and put that in the bundle for us too. When building a plan, we'll additionally need to provide an _entrypoint_ — this would be the path to either a package or a rule, from which the plan should be built. The path to the entry points (more than one is allowed) will later be used to query an implementation capable of parsing and evaluating our plan. Let's build a bundle with the plan target, and the entrypoint set to that of our `allow` rule:

```bash
opa build --target plan --entrypoint policy/allow .
```

This will create a `bundle.tar.gz` file in the current directory, with our plan inside of it. Since we're only interested in the plan for now, let's extract it from the bundle:

```bash
tar -zxvf bundle.tar.gz /plan.json
```

## The plan.json file

We now have a plan to work with! Let's see what's in that plan.json file. The first thing you'll notice is that the plan file contains three top level attributes — `static`, `plans` and `funcs`. The `static` object is fairly straightforward:

```json
{
  "static": {
    "strings": [{ "value": "result" }, { "value": "user" }, { "value": "roles" }, { "value": "admin" }],
    "builtin_funcs": [
      {
        "name": "internal.member_2",
        "decl": {
          "args": [{ "type": "any" }, { "type": "any" }],
          "result": { "type": "boolean" },
          "type": "function"
        }
      }
    ],
    "files": [{ "value": "policy.rego" }]
  }
}
```

The `strings` array contains references to all strings included in the plan, and will be referenced whenever needed in evaluation. We'll recognize the "user", "roles" and "admin" strings from our policy, while the "result" string has been added by the plan builder, to be used as a key in the result set from plan evaluation. Thanks, plan builder! The `builtin_funcs` array provides a list of all the built-in functions used in our policy, along with the types expected for their arguments and return values. While "internal.member_2" might look unfamiliar, it's the internal name used for the built-in function representing the `in` operator used in our policy! Finally, the files array contains a list of all files used to build the plan, which in our case is only `policy.rego`.

The next attribute is the actual _plans_, and here's where things start to turn a bit cryptic. But don't worry, I'll walk you through it!

```json
{
  "plans": {
    "plans": [
      {
        "name": "policy/allow",
        "blocks": [
          {
            "stmts": [
              {
                "type": "CallStmt",
                "stmt": {
                  "func": "g0.data.policy.allow",
                  "args": [{ "type": "local", "value": 0 }, { "type": "local", "value": 1 }],
                  "result": 2
                }
              },
              {
                "type": "AssignVarStmt",
                "stmt": {
                  "source": { "type": "local", "value": 2 },
                  "target": 3
                }
              },
              {
                "type": "MakeObjectStmt",
                "stmt": {
                  "target": 4
                }
              },
              {
                "type": "ObjectInsertStmt",
                "stmt": {
                  "key": { "type": "string_index", "value": 0 },
                  "value": { "type": "local", "value": 3 },
                  "object": 4
                }
              },
              {
                "type": "ResultSetAddStmt",
                "stmt": {
                  "value": 4
                }
              }
            ]
          }
        ]
      }
    ]
  }
}
```

## Functions, statements, blocks

For each entrypoint provided, we'll find a corresponding _plan_, which represents the planned evaluation path for that entrypoint. The `name` of the plan is our entrypoint ("policy/allow") and the `blocks` attribute contains the statements to be evaluated in order to "run" the plan. Quite literally in order too, as each statement block and statement will be evaluated in an entirely procedural fashion. Quite a contrast to the Rego code that produced it!

The first statement is a `CallStmt`, which means we'll need to evaluate the function (i.e. `func`) corresponding to the provided name in the `funcs` object — in our case this has been named "g0.data.policy.allow" (mapped from our `allow` rule) — and we'll take a closer look at the `funcs` part in a minute. The `args` provided to the function is "local" value 0, and "local" value 1. These represent the global **input** and **data** variables that you're likely familiar with from your Rego policies, and "value" in this case is rather the "name" — or pointer — to the values in the local _scope_. Who said naming was one of the hardest problems in computer science? Not the OPA plan compiler!

## Where the Locals Go

Since you'll see a lot of references to `local` throughout compiled plans, learning how it's used is imperative to understanding the steps involved in plan execution. When a function is invoked, like our "g0.data.policy.allow" above, a local object is created to represent the inputs to that function. The _statements_ that comprise the function may in turn both read from the local object, as well as write to it, effectively making it a bearer of local _state_.

If a statement inside of a function involves calling _another_ function, a new, "inner" `local` object will be created for the scope of that function, and the result of the function evaluation will be stored in the "outer" `local`, and so on.

Back to our `CallStmt`! We now know that it'll be invoked with the input (local 0) and data (local 1) as its arguments. The next attribute in the statement simply says "return": 2, meaning that whatever value is returned by the function should be stored in the next position — i.e. 2 — in the local state. Next up, the `AssignVarStmt` is used to assign the value at position 2 — that's our return value — to a new local value at position 3 (as defined by the "target" attribute). Moving on, we'll see a `MakeObjectStmt` used to create an empty object, which is placed at local value 4. In the next step, a key value pair is inserted into the object (`ObjectInsertStmt`) where the key is the "string_index" at position 0. Remember the `strings` attribute from the `static` object from before? This is it. The first item in that array is "result", so it looks like we're building a result set over here! The value associated with the result is good ol' local value 3, which we might recall was the result of invoking the "g0.data.policy.allow" function. Finally `ResultSetAddStmt` signals that we're done here. We have a result from the plan, and we're now ready to return it.

## Following Procedure

What about "g0.data.policy.allow" then? I promised we'd get back to the "funcs" object in a minute, and wow, time really flies when describing procedural instructions of an evaluation plan! A quick glance at the [funcs](https://gist.github.com/anderseknert/d2aec51b950362e156bb413ad312fca3) object reveals that it contains not just the "allow" function, but also our "is_admin" function, here in the form of "g0.data.policy.is_admin". Since the allow function merely mirrors the result of is_admin, let's zoom in on the latter to learn how an implementation would evaluate the statements step by step, and how the state of the local object is updated in (almost) each step along the way. Rather than describing the steps in words, let's use a table for demonstration. In the left column you'll see a simplified version of each statement called, and in the right you'll see the local state after the statement has been applied. Note how each step procedurally builds up the final state, which is eventually returned to the caller. Beautiful, isn't it?

![Evaluation plan statements and local state](/img/blog/i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d/1.png)

_Evaluation plan statements and local state. As Medium does not have embeddable tables, you may find the spreadsheet from the image above in [this Google Sheets document](https://docs.google.com/spreadsheets/d/1pU9J3QkfKwkFOZf7UV_PWPHZspQvHOJxOw_zh4poLoQ/edit?usp=sharing)._

To learn more about the different statements an evaluator implementation may encounter, consult the [OPA docs](https://www.openpolicyagent.org/docs/latest/ir/) on the topic.

## Planning Ahead

Every aspect and instruction of the IR format would be too much for a single blog to cover, but the process of untangling an evaluation plan has hopefully been made clearer by now. While creating a full-fledged "OPA" native to your language or platform of choice might be a huge undertaking, even a basic implementation, with only a handful of built-in functions implemented, gets you surprisingly far towards something that actually feels _usable_. Open source implementations of course would have the benefit of others being able to contribute the parts that make the project usable to them. On the topic of open source implementations, are there any of those out there yet? In fact, there is!

## Introducing Jarl

For the past few months, fellow OPA maintainer [Johan Fylling](https://github.com/johanfylling) and I have spent some of our spare time hacking away on an IR implementation for the Java Virtual Machine (JVM) called [Jarl](https://github.com/johanfylling/jarl). A jarl was a chieftain in the age of vikings, and given the viking theme of OPA itself, we figured it would be a good name for the project. And of course, it has that "J" in there too, which seems almost mandatory for JVM-based software.

We chose to use [Clojure](https://clojure.org/) for our implementation, and while we are both rather novice Clojure coders, it's been a lot of fun to work with! Not only that, but using Clojure means getting access to the broader JVM ecosystem, both in terms of libraries available, and that applications written in Java, Kotlin or Scala will be able to use Jarl eventually. As an added bonus, Clojurescript allows the library to be compiled into Javascript as well, allowing us to target deployments in Node, or web browsers. Quite a versatile platform to build on!

While we still have a long way to go before Jarl is anywhere near production readiness, we're already at a point where it's usable for evaluation of many types of common policies. As we wanted to ensure conformance against OPA from the start, we ported the OPA [compliance test suite](https://github.com/johanfylling/opa-compliance-test) to the IR format, which has proven to be tremendously useful for testing not just plan evaluation, but also the behavior of built-in function implementations. This code should be useful for anyone building their own implementation, so if that's you, make sure to check it out. As for the built-in functions, we currently have [most of them](https://github.com/johanfylling/jarl/blob/main/doc/builtins.md) ported, but some work remains to be done before we're able to have all tests from OPA pass. Then awaits the management features…

If the project sounds interesting to you, we'd love to hear from you! Reach out on the OPA Slack, or just try it out and report back on any issues, feature requests or ideas you might have. Or if you'd rather work on your own implementation, we'd love to help you get started.

## Wrapping Up

With the introduction of the intermediate representation format, another integration option for OPA has been made available. While it might be a bit of a niche — and certainly not the first choice to consider for most applications — it opens the door for many new and interesting use cases where an OPA integration using the standalone server model might not have been the best fit, or possible at all. Although perhaps not a well-known option until now, I hope this blog may contribute to changing that, and I'm looking forward to seeing how this alternative is leveraged in the future. Interesting times ahead!
