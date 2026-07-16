---
title: "Open Policy Agent v0.19 Release"
authors: ["tsandall"]
date: 2020-04-23
slug: open-policy-agent-v0-19-release-921d49179440
---

![Open Policy Agent v0.19 release announcement banner](/img/blog/open-policy-agent-v0-19-release-921d49179440/banner.webp)

Last week we released OPA v0.19, containing 63 commits from 12 contributors (of which, 9 were external.) This release includes many important fixes and enhancements, as well as a new Rego parser written in Go that speeds up parsing time by ~100x in most cases. You can find more details on the [GitHub releases](https://github.com/open-policy-agent/opa/releases) page.

## Community Updates

Since many in-person events have gone virtual due to the COVID-19 crisis, there have been several virtual events, webinars and podcasts featuring OPA over the last few weeks. Here's a quick roundup:

- [@LachlanEvenson](https://twitter.com/LachlanEvenson) ("B-grade Bollywood Actor") and Sertaç Özercan (Software Engineer @Azure, OPA Gatekeeper maintainer, [@sozercan](https://twitter.com/sozercan?lang=en)) presented at the CNCF Member Webinar about ensuring compliance in Kubernetes using OPA Gatekeeper. [Slides and recording are here](https://www.cncf.io/webinars/ensuring-compliance-without-sacrificing-development-agility-and-operational-independence-in-k8s-with-opa-gatekeeper/).
- Michael Hausenblas (Developer Advocate at AWS, [@mhausenblas](https://twitter.com/mhausenblas)) [did a stream](https://www.youtube.com/watch?v=dlKXFYBngBw) talking about Deprek8 with Steve Wade (K8s consultant & Trainer and Platform Lead at Mettle). You can read more in this [Deprek8 article on Opensource.com](https://opensource.com/article/20/3/deprek8).
- Rosemary Wang (Developer Advocate at HashiCorp, [@joatmon08](https://twitter.com/joatmon08)) talked about security & policy for infrastructure as code on OWASP DevSlop! [The stream included live demos of OPA, conftest and more.](https://www.youtube.com/watch?v=KOTXCIN0yE0)
- Alex Krause (Software Engineer at QAware, [@alex0ptr](https://twitter.com/alex0ptr)) spoke at the Cloud Native Virtual Summit about cloud compliance with OPA. You can find the [SlideShare presentation slides](https://www.slideshare.net/QAware/cloud-compliance-with-open-policy-agent). The stream is also [available](https://gateway.on24.com/wcc/eh/2010041/lp/2235047/qaware-gmbh-cloud-compliance-with-open-policy-agent) but requires registration.
- Kevin Harris (Cloud Architect at Microsoft) talked about how OPA and Kubernetes Admission Control at the [Cyber Tech & Risk Virtual Event.](https://www.youtube.com/watch?v=41Ecd8Uuyvs&feature=youtu.be)
- Daniel Mangum (Engineer at Crossplane.io, [@hasheddan](https://twitter.com/hasheddan?lang=en)) [hosted me on The Binding Show to talk about OPA, Crossplane and more.](https://www.youtube.com/watch?v=TaF0_syejXc) I also joined SW Engineering Radio to talk about OPA and distributed policy enforcement in general. You can find the [SW Engineering Radio episode recording](http://hwcdn.libsyn.com/p/b/c/c/bcc49f4be8bc53f1/Episode-406-Torin-Sandall-on-Distributed-Policy-Enforcement_.mp3?c_id=69980306&cs_id=69980306&destination_id=1520171&expiration=1587590198&hwt=48bcf40f509d2271f0952d1e51c23a6d).
- [The New Stack published an article](https://thenewstack.io/open-policy-agent-authorization-for-the-cloud) from Tim Hinrichs (co-creator of OPA and founder of Styra, [@tlhinrichs](https://twitter.com/tlhinrichs)) describing various use cases organizations use OPA for today. Tim also recently published a [great series of blog posts about the design principles behind Rego](/blog/rego-design-principle-1-syntax-should-reflect-real-world-policies-e1a801ab8bfb).

Since KubeCon 2019 in Barcelona, we have asked users to post Q&A style queries on Stack Overflow instead of slack.openpolicyagent.org. The reason is that most **answers posted on Slack are not discoverable**! If you have Q&A style questions (e.g., ["How to test `not deny`?"](https://stackoverflow.com/questions/60083793/rego-testing-how-to-test-not-deny)), try posting on [Stack Overflow and tagging with open-policy-agent](https://stackoverflow.com/questions/tagged/open-policy-agent).

## Faster Parsing & Better Errors

The largest change in v0.19 is the new Rego parser, which is written from scratch in Go. Previously, OPA relied on a [generated](https://github.com/open-policy-agent/opa/blob/v0.18.0/ast/rego.peg) [parser](https://github.com/open-policy-agent/opa/blob/v0.18.0/ast/parser.go) that was defined using [PEG (Parsing Expression Grammar [wikipedia])](https://en.wikipedia.org/wiki/Parsing_expression_grammar). Over the years, as the grammar has grown, and larger inputs have been thrown at it, the generated parser became a bottleneck (e.g., it could take about 10x longer to parse an input than compile and evaluate the query.

Inside OPA we were able to workaround the performance problems with caching, using Go's "encoding/json" package and manually converting to AST ("Abstract Syntax Tree") values when possible, etc. However, new users embedding OPA as a library would (understandably) make mistakes and wonder why performance was poor. The majority of the performance problems in the generated parser were due to a significant amount of heap allocations required to parse any input.

In addition to performance, we also struggled with usability around parser error messages. If the parser was not able to match an input, you would be presented with an error like "policy.rego:19: no match found". No match? Tell me more!

Rather than attempt to continue incrementally improving the existing parser, we decided to rewrite it from scratch in Go. The result is a new parser that allocates significantly less memory (which improves performance by approximately 100x in most cases) and has better error messages. One important requirement for the new parser was backwards compatibility — the new parser could not break existing policies OR programs that embed OPA as a library (e.g., the parser APIs and the AST types also had to remain the same). To ensure we did not break existing (valid) policies, we checked for differences in the output of the old and new parser for hundreds of thousands of Rego snippets (which deserves another blog post in the future.) Lastly, we also applied the wonderful [go-fuzz](https://github.com/dvyukov/go-fuzz) project [to the parser](https://github.com/tsandall/fuzz-opa) to help catch crashes and other bugs.

> Since we no longer have a declarative representation of the language grammar in Go, please refer to the [ENBF grammar in the OPA documentation](https://www.openpolicyagent.org/docs/latest/policy-reference/#grammar) as the authoritative source.

The chart below shows the difference in performance between the old (v0.18 and earlier) and new (v0.19 and later) parser (log scale):

![The chart below shows the difference in performance between the old (v0.18 and earlier) and new (v0.19 and later) parser (log scale)](/img/blog/open-policy-agent-v0-19-release-921d49179440/1.png)

Overall, we are happy with the process. In the future we plan to continue optimizing performance in the parser and looking for ways to improve error messaging and usability.

## man(1) pages, http.send, and Emacs support

In addition to the new parser, v0.19 includes dozens of bugfixes and feature enhancements. [@olivierlemasle](https://github.com/olivierlemasle) contributed code to generate OPA man pages from the OPA CLI definitions. The `man` pages are automatically available if you:

```bash
brew install opa
```

![Install OPA with homebrew and use 'man opa' to learn about it.](/img/blog/open-policy-agent-v0-19-release-921d49179440/2.gif)

_Install OPA with homebrew and use 'man opa' to learn about it._

[@jpeach](https://github.com/jpeach) submitted a number of patches that improve testing and support for the [http.send](https://www.openpolicyagent.org/docs/latest/policy-reference/#http) built-in function. For example, policies can now explicitly set TLS server names as well as certificates and keys when invoking the built-in function (previously they could only come from the environment or local files). This is useful if you want to specify those values in data or as local variables inside the policy itself.

Lastly, the release also includes a pointer to the new [rego-mode](https://github.com/psibi/rego-mode) Emacs package developed by [@psibi](https://github.com/psibi). The package provides syntax highlighting, formatting and more. In the future, the package could be extended to support many of the same features as the OPA extension for VS Code.

## WebAssembly Update

At KubeCon 2019 in San Diego we announced [support for compiling OPA policies into WebAssembly (Wasm)](/blog/opa-v0-15-1-rego-on-webassembly-81c226c51be4). Wasm enables OPA policies to execute in new environments like CDNs, service proxies and more without requiring an out-of-process RPC call to query OPA.

This week we are excited to release further support for Wasm in OPA with the new [golang-opa-wasm](https://github.com/open-policy-agent/golang-opa-wasm) project! This project wraps the [wasmerio/go-ext-wasm](http://wasmerio/go-ext-wasm) runtime library to provide convenient APIs for policy execution and more. The golang-opa-wasm SDK is still work-in-progress but feedback and contributions are welcome.
