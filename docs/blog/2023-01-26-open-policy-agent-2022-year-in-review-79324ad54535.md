---
title: "Open Policy Agent 2022, Year in Review"
authors: ["anderseknert"]
date: 2023-01-26
slug: open-policy-agent-2022-year-in-review-79324ad54535
---

![Banner image for OPA 2022 year in review blog post](/img/blog/open-policy-agent-2022-year-in-review-79324ad54535/banner.webp)

It's a new year, and once again it's time to look back and reflect on the year that passed in the world of Open Policy Agent! 2022 was OPA's first full year as a CNCF graduated project, and while it would be easy to think that things would slow down after reaching that point of maturity and recognition, things have rather sped up. In community growth as well as pace of development — both of which we'll take a closer look at here.

This year, tech communities like ours enjoyed finally getting to meet in person again, and major conferences in our space, like KubeCon / CloudNativeCon, saw thousands of attendees in both Europe and North America. We also saw meetups, hackathons and other smaller gatherings move back from virtual to in-person events. On the topic of events — let's start our review of the year 2022 for Open Policy Agent there.

## Notable Events

### 2022 User Survey

While we queried our nearest OPA instance for policy decisions, we also queried the OPA community to learn about their experience of interacting with the project, documentation and tooling. The [survey results](/blog/open-policy-agent-2022-user-survey-summary-370cf0243bb7) provided us valuable insights into where we might want to spend some extra time and effort in 2023 — richer documentation with more examples was requested by many. We're also seeing a need to better highlight the benefits of the various management capabilities in OPA, like decision logging and monitoring. An interesting trend which continues from 2022 is the increasing number of OPA use cases inside organizations. While infrastructure policies (including Kubernetes) still come out on top, we're seeing more organizations embrace OPA for policy across their whole stack. A standardized way of working with policy was always a goal of the project, so it's really exciting to see more organizations seeing the benefits of this approach!

### Open Policy Day with OPA

This year we were excited to see an entire event dedicated to OPA — the Open Policy Day with OPA co-located with KubeCon North America. During the course of the day, attendees got to hear end-user stories on using OPA in production, with speakers from organizations like Nvidia, T-Mobile. Capital One, Chime and Snowflake. If you couldn't attend in person, all the talks are up on [YouTube](https://www.youtube.com/@styra6251/videos)!

### Conferences and Meetups

In the cloud-native space, OPA was represented in talks at both KubeCon / CloudNativeCon [Europe](https://www.youtube.com/watch?v=MhyQxIp1H58&t=4s) as well as [North America](https://www.youtube.com/watch?v=RMiovzGGCfI). As the interest in policy as code grew over the year, we saw a number of talks, workshops and events focused on the topic, and many discovered the benefits of unified policy management across tech stacks and organizations.

Additionally — while OPA has been, and continues to be, a popular topic at cloud-native, security and DevOps themed meetups, it's great to see new meetup groups dedicated entirely to the topic of OPA. Several OPA meetups took place in 2022, and we'd love to see that trend continue in the new year! If you'd like to host your own, [let us know](https://www.openpolicyagent.org/community).

It was great to see so much buzz around OPA at these events, and much of that is thanks to the amazing work going on within the OPA projects — let's look into what's been going on there next!

## New Features

OPA and Rego saw a record number of new features added in 2022.

### New Keywords

A few new keywords made a big difference both to the aesthetics of Rego, as well as its functionality. First off, the new [`every` keyword](https://www.openpolicyagent.org/docs/v0.38.1/policy-language/#every-keyword) elegantly helps solve the problem of expressing "[for all](https://www.openpolicyagent.org/docs/latest/#for-some-and-for-all)" type of queries:

```rego
import future.keywords.every

only_dev_servers {
    some site in sites
    site.name == "dev"

    every server in site.servers {
        endswith(server.name, "-dev")
    }
}
```

Next, the new `if` keyword helps policy authors express their rules in the same way as they normally should be read — as conditional assignments: "allow is true if conditions x, y and z are true". As an added bonus, the `if` keyword allows skipping the braces around single-line rule bodies, leading to rule constructs that read more like plain English:

```rego
allow {
    "admin" in input.user.roles
}
```

May now be written as:

```rego
allow if "admin" in input.user.roles
```

Similarly, the new `contains` keyword allows set-building partial rules to be expressed as "the set **contains** x if conditions x, y and z are true":

```rego
deny[message] {
    input.user.security_clearance_level < 2
    message := "Security clearance level 2 or higher required"
}
```

May now be written as:

```rego
deny contains message if {
    input.user.security_clearance_level < 2
    message := "Security clearance level 2 or higher required"
}
```

### Refs in Rule Heads

It is said that one of the hardest things in computer science is naming things… "[refs in rule heads](https://github.com/open-policy-agent/opa/releases/tag/v0.46.1)" seems to support that claim! Don't let the name intimidate you though, this is a great addition to Rego! Dynamically creating deeply nested objects would previously often require the use of nested packages, with each package provided in its own file. This is no longer the case, as nesting can now be expressed in one place:

```rego
package policy

claims.user.name := concat(" ", [input.user.first_name, input.user.last_name])

request.method := input.request.method
request.valid := validate(input.request)
```

Evaluating the policy package will now provide something like the below result:

```json
{
  "claims": {
    "user": {
      "name": "John Doe"
    }
  },
  "request": {
    "method": "PUT",
    "valid": true
  }
}
```

### Built-in Function Metadata and Metadata Introspection

Another exciting addition to Rego this year was the introduction of [metadata annotations](https://www.openpolicyagent.org/docs/latest/annotations/). Previously, policy authors would have to devise custom methods and formats for annotating their policies, packages and rules with metadata in the form of comments. Native support for annotations now provides a unified way not only for authoring structured annotations, but also for parsing them using either the "opa inspect" command, programmatically from Go, or even from Rego policies themselves via the new built-in rego.metadata functions. Having a standardized way of annotating packages and rules with metadata should benefit both human consumers as well as tooling.

### Testing

A major advantage of treating policy as code is that code is testable. Unit testing provides effective guardrails around policies and rules, and allows frequent updates without the risk of breaking things. While test-driven development has been considered a best practice for Rego since the start, one feature that many found missing was the ability to mock functions. [Function mocking](https://www.openpolicyagent.org/docs/latest/policy-testing/#data-and-function-mocking) allows replacing built-in functions, like http.send, or custom functions, with different implementations, commonly free of side-effects, all using the same `with` keyword familiar to test authors.

### Policy and Data Distribution

Policy is commonly only half of the equation when OPA makes decisions — having access to up-to-date _data_ related to users, endpoints or resources is often just as important. Data tends to be more dynamic in nature than policy, and certain types of deployments require a **lot** of data. The combination of huge datasets and frequently changing data would previously require continuous transfer of all required data in a bundle — even when only a few attributes had been updated. The introduction of [delta bundles](https://www.openpolicyagent.org/docs/latest/management-bundles/#delta-bundles) solves this by providing a new bundle type containing only the changes to data made since the last fetch, i.e. the _delta_. A much-awaited feature, and one that will help ensure OPA covers even the most complex of use cases going forward.

Another introduction this year was support for [OCI](https://opencontainers.org/) [bundle registries](https://www.openpolicyagent.org/docs/latest/management-bundles/#oci-registry). In an increasingly containerized world, distribution of applications is no longer the only use case for containers, and providing policy and data using the same channels as you would for e.g. Docker images, simplify things considerably in many environments.

### Disk Storage

While delta bundles may solve the problem of _distributing_ large volumes of data, that data must eventually still be stored somewhere for OPA to make use of it. Up until this year, only a single option for storage was provided: in-memory. While this is normally the best place to store it for fast access, large datasets distributed in-memory across a large number of running instances quickly adds up, and many are those who learnt the hard way that the old slogan of "memory is cheap" isn't always true at scale. The [disk based storage](https://www.openpolicyagent.org/docs/latest/configuration/#disk-storage) option now provides a balance between performance and costs, and is a welcome addition to OPA in many types of integrations.

### Compiler Strict mode

The Topdown compiler in OPA was enhanced with a new [strict mode](https://www.openpolicyagent.org/docs/latest/strict/) option, allowing policy authors to catch common mistakes before deploying their policy to production. Unused imports and variables, or use of deprecated built-in functions — now all flagged by the compiler with strict mode turned on. Together with JSON schema-based [type checking](https://www.openpolicyagent.org/docs/latest/schemas/), developers are provided some powerful tools to ensure the robustness of their policy. Guard rails around the guard rails!

### Intermediate Representation (IR)

The OPA project has the ambitious goal of standardizing policy across the full stack, because of this it's sometimes necessary to consider alternative deployment models to the traditional standalone service. This year we saw OPA provide a new [intermediate representation](/blog/i-have-a-plan-exploring-the-opa-intermediate-representation-ir-format-7319cd94b37d) format, allowing custom implementations to parse and execute evaluation plans. An implementation for the JVM and Javascript has already been made available with the [Jarl](https://github.com/borgeby/jarl) project, and hopefully we'll see more to follow in 2023!

## Built-in functions

20 new [built-in functions](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions) got added in 2022 — more than any year before! We saw new functions in almost every existing category, and OPA's exciting new [GraphQL](https://www.openpolicyagent.org/docs/latest/graphql-api-authorization/) capabilities create a category of their own.

**GraphQL**: `graphql.is_valid`, `graphql.parse`, `graphql.parse_and_verify`, `graphql.parse_query`, `graphql.parse_schema`, `graphql.schema_is_valid`

**Strings**: `indexof_n`, `regex.replace`, `strings.any_prefix_match`, `strings.any_suffix_match`

**Objects**: `object.subset`, `object.union_n`, `object.keys`

**Crypto**: `crypto.hmac.md5`, `crypto.hmac.sha1`, `crypto.hmac.sha256`, `crypto.hmac.sha512`

**Misc**: `providers.aws.sign_req`, `net.cidr_is_valid`, `graph.reachable_paths`

## Performance Improvements

Some improvements are more understated, though no less noteworthy. As a mature software project, deployed in production in thousands of organizations across the world, OPA needs to be both robust and performant. This year saw the following exciting improvements in OPA performance:

- The — optimize flag now works for more commands (previously only available for `opa build`)
- Lazy objects optimization allows delaying evaluation of attributes until needed
- Built-in function optimizations for `object.get`, `in`, `object.union_n`, and others
- Two new highly optimized built-in functions for doing prefix and suffix matching _en masse_: `strings.any_prefix_match` and `strings.any_suffix_match`
- Internal optimizations to set element addition, object insertion and set union.

## Ecosystem and integrations

### Gatekeeper

A whole lot of great things landed in the Gatekeeper project this year! Following recent developments, Gatekeeper was made compatible with the Kubernetes v1.25 shift from Pod Security Policies to Pod Security Admission. On the topic of Kubernetes workloads, the new [Validation of Workload Resources](https://open-policy-agent.github.io/gatekeeper/website/docs/expansion) feature allows writing rules that apply to any Pod spec, whether deployed as a standalone pod or embedded in a parent resource, like a Deployment. Similarly, Gatekeeper now also allows validating subresources. The [external data feature](https://open-policy-agent.github.io/gatekeeper/website/docs/externaldata), which, as the name implies, allows Gatekeeper to interface with various external data sources for validation and mutation, moved to beta this year, and the next feature to do so is [Gator](https://open-policy-agent.github.io/gatekeeper/website/docs/gator), which allows testing of Gatekeeper ConstraintTemplates and Constraints in a local environment. Finally, the mutation feature is now considered stable.

In addition to all the features listed above, Gatekeeper is now faster than ever before! Some of the most notable improvements include reduced time for template compilation, adding and evaluating constraints, a whopping ~20X reduction in persistent audit memory usage, reduced request duration for policies with replicated data and reduced CPU time when adding data to OPA storage.

The ecosystem around Gatekeeper also saw improvements this year, where the most notable ones were the new Gatekeeper Policies [website](https://open-policy-agent.github.io/gatekeeper-library/website), and the inclusion of Gatekeeper policies on [ArtifactHub](https://artifacthub.io/packages/search?repo=gatekeeper-policies).

### Conftest

Conftest saw a rapid pace of development this year, with 12 releases pushed — from version 0.29.0, and ending in version 0.37.0! The project added support for policy authoring using a number of additional file types — like env, hcl, jsonc, CycloneDX, and SPDX — as input. Possibly even more exciting is the addition of several new built-in functions exclusive to Conftest, like parse_config, parse_config_file, and parse_combined_config. These functions all allow policy authors to pull in configuration to test from inside of a policy, allowing a greater deal of flexibility in how config tests are executed.

The tooling around Conftest improved as well: when using the `--version` flag, the version of OPA used by Conftest will now also be displayed. Additionally, the new `--quiet` flag allows excluding anything but errors in the output, which should help in quickly identifying issues.

### Integrations

OPA would not be what it is without its massive ecosystem of tools, integrations and useful and fun projects. The year started out with some great news in the infrastructure space, with AWS opening up for the possibility of externalizing compliance checks of CloudFormation templates via hooks, and it did not take long for the [AWS CloudFormation hook for OPA](https://github.com/StyraInc/opa-aws-cloudformation-hook) to arrive on the scene. Later this year, Hashicorp announced [support for OPA](https://developer.hashicorp.com/terraform/cloud-docs/policy-enforcement) in their Terraform Cloud offering. A [Pulumi](https://github.com/pulumi/pulumi-policy-opa) integration was also added to the [ecosystem](https://www.openpolicyagent.org/docs/latest/ecosystem/). The message seems clear — the tool to use for infrastructure as code (IaC) compliance is OPA, and the language to define IaC policies is Rego!

Outside of the infrastructure space, we saw a number of interesting integrations being built by the community, like [Alfred](https://github.com/dolevf/Open-Policy-Agent-Alfred), a Rego Playground you can self host, a [CircleCI](https://circleci.com/docs/config-policy-management-overview/) integration for CI/CD pipeline policies, [fig](https://github.com/open-policy-agent/contrib/tree/main/opa_fig_autocomplete) support for command line auto-completion goodness, [self-sovereign identity](https://docs.walt.id/v/ssikit/ssi-kit/open-policy-agent) (SSI) integrations, and even policy-driven access to remote systems via [SansShell](https://github.com/Snowflake-Labs/sansshell). OPA-powered policy enforcement even made it to the desktop this year, with the CISA-developed [ScubaGear](https://github.com/cisagov/ScubaGear/) project using Rego for validating M365 tenant configurations!

Finally, a much awaited addition to the OPA ecosystem — the [Rego Style Guide](https://github.com/StyraInc/rego-style-guide) now offers policy authors a comprehensive set of rules and best practices for authoring Rego.

For a more comprehensive list of OPA integrations, check out the OPA [ecosystem page](https://www.openpolicyagent.org/docs/latest/ecosystem/), and the [Awesome OPA](https://github.com/anderseknert/awesome-opa) list.

## Credits

None of the above would have been made possible without the amazing community around OPA. In 2022, we saw an incredible number of people contribute to the project in all imaginable ways — code, documentation, bug reports, support discussions, integrations and tools. OPA is being used more and more widely around the world, and in different domains. With this growth, it'd be easy to overlook the huge effort in growing a community to support the project. We know how hard the community has worked to get to where we are and for that we are immensely grateful. Thank you all for getting us to where we are today and laying the foundation for another fantastic year with Open Policy Agent.
