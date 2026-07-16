---
title: "Open Policy Agent 2020, year in review"
authors: ["anderseknert"]
date: 2021-01-04
slug: open-policy-agent-2020-year-in-review-dc25b60308d7
---

![Open Policy Agent 2020 year in review banner image](/img/blog/open-policy-agent-2020-year-in-review-dc25b60308d7/banner.webp)

## Introduction

To call 2020 an eventful year would be an understatement. With a pandemic raging globally, we saw the tech community quickly adapt by moving largely online. Working from home became the new norm as meetings, conferences and other events also moved to digital channels. With so much of our lives now taking place online, the importance of our online platforms became overwhelmingly apparent. As businesses and organizations worked hard to accommodate an ever increasing number of users, both interest and investments in securing their platforms followed.

2020 saw a big uptake in the adoption of OPA. As the de facto standard for cloud-native authorization, OPA has also found its way into many new and interesting domains. The [list of integrations](https://www.openpolicyagent.org/docs/latest/ecosystem/) has been constantly growing during the year, and many more are being worked on. Hundreds of new open source projects, businesses and organizations have come to rely on OPA as their open source unified policy framework across the stack. And, with the increasing adoption of OPA, we've seen the community grow with it — from the number of contributors to active users on Slack. The trajectory for next year has been set, and it truly looks great — for OPA and its community. Before we look ahead though, let's take some time to look back at what we achieved in 2020.

## OPA 2020 in numbers

- ~29 million downloads
- ~1500 new Github stars
- ~1700 new Slack users, doubled in size!
- ~750 commits
- 90 contributors
- 9 major versions released, 17 point releases
- 100th release pushed!
- 1000th issue closed!
- 10000 VS Code OPA plug-in installs!

## Notable features and enhancements

### Management

Several management capabilities were added. The [bundle signing](https://www.openpolicyagent.org/docs/latest/management/#signing) feature allows verifying the integrity of any bundle processed by OPA. Moreover, the new [bundle persistence](https://github.com/open-policy-agent/opa/releases/tag/v0.24.0) feature enables OPA to store a local copy of bundles received, which may be used in case the server is forced to restart while the configured bundle endpoints are unreachable. To authenticate at remote endpoints, several new credential providers were added, including:

- [GCP metadata token](https://www.openpolicyagent.org/docs/latest/configuration/#gcp-metadata-token) (thanks [@kelseyhightower](https://github.com/kelseyhightower)!)
- [AWS web identity](https://www.openpolicyagent.org/docs/latest/configuration/#using-eks-iam-roles-for-service-account-web-identity-credentials) (thanks [@richicoder](https://github.com/RichiCoder))!)
- [OAuth2](https://www.openpolicyagent.org/docs/latest/configuration/#oauth2-client-credentials)

For example, the following configuration instructs OPA to use the GCP Metadata Server to obtain credentials for downloading bundles from a remote endpoint (which happens to be hosted on Cloud Run):

```yaml
services:
  cloudrun:
    url: <BUNDLE_SERVICE_URL>
    response_header_timeout_seconds: 5
    credentials:
      gcp_metadata:
        audience: <BUNDLE_SERVICE_URL>
bundles:
  authz:
    service: cloudrun
    resource: bundles/http/example/authz.tar.gz
    polling:
      min_delay_seconds: 60
      max_delay_seconds: 120
```

Lastly, the decision logger now supports [mutating masks](https://www.openpolicyagent.org/docs/latest/management/#masking-sensitive-data) so that administrators can inject or overwrite (as opposed to just removing) fields inside decision logs.

### Tooling

The OPA binary had several new flags and features added. The most notable additions are probably the remodeled `opa build` command for creating bundles and the new `opa bench`/`opa test --bench` for [benchmarking policy evaluation](https://www.openpolicyagent.org/docs/latest/policy-performance/#benchmarking-queries).

### WebAssembly (Wasm)

Many improvements for Wasm were incorporated this year. Among the bigger ones we saw the opa build command get support for building Wasm modules from Rego policies, and the addition of an [SDK for Javascript](https://www.openpolicyagent.org/docs/latest/wasm/#javascript-sdk). Plenty of built-ins were implemented natively, and the status for Wasm support is now listed for each on the [policy reference](https://www.openpolicyagent.org/docs/latest/policy-reference/#built-in-functions) page.

### Performance

In the performance department we saw many improvements. The [comprehension indexing](https://www.openpolicyagent.org/docs/latest/policy-performance/#comprehension-indexing), which allows O(n) runtime complexity on "group-by" operations, is probably the one to stand out the most. Less visible, but of no less importance, a new parser was introduced, improving the internals of OPA and resulting in a 100x speedup on most .rego files!

### Rego & Built-in Functions

Built-in functions error handling [was remodeled](https://github.com/open-policy-agent/opa/releases/tag/v0.25.0) to allow policy authors to gracefully deal with errors like garbled input, which would previously halt policy evaluation in certain cases.

The new caching options for `http.send` introduced this year vastly increases its utility, allowing it to be used for things like OAuth2 and OpenID Connect metadata retrieval. The raise_error option allows policy authors to deal with errors in communication rather than having policy evaluation halt. Given the number of improvements made to the HTTP client this year — and the number of policies in where it is used — it makes the previous "experimental" label it carried less relevant, and the feature should be considered stable moving forward.

More than 30 new built-ins were added to OPA in 2020. These include functions for:

- JSON manipulation: `json.patch`, `json.remove`.
- Bitwise operations: `bits.or`, `bits.and`, `bits.negate`, `bits.xor`, `bits.lsh`, `bits.rsh`.
- Hex encoding: `hex.encode`, `hex.decode`.
- Hashing: `crypto.md5`, `crypto.sha1`, `crypto.sha256`.
- And a whole bunch of functions for validating various formats: `json.is_valid`, `yaml.is_valid`, `base64.is_valid`, `regex.is_valid`, `semver.is_valid`.
- Other useful built-ins added include `graph.reachable`, `object.get`, `numbers.range` and `semver.compare`.

The wide variety of these help push OPA as a true general purpose policy engine, further increasing the number of domains where Rego may be used. In addition to all the above, hundreds of bugs were fixed and tons of improvements to documentation were made. It's been an incredible year for the project.

## Ecosystem and integrations

### Gatekeeper

The Gatekeeper project took some great leaps forward this year. In terms of new features this meant, among other things, granular namespace exclusions (narrowing the scope of resources to present for audit, webhooks and sync), Helm 3 support and the Gatekeeper Pod Security Policies being referenced as a serious contender to the Kubernetes provided PSPs.

In terms of stability, Gatekeeper gained support for multi-pod deployments, completed a CNCF security review and had their first stable non-beta release pushed. Other notable enhancements include semantic logging (getting cluster wide resources in violation of policy from logs), standalone auditing and the dependency on finalizers removed.

Still in early alpha, we're seeing much anticipated support for mutating webhooks. Surely one thing to look out for in 2021!

If you haven't already, make sure to check out the [gatekeeper-library](https://github.com/open-policy-agent/gatekeeper-library) repo, which was moved out from the Gatekeeper repository this year and has seen continuous additions and improvements since then.

### Conftest

2020 was an exciting year in the development of the Conftest project. Having previously been maintained independently, this year we saw the project included in the larger OPA family of projects.

As for features, the already versatile tool gained support for a number of new input formats (VCL, XML, EDN, TOML, HOCON, Jsonnet) and a new output format (JUnit). Using conftest for Terraform policies became a common use case, and many improvements for Terraform and the HCL2 format got implemented this year. A new system for native plug-ins was added, as was support for loading data files alongside policies. Keeping conftest up to date on Linux was made easier than ever with both RPM and deb packages being made available. A new documentation site was launched at [https://www.conftest.dev/](https://www.conftest.dev/)

### OPA Envoy plugin

The OPA Envoy plugin now also supports the v3 transport API, as well as decoding gRPC payloads. Oh, and the project changed its name too — while Istio is very much still supported it is not limited to that implementation, and OPA Envoy plugin was deemed a better name than the OPA Istio plugin.

### IntelliJ IDEA plugin

Of all new integrations and projects worked on in the larger OPA ecosystem, the OPA plug-in for IntelliJ IDEA was probably the most anticipated one. After some time in development, a first release was announced this fall and proved to be well worth waiting for. Not only does it cover the necessities, like syntax highlighting, but integrates features for both evaluating rules as well as for running tests right from inside of the policy editor. If you haven't already, make sure to check it out in the IntelliJ plug-in marketplace or at the project [GitHub page](https://github.com/open-policy-agent/opa-idea-plugin).

![OPA Plugin for IntelliJ IDEA](/img/blog/open-policy-agent-2020-year-in-review-dc25b60308d7/2.webp)

## Credits

To all who engaged with or contributed to OPA and its ecosystem in 2020 — whether as [maintainers](https://github.com/open-policy-agent/opa/blob/master/MAINTAINERS.md), [contributors](https://github.com/open-policy-agent/opa/graphs/contributors), [integrators](https://www.openpolicyagent.org/docs/latest/ecosystem/), [adopters](https://github.com/open-policy-agent/opa/blob/master/ADOPTERS.md), users, or by helping others on Slack: You have all contributed not only to OPA but just as much to making this community be such a fun and rewarding place to be. Who knows what 2021 has in store for us? One thing however is for certain — with all the knowledge, skills and creative energy found in the OPA community, it will be no less eventful.

Thank you!
