---
title: "Rego Design Principle #3: Optimize Performance Automatically"
authors: ["timhinrichs"]
date: 2020-04-07
slug: rego-design-principle-3-optimize-performance-automatically-2d29ad3ce96d
---

![Banner for Rego design principle on automatic performance optimization](/img/blog/rego-design-principle-3-optimize-performance-automatically-2d29ad3ce96d/banner.webp)

This is the third part of a blog series on the design principles behind Open Policy Agent's (OPA's) policy language Rego. Previously, we described how Rego's [syntax was designed to mirror the structure of real-world policies](/blog/rego-design-principle-1-syntax-should-reflect-real-world-policies-e1a801ab8bfb) and how Rego [embraces hierarchical data](/blog/rego-design-principle-2-embrace-hierarchical-data-8a4913bdfea1). In this part of the series, we look at Rego/OPA's commitment to automated performance optimization so that policy authors can focus on writing readable, maintainable policies, and OPA can shoulder the burden of evaluating those policies efficiently.

## Performance is Important, but Policy Authors Should Be Able to Ignore It

The goal of OPA is to take policies that people write and automatically enforce, monitor and remediate them. No more relying on people to remember the policies, to understand them or to correctly apply them. OPA helps everyone follow the rules and protects the infrastructure, applications, etc. from security, compliance and operations problems.

The scale and speed of modern, cloud-native systems means that OPA policies are routinely applied to millions of objects and actions. [Pinterest, at the last Kubecon, described using OPA to make 400k decisions per second (8M with caching)](https://www.youtube.com/watch?v=LhgxFICWsA8), globally across all infrastructure. At that scale, how long it takes to evaluate an OPA policy, how much memory it uses, and how many requests can happen in parallel all matter a great deal.

Despite the importance of performance one of OPA's design principles is to minimize how much a policy author worries about performance. The policy author should write the logic of her policy so that it is easy to read, maintain, extend, and combine with other peoples' policies. The author shouldn't be forced to complicate the logic of her policy in order to make it perform well.

> The policy author handles correctness. OPA handles performance.

In the end, this design principle is all about usability. Policies are easier to write because you only think about correctness and not performance. Policies are easier to read because they're written closer to the way people think about them in the real world. Policies are easier to maintain and extend because you write them each individually and leave global optimizations to OPA. Upgrade OPA and your policies get faster.

As a point of comparison every programming language aims for good performance too, but OPA's goal is qualitatively more ambitious in the sense that even the global asymptotic runtime of policy evaluation is something that the policy author should be able to ignore. If the clearest way to write/read your policy looks like an O(n¹⁰) algorithm in a traditional language even though it can be done with 10 linear scans, OPA aims to analyze that policy, reformulate it, set up the proper indexing and evaluate it in linear time.

## How OPA Automates Performance Optimization

Our approach to realizing this design principle was to base Rego on database query languages and leverage 50 years of R&D on automated performance optimization. In particular:

**Rego has no side-effects**. Rego always produces the same outputs given the same inputs, and there are no side-effects before, during or after evaluation. Side-effects would include writing to a file or modifying the value of an in-memory object. Side-effects greatly complicate the design of optimization algorithms because those algorithms need to understand how to preserve those side-effects.

[**Rego rules are unordered**](/blog/orderly-versus-disorderly-policies-717475c23d2f). A Rego policy means the same thing regardless of how the rules are ordered. Since ordering is irrelevant, the meaning of each statement stands on its own and can be optimized on its own.

In contrast, in languages where ordering matters, every optimization must respect that ordering. Firewall rules are a perennial example, where to understand the impact that the 1000th firewall rule has, you need to understand the impact of the first 999. (Rego does support the 'else' keyword for ordering, but we advise using it sparingly.)

```rego
# Both allow and deny are true.
# Order of the statements does not matter.
allow = true {
    input.method == "GET"
    input.path == "/"
}
deny = true {
    input.method == "GET"
    input.path == "/"
}
```

**Rego rules are simple**. Just about every statement in Rego is a conditional variable assignment. A Rego statements assigns a variable to a value if some conditions are true. This uniformity and simplicity help ensure that optimizations apply across all Rego policy statements.

```rego
allow = true {                # allow is assigned true if ...
    input.method == "GET"     # method is "GET" AND
    input.path == "/"         # path is "/"
}
```

**Rego is designed in layers**. Rego was designed like an onion: the core is syntactically quite simple and highly efficient; the layer above that is syntactically more expressive at the cost of some performance; and so on. The outer layer today is quite expressive but not Turing complete (though in the future we could relax some of the restrictions on the language if we wanted that escape hatch). Below is a diagram showing some of the layers of the language.

![Performance and Expressiveness layers for Rego](/img/blog/rego-design-principle-3-optimize-performance-automatically-2d29ad3ce96d/3.webp)

Of course there are limits to what can be done at all and what has been implemented, but it's a clear design principle — that the language itself should be amenable to deep, automated analysis and global optimizations. At the time of writing, OPA has the following features either built out (GA), in progress (WIP), or planned.

[**Rego has automatic, multi-dimensional indexing**](/blog/optimizing-opa-rule-indexing-59f03f17caf3) **(GA)**. OPA analyzes rules and automatically constructs a trie that finds the minimal set of applicable rules. For example, the following rules will be represented as the trie shown below.

```rego
# Rules as they appear in a Rego file
allow = true {
    input.method == "POST"
    input.path == "/pets"
    something_complex
}
allow = true {
    input.method == "GET"
    input.path == "/pets/dogs"
}
```

![Rules are represented in memory for fast evaluation](/img/blog/rego-design-principle-3-optimize-performance-automatically-2d29ad3ce96d/2.webp)

[**Partial evaluation**](/blog/partial-evaluation-162750eaf422) **(GA)**. Partial evaluation attempts to compile a policy from one layer into a lower layer (sometimes at the cost of creating additional rules). This could, for example, convert a linear-time policy into a constant-time policy.

```rego
# Before partial evaluation, a data-driven policy that is easy
#    for people unfamiliar with Rego to contribute to.
allow {
    op = allowed_operations[_]
    input.method == op.method
    input.resource == op.resource
}
allowed_operations = [
    {"method": "PUT", "resource": "air-conditioner"},
    {"method": "GET", "resource": "security-camera"},
    {"method": "POST", "resource": "garage-door"},
]
```

After partial evaluation runs we get rules that the indexer can more easily analyze and organize in a trie.

```rego
# After partial evaluation
allow {
    input.method == "PUT"
    input.resource == "air-conditioner"
}
allow {
    input.method == "GET"
    input.resource == "security-camera"
}
allow {
    input.method == "POST"
    input.resource == "garage-door"
}
```

[**Compilation to WebAssembly**](/blog/opa-v0-15-1-rego-on-webassembly-81c226c51be4) **(WIP)**. WebAssembly is a popular general-purpose virtual machine that has implementations in a growing number of languages like Go, Node, JavaScript, and Java. The Rego WebAssembly compiler takes a Rego policy and generates custom WebAssembly code that implements that policy, and like most compiler technology eliminates overhead for the interpreter and provides the opportunity for deep, automated performance tuning.

**Automated asymptotic analysis (WIP)**. While still a work in progress, the goal of this tool is to identify the run-time complexity of a policy. Which of these three layers does it belong to? If it is a linear-time policy what slice of JSON data is it iterating over?

For example, in Kubernetes a common policy is to check if all images come from a trusted registry, say `hooli.com`. The complexity analysis will tell you that this policy has complexity `O(input.request.object.spec.containers)`.

```rego
deny {
 input.request.kind.kind == "Pod"
 some i
 image := input.request.object.spec.containers[i].image
 not startswith(image, "hooli.com/")
}
```

**Query optimization (Planned)**. As use cases like audit that require deeper searches over significant amounts of data become more popular, we plan to include optimizations that analyze dependencies, reorder rule evaluation, reorder conditions within rules, shift evaluation of conditions between rules, and the like.

## Summary

One of OPA and Rego's design principles is that Rego policy authors should focus on correctness, maintainability and composability. They should not complicate their policy logic to make policy evaluation more efficient. More specifically:

- OPA aims to optimize both local operations and the global asymptotic runtime
- While Rego's syntax looks like a programming language, its semantics is based on database query languages.
- The automated optimizations implemented in OPA continue to grow over time.

While OPA and Rego today have made terrific progress in terms of realizing the goal of automated performance optimization, there's still a lot to do. If you want to help, hop onto Slack or Github and contribute!

If you want to know more, check out the other blog posts in the series:

- [Design principle #1: Syntax should reflect real-world policies](/blog/rego-design-principle-1-syntax-should-reflect-real-world-policies-e1a801ab8bfb)
- [Design principle #2: Embrace hierarchical data](/blog/rego-design-principle-2-embrace-hierarchical-data-8a4913bdfea1)
