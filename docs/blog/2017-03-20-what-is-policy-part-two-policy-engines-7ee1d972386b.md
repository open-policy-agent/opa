---
title: "What is Policy? Part Two: Policy Engines"
authors: ["tsandall"]
date: 2017-03-20
slug: what-is-policy-part-two-policy-engines-7ee1d972386b
---

_This is the second post in a two-part series about policy enforcement. If you're unfamiliar with the idea of policy enforcement check out [Part One: Enforcement](/blog/what-is-policy-part-one-enforcement-bad8ea8eb35c)._

Last time we examined alternative approaches to policy enforcement. We saw that tribal knowledge and documentation on wikis provide few guarantees about enforcement but also that automated solutions often make policy difficult to understand and expensive to maintain. In this post, we'll examine how policy engines can help balance the desire for automated enforcement and the need for ease-of-use.

Policy engines have been around for years. If you've worked on networking or authorization, you've used one when setting up [ACLs](https://en.wikipedia.org/wiki/Access_control_list) or writing [RBAC](https://en.wikipedia.org/wiki/Role-based_access_control) policies. You may also have heard of more general-purpose policy technologies, such as [XACML](https://en.wikipedia.org/wiki/XACML), or protocols like [RADIUS](https://tools.ietf.org/html/rfc2865).

At a high level, policy engines take policy and data as input and produce answers to policy questions as output. We can design policy engines as libraries, [sidecars](https://www.usenix.org/system/files/conference/hotcloud16/hotcloud16_burns.pdf), or full-blown services. In a future post, we'll examine the trade-offs with each approach.

When policy engines are integrated into our systems, we refer to those systems as **policy-enabled**. The goal of policy-enabling systems is to **decouple policy decisions from policy enforcement**. This decoupling results in policy implementations that are easier to understand, flexible enough to handle future requirements, and less expensive to maintain.

For example, a policy-enabled API-gateway asks its policy engine whether a client request should be allowed. The policy engine makes a decision, and the API-gateway rejects the request or forwards it along. Without the policy engine, the logic that decides whether to accept or reject is hard-coded/configured into the API-gateway.

![Diagram of a policy-enabled API gateway querying a policy engine](/img/blog/what-is-policy-part-two-policy-engines-7ee1d972386b/1.webp)

Decoupling allows us to define policy in a language different from the one used to implement the service that enforces policy. We can choose a higher-level language for expressing policy that makes policy easier to write, update, understand, analyze, and optimize. For example, it's much simpler (for most people) to read and write **"permit tcp host 1.2.3.4 port http"** than it is read or write the equivalent C (or Java or Python or …) code.

Policy engines usually support **declarative languages** for defining policy. A declarative language lets us tell the system _what_ we want it to do as opposed to imperative code where we tell the system _how_ to do what we want. Declarative languages balance peoples' need for expressing policy with the policy engine's need to understand the policy definition.

Declarative languages are also nice because they can provide:

- **Guarantees that code will return an answer (i.e., it will not run forever.)
- **Concise and readable syntax designed to express constraints.
- **Consistent and repeatable results given the same code and data.
- **Dry-run features to see what would happen if code or data changes somehow.
- **Hot-reloading when we want to change the deployed policy.
- **Performance optimizations without requiring us to change our code.
- **Debugging support to answer questions like "why was decision X made?"

Beyond the strengths of using declarative languages for policy, decoupling also enables:

- **Visibility into policy violations that have occurred in the system.
- **Automatic remediation when the policy or relevant state changes in the system.
- **Sharing across different components in the system (which may be written in different languages).

All of the benefits described above become available when we decouple policy decisions from policy enforcement. Furthermore, when we use high-level declarative languages to express policy, we simplify the task of reading, writing, and managing the rules that govern our systems.

That said, it's a lot of work to build policy engines with everything described above. We have to create well-defined languages, implement parsers, compilers, and query evaluation. We also need solid APIs and powerful tools to ingest data, execute queries, debug errors, profile performance, and so on.

In upcoming series we'll dive into existing policy efforts in the cloud native ecosystem and talk more about [the Open Policy Agent project](http://www.openpolicyagent.org). Thanks for reading!
