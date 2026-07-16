---
title: "Announcing OPA 1.0: A New Standard for Policy as Code"
authors: ["charlie"]
date: 2024-12-20
slug: announcing-opa-1-0-a-new-standard-for-policy-as-code-a6d8427ee828
---

![Banner graphic announcing the OPA 1.0 release](/img/blog/announcing-opa-1-0-a-new-standard-for-policy-as-code-a6d8427ee828/banner.webp)

We are excited to announce OPA 1.0, a milestone release consolidating an improved developer experience for the future of Policy as Code. After nearly 10 years of innovations and contributions from over 450 developers, OPA 1.0 is finally here. The release makes new functionality designed to simplify policy writing and improve the language's consistency the default. This release marks the beginning of a new era for our project and represents a robust foundation for Policy as Code projects in the years ahead.

Since the project's [CNCF graduation](https://www.cncf.io/announcements/2021/02/04/cloud-native-computing-foundation-announces-open-policy-agent-graduation/) at the start of 2021, our work has been focused on Rego's developer experience and consistency. While much of the new functionality has been around for some time, OPA 1.0 will make new features the default — warranting the [SemVer](https://semver.org/) bump to `v1.0.0`. At this time, we would like to take the chance to highlight the following key changes to the defaults in Rego v1:

- Using `if` for all rule definitions and `contains` for multi-value rules is now mandatory, not just when using the `rego.v1` import.
- Other new keywords (`every`, `in`) are available without any imports.
- Previously requirements that were only run in "strict mode" (like `opa check --strict`) are now the default. Duplicate imports and imports which shadow each other are no longer allowed.
- OPA 1.0 comes with a range of backwards compatibility features to aid your migrations, please see the [v0 compatibility guide](https://www.openpolicyagent.org/docs/latest/v0-compatibility/) if you must continue to support v0 Rego.

Many users have already started using the new syntax (we started adding the new keywords three years ago back in v0.34.0) but for those who haven't; in order to get the best of OPA 1.0; and to remain abreast of follow on updates; users are encouraged to upgrade as soon as possible (using the backwards compatibility functionality if required). Most users will be able to update their Rego quickly as the process can be largely automated using the Rego tools already built into OPA. The process can be summarized as follows:

- Download and install an [OPA 1.0 binary](https://github.com/open-policy-agent/opa/releases/tag/v1.0.0) before running the following commands.
- `opa check --v0-v1`: Find parser and compiler errors that might be present in old code.
- `opa check --v0-v1 --strict`: Find problems in Rego code no longer permitted in OPA 1.0.
- `opa fmt --write --v0-v1`: Automatically update code to the OPA 1.0 syntax.
- `regal lint`: using Regal, the linter for Rego is also recommended to find bugs and performance issues.

For users looking for more detailed information, please see the following resources: [OPA Documentation on upgrading to 1.0](http://openpolicyagent.org/docs/latest/v0-upgrade/); the [Renovating Rego](https://www.styra.com/blog/renovating-rego/) post for older projects which digs into the above process in detail; the [Maintainer Track presentation](https://youtu.be/QuotLxFb2f4?feature=shared&t=800) from KubeCon NA 2024 for a video overview.

Those [integrating with OPA's Go packages](https://www.openpolicyagent.org/docs/latest/integration/) — both via the SDK and the low-level Rego package — are encouraged to update their applications to use the new v1 packages. This is also documented in the [upgrading documentation](http://openpolicyagent.org/docs/latest/v0-upgrade/) and is a straightforward process. In some cases, users or integrators will need to support both v0 and v1 Rego simultaneously in the same application. This is generally only applicable to those offering OPA as part of a managed offering where the Rego is controlled by end users. Those who do have this use case, please review the [v0 compatibility guide](https://www.openpolicyagent.org/docs/latest/v0-compatibility/) to review the most suitable option for your application.

One last thing, while OPA 1.0 is primarily about consolidating the Rego developer experience, the release also comes with some significant improvements to performance. Check the [release notes](https://github.com/open-policy-agent/opa/releases/tag/v1.0.0) to dig into the details.

And that's a wrap! OPA 1.0 is here, it's a milestone release for our project and we're proud of the effort this release represents. We're excited for you to upgrade and experience these improvements firsthand. As you do, please report any issues via [Slack](http://slack.openpolicyagent.org/) or [GitHub Discussions](https://github.com/orgs/open-policy-agent/discussions), and feel free to open bug reports or feature requests to help shape future releases. Also, don't forget to use the [Regal language server](https://docs.styra.com/regal/editor-support) to ensure your Rego code is efficient & error-free too.

We want to extend our gratitude to our incredible community for always testing the latest versions, reporting bugs, contributing code and supporting fellow community members on their Rego journeys. We're not done yet and look forward to working together for years to come.

_Happy holidays — the OPA team_
