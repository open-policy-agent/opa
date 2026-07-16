---
title: "Open Policy Agent 2023, Year in Review"
authors: ["anderseknert"]
date: 2023-12-20
slug: open-policy-agent-2023-year-in-review-4c12df22e351
---

![Banner image for Open Policy Agent 2023 year in review post](/img/blog/open-policy-agent-2023-year-in-review-4c12df22e351/banner.webp)

As 2023 draws to a close, the time has come to reflect on another important year for Open Policy Agent (OPA). Now more than two years deep into CNCF Graduated status, OPA continues to see accelerated growth in production deployments — and across a diverse range of use cases. Such use cases demand both performance and stability, while user base growth depends on learning resources and ease of use. This year, the OPA community has worked hard and delivered on all fronts, for new and experienced users alike. This post takes time to share how this was achieved; highlight prominent events and updates; celebrate input from the wider community and set the scene for a historic year of OPA in 2024.

## OPA 'Away From Keyboard'

While OPA users and maintainers predominantly collaborate online, there were a good number of occasions where OPA existed very much in the physical realm this year too.

KubeCon EU enabled a few OPA events in Amsterdam early this summer. For the first time ever, an OPA-themed ContribFest session was held, where OPA, [Conftest](https://www.conftest.dev) and [OPA Gatekeeper](https://github.com/open-policy-agent/gatekeeper) maintainers worked with new contributors to the different OPA projects. In Amsterdam we also saw an OPA meet-up where speakers from [Miro](https://medium.com/miro-engineering/how-miro-leverages-open-policy-agent-to-implement-authorization-as-a-service-763f08469e5), [Bankdata](https://www.bankdata.dk) and [Styra](http://styra.com) presented. At this KubeCon EU there were four OPA talks:

- [The Compliance Business Case for Kubernetes in the EU: Anders Eknert](https://www.youtube.com/watch?v=XoWf4QcSbDw)
- [Open Policy Agent. (OPA) Intro & Deep Dive — Charlie Egan, Rita Zhang](https://www.youtube.com/watch?v=6RNp3m_THw4)
- [Calling OPA from eBPF, Through WASM, in the Kernel? You've Gone Mad! — Nandor Kracser](https://www.youtube.com/watch?v=JSKNch6piyY)
- [Scratching an Itch: Running Policy in Hard to Reach Places with WASM & OPA — Charlie Egan](https://www.youtube.com/watch?v=BdeBhukLwt4)

![Contribfest session at KubeCon EU in Amsterdam](/img/blog/open-policy-agent-2023-year-in-review-4c12df22e351/2.webp)

![OPA Meetup in Amsterdam hosted by Miro](/img/blog/open-policy-agent-2023-year-in-review-4c12df22e351/3.webp)

Rolling forward a few months, OPA also had a strong presence in Chicago at KubeCon NA. KubeCon is a huge event and it was great to get so many eyes on OPA as part of the graduated projects update in the keynote session. On top of that, the OPA kiosk in the project pavilion was an important meeting place for maintainers and users at the event. Discussions covered all sorts of use cases from authorization of applications, Kubernetes admission, IAC policy and beyond. Don't forget to check out the [OPA project update](https://www.youtube.com/watch?v=wJkjsvVpj_Q) from the conference's maintainer track.

![OPA Update on the big stage](/img/blog/open-policy-agent-2023-year-in-review-4c12df22e351/4.webp)

![Open Policy Agent kiosk in the project pavilion](/img/blog/open-policy-agent-2023-year-in-review-4c12df22e351/5.webp)

## From Strength to Strength

OPA grows in so many different ways each year it's sometimes hard to know how to quantify it. Here are some highlighted figures which illustrate OPA's trajectory as we enter 2024.

**2700 Contributors**. [Nearly 3000](https://opa.devstats.cncf.io/) people have helped make OPA into the project it is today. Contributors help make OPA better by making changes to docs and code; by participating in GitHub discussions and by filing bugs. What's equally impressive is how these contributors are from over 450 different companies. OPA is a general purpose, domain agnostic policy engine so it's vital the project is guided by such a varied contributor base.

**9 years** spent by users reading the documentation on the OPA website. This year OPA contributors worked hard and made over [160](https://gist.github.com/charlieegan3/533ca9795787265c8b536b32fd8e2c8b) updates to the docs; and so it's reassuring to look back at the end of the year and see just how many users benefited from the hard work.

**2000 Go repositories build on OPA**. [Integrating with OPA](https://www.openpolicyagent.org/docs/latest/integration/) has always been a priority so it's fantastic to see that just so many different projects are adding policy functionality in this way. With the [OPA SDK](https://pkg.go.dev/github.com/open-policy-agent/opa/sdk), it's possible to bring all the best parts of OPA right into your Go application making it a powerful tool when standardizing your policy as code stack.

**1.5 million Playground Runs.** The [Rego Playground](https://play.openpolicyagent.org) is for every OPA user, it's there as a learning tool, as a collaborative scratch pad and now also integrates the output from [Regal](https://docs.styra.com/regal), the new linter for Rego. On average, every 20s someone clicks the 'Evaluate' button on the playground, all day long, all year long. One of the major uses of the playground is for users and maintainers collaborating on support in the OPA Slack, if you're interested in getting help within your team or on the Slack, creating a minimal example on the playground is a place to start.

It's not just OPA's community that's moving forward in leaps and bounds, OPA itself has been keeping pace and has received loads of great updates this year too. Let's dig into that now.

## New Features

### General references in rule heads

The single most important addition to Rego this year was arguably general references in rule heads. Simply put, it is now possible to include variables in rule names (or "references"), making it possible to build complex, nested map structures which would previously require multiple rules distributed over several packages.

Example using dynamic policy composition to collect informative notices from all "rules" policies, and have them organized by category and title.

```rego
grouped_notices[category][title] contains notice if {
    some category, title
    rules_to_run[category][title]

    some notice in data.rules[category][title].notices
}
```

Output would be a nested structure, as expected:

```json
{
  "grouped_notices": {
    "testing": {
      "file-missing-test-suffix": ["ignored"]
    },
    "custom": {
      "naming-convention": ["ignored"],
      "one-liner-rule": ["obsolete", "ignored"]
    }
  }
}
```

For more examples and information, see the [OPA docs](https://www.openpolicyagent.org/docs/latest/policy-language/#rule-heads-containing-references) on the topic.

### Default keyword on functions

The default keyword has been around since forever, and is considered idiomatic for scenarios where a "fallback" value is needed, should rule evaluation fail in other rules sharing the same name. A long requested feature has been to extend support for default to cover custom functions, and 2023 was the year it [happened](https://www.openpolicyagent.org/docs/latest/policy-language/#default-keyword).

```rego
package functions

default first_name(_) := "unknown"

first_name(full_name) := split(full_name, " ")[0]
```

### New built-in functions

Seven new built-in functions were added to Rego this year. The [json.verify_schema](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-object-jsonverify_schema) and [json.match_schema](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-object-jsonmatch_schema) functions are both recent additions for evaluating policy against JSON schemas — a use case that's been increasingly common in recent times. The [time.format](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-time-timeformat) function will help policy authors present dates and time using either a custom format, or one of the supported constants for [datetime formats](https://www.openpolicyagent.org/docs/latest/policy-reference/#timestamp-parsing) that were also added this year. Three new crypto functions were added: [crypto.hmac.equal](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-crypto-cryptohmacequal), [crypto.x509.parse_keypair](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-crypto-cryptox509parse_keypair), and [crypto.parse_private_keys](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-crypto-cryptoparse_private_keys). Finally, the new [numbers.range_step](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-numbers-numbersrange_step) function, which works just as numbers.range, but with a configurable step value.

### Package scoped annotations

A metadata annotation using the scope of package is now truly [scoped to the package](https://github.com/open-policy-agent/opa/releases/tag/v0.50.0), and not just the file in which it is declared. This allows for some interesting opportunities to separate metadata declarations from "implementing" packages, and build things like lightweight frameworks leveraging Rego's metadata annotations. Additionally, it'll allow defining package scoped annotations for "packages" created via general references in rule heads, where the package scope isn't directly allowed on the rule itself.

### Debugging

The Swiss army knife of OPA also known as "opa eval" got a new flag to help debugging policy this year. Using the `--show-builtin-errors` flag, policy authors may now get a list of all errors produced by built-in functions as part of evaluation, making it much faster to identify certain types of problems.

### Performance

OPA keeps getting faster, and in 2023 we saw some great improvements in this area. The [json.patch](https://www.openpolicyagent.org/docs/latest/policy-reference/#builtin-object-jsonpatch) built-in function was remodeled entirely and now performs extremely well even when provided with a huge list of changes. Performance isn't entirely in the hands of OPA though. Some [optimizations](https://www.openpolicyagent.org/docs/latest/policy-performance/) can only be performed at the level of an actual Rego policy, and OPA provides several tools to help policy authors with this. The [profiler](https://www.openpolicyagent.org/docs/latest/policy-performance/#profiling) (`opa eval --profile`) is one such tool, and from this year it'll now also include the number of generated expressions in evaluation. This helps policy authors better understand why some expressions are evaluated more times than what one might expect.

### Server

Several improvements to the server (and by extension, the [OPA SDK](https://www.openpolicyagent.org/docs/latest/integration/#integrating-with-the-go-sdk)) landed in OPA this year. Bundle fetching now works with [AWS Signing Version 4A](https://www.openpolicyagent.org/docs/latest/configuration/#aws-signature), allowing bundles hosted on AWS to be distributed across different geographical regions. Also, a new shorthand format for quickly running the server pointed at a remote bundle was [introduced](https://github.com/open-policy-agent/opa/releases/tag/v0.50.0). The OCI downloader saw several [new authentication methods](https://www.openpolicyagent.org/docs/v0.52.0/configuration/#using-private-image-from-oci-repositories) added. Finally, instance [labels](https://www.openpolicyagent.org/docs/v0.52.0/management-discovery/#limitations) may now be added via discovery allowing for greater flexibility in runtime re-configuration of long running OPAs.

### Monitoring

Given the large number of OPA instances running in production, having a good story around monitoring is essential. The [status API](https://www.openpolicyagent.org/docs/latest/management-status/#status-service-api) provides a way for any OPA deployed to report its current status to a centralized control plane or monitoring system. In 2023, several new metrics got added to the status reports, including most notably the request count for unauthorized calls to the OPA REST API when an [authentication/authorization policy](https://www.openpolicyagent.org/docs/latest/security/#authentication-and-authorization) is in use, as well as errors that might have happened in [decision logging](https://www.openpolicyagent.org/docs/latest/management-decision-logs/).

### Security

2023 was the year the OPA docker images finally made rootless the default, and the special "-rootless" images that previously existed for this purpose are now obsolete. If you're still using them, make sure to remove the suffix from the image on your next version upgrade!

## Ecosystem

### Gatekeeper

The OPA Gatekeeper project had a busy 2023, with many improvements landing this year. The external data feature allows users to connect with external data sources as part of policy evaluation. This year it gained support for caching of responses from external data providers for both audit and admission. A new AssignImage mutator which enables mutation of image registry or tag was also made available. The new PubSub feature (currently in alpha) enables users to subscribe to pubsub services to consume a large number of audit violations. Additionally, observability statistics for admission, audit and gator CLI are now available!

Speaking of the Gator CLI — the tool now prints violating object names on test output, and additionally supports trace and image flags. It may now also be provided an AdmissionReview object for verification.

Using the new (experimental) Kubernetes Native Validation feature, users can now write CEL (Common Expression Language) based rules in addition to Rego rules in constraint templates, similar to Kubernetes ValidatingAdmissionPolicy. Finally, the ExpansionTemplate feature, which enables validation of workload resources, has graduated to beta.

### Conftest

The Conftest project saw many improvements around tooling this year. A new `--strict` flag was added to the verify and test commands, which will enforce additional safety checks on the policies such as unused arguments, duplicate imports, [and more](https://www.openpolicyagent.org/docs/latest/policy-language/#strict-mode). Two more flags got added: the `--quiet` flag to the verify command which will silence success notifications and only show errors, and a `--config` flag which allows users to specify where the config file to be tested lives. Test results may now also be emitted in a format compatible with Azure DevOps. On the topic of formats, a new input format was added to the already long list of supported ones, and [textproto](https://protobuf.dev/reference/protobuf/textformat-spec/) files may now be targeted for policy evaluation too. Finally, the Confest Docker images now also support both the `linux/amd64` and `linux/arm64` platforms.

### OPA Ecosystem

One goal of the OPA project is to build a domain agnostic policy engine. Being domain agnostic is achieved by simultaneously building generic core policy functionality, while also supporting a range of out-of-the-box integrations for different use cases. This year, the wider OPA community has wholeheartedly delivered on the latter and listed 22 new integrations on the website. The OPA Ecosystem also has a new home as a top level page, where integrations can be browsed by category, [check it out](http://openpolicyagent.org/ecosystem/)!

![New OPA Ecosystem Showcase](/img/blog/open-policy-agent-2023-year-in-review-4c12df22e351/6.webp)

Most new ecosystem additions this year have been with other open source tools, generally adding policy functionality to a larger tool or leaning on Rego to provide a solid foundation for a domain-specific policy tool. Some notable examples include:

- Source Code Management: [Reposaur](https://www.openpolicyagent.org/integrations/reposaur/), a repository compliance tool; and [Legitify](https://www.openpolicyagent.org/integrations/legitify/), a repository security configuration scanner.
- Supply Chain Security: [dependency-management-data](https://www.openpolicyagent.org/integrations/dependency-management-data/), helps understand software dependency posture; and [Enterprise Contract](https://www.openpolicyagent.org/integrations/enterprise-contract/) verifies supply chain security artifacts with Rego policy.
- Infrastructure CD checks: [Torque](https://www.openpolicyagent.org/integrations/torque/), [Spinnaker](https://www.openpolicyagent.org/integrations/spinnaker-pipeline/) integrate Rego-based checks for continuous deployment while [ccbr](https://www.openpolicyagent.org/integrations/wirelesssecuritylab/) and [​​BrainIAC](https://www.openpolicyagent.org/integrations/carbonetes/) support a range of checks on existing IAC codebases.
- Extending Authorization with OPA: The data orchestration tool [Alluxio](https://www.openpolicyagent.org/integrations/alluxio/) now also supports delegation of permissions to OPA.

[Digger](https://www.openpolicyagent.org/integrations/digger/), an open source CI/CD orchestrator for Terraform both integrates OPA for user RBAC and leaning into existing tooling by leveraging OPA project [conftest](http://conftest.dev) for IAC policy.

Meanwhile, other integrations went deeper and applied Rego in previously unexplored ways. [regocpp](https://www.openpolicyagent.org/integrations/regocpp/) is a cutting-edge project from collaborators at Microsoft that aims to bring Rego to other environments, natively. Based on C++, regocpp supports a number of Rego built-ins and the grammar as of v0.55.0.

The aforementioned linter, [Regal](https://www.openpolicyagent.org/integrations/regal/) also pushes the boundaries of where Rego can be used to write policies. Using the JSON representation of the Rego abstract syntax tree, this project implements a range of [linting rules](https://docs.styra.com/regal/category/rules)… in Rego! Regal has already been deployed by a number of open source Rego policy libraries and now supports over 60 rules. Integrated with the [Rego Playground](https://play.openpolicyagent.org) the linter is already available to everyone. There's no doubt that this will be a great tool for OPA learners and long-timers alike while continuing to help [scale](https://thenewstack.io/scaling-open-source-community-by-getting-closer-to-users/) the OPA community.

If you're interested in listing your OPA integration or project, please see [the instructions](https://github.com/open-policy-agent/opa/tree/main/docs#opa-ecosystem) or stop by the #ecosystem channel in the [OPA slack](https://communityinviter.com/apps/openpolicyagent/signup) if you have any questions.

## Thanks

2023 was an exciting year for OPA and its community. With so many projects using, integrating or extending OPA for all sorts of use cases — and so many users helping to contribute in all sorts of ways — this community is truly a great place to be. Thank you all who helped make it so! Your efforts are seen and appreciated.

There's a lot of great stuff lined up for next year already, so buckle up, and let's `import future.2024`!

Special thanks to Charlie Egan, Rita Zhang and John Reese for having helped contribute to this blog.
