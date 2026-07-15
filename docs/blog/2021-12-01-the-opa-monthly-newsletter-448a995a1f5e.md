---
title: "OPA Newsletter: November 2021"
sidebar_label: "November Newsletter"
authors: ["peteroneilljr"]
date: 2021-12-01
slug: the-opa-monthly-newsletter-448a995a1f5e
---

_November Edition_

## Intro

Hello everyone and welcome to the very first edition of the OPA Monthly Newsletter! We are excited to bring you all the happenings in the OPA ecosystem. You can expect to find a bit of everything in this newsletter, some community updates, a bit of contributor news, a handful of release notes, and any interesting content we've found on the internet this month.

## Slack Updates

![OPA Slack community member growth screenshot](/img/blog/the-opa-monthly-newsletter-448a995a1f5e/2.png)

Our Slack Org now hosts over 5,150 OPA community members!! The OPA team has been hard at work revamping the space to make it functional and valuable for all of our members. A little while ago you may remember we announced a [Slack Reorg](/blog/opa-slack-tune-up-b3c52492e2fc) to consolidate and update channel names and descriptions. This effort was to give everyone a clear understanding of what's going on and where to go.

To continue to improve the Slack experience for our members we've added 2 new channels. For everyone interested in contributing to the OPA project you can now hang out in the [#development](https://openpolicyagent.slack.com/archives/C02L1TLPN59) channel to speak directly with other contributors and maintainers.

We've also added a [#vendor](https://openpolicyagent.slack.com/archives/C02J6LBL6GH) channel to allow members to reach out directly to our rich ecosystem of vendors that are building products on top of OPA. Jump into the channel today and ask questions about how to improve your OPA management.

## News Highlights

One of our community members @boranx shared with the community that Conftest has made it into the [Technology Radar by ThoughtWorks](https://www.thoughtworks.com/radar/tools/conftest)

## GitHub Updates

The OPA project wouldn't be the same without all of the contributions from the community. As such we would like to send a big thank you to all of the contributors from the v0.34 release.

- Edward Paget has contributed ([#3826](https://github.com/open-policy-agent/opa/issues/3826) SDK Feat) & ([#3863](https://github.com/open-policy-agent/opa/issues/3863) Bundles Fix)
- Kirk Patton a long time contributor added ([#3773](https://github.com/open-policy-agent/opa/issues/3773) Fix for exit statuses)
- GitHub User [@0xAP](https://github.com/0xAP) first time contributor added ([#3860](https://github.com/open-policy-agent/opa/issues/3860) Bundles improvement)
- Andreas Brehmer first time contributor added ([#3836](https://github.com/open-policy-agent/opa/issues/3836) Fmt fix)
- Florian Gasc first time contributor added ([#3879](https://github.com/open-policy-agent/opa/issues/3879) Storage fix)
- Omolola Olamide has landed ([#3910](https://github.com/open-policy-agent/opa/issues/3910) Tutorial Updates)

## Twitter Highlights

For those not active on Twitter, we've collected some of the highlights and OPA shoutouts here:

[https://twitter.com/that_tech_tea/status/1451930146835861504](https://twitter.com/that_tech_tea/status/1451930146835861504)

![Screenshot of that_tech_tea tweet mentioning OPA](/img/blog/the-opa-monthly-newsletter-448a995a1f5e/3.jpeg)

[https://twitter.com/nusairat/status/1458815340985520130](https://twitter.com/nusairat/status/1458815340985520130)

![Screenshot of nusairat tweet mentioning OPA](/img/blog/the-opa-monthly-newsletter-448a995a1f5e/4.jpeg)

[https://twitter.com/nmeisenzahl/status/1458419364433117184](https://twitter.com/nmeisenzahl/status/1458419364433117184)

![Screenshot of nmeisenzahl tweet sharing OPA slides](/img/blog/the-opa-monthly-newsletter-448a995a1f5e/5.jpeg)

Check out the slides and demos that [Nico Meisenzahl](https://twitter.com/nmeisenzahl) created:

- [enhance-your-compliance-and-governance-with-policy-based-cicd](https://www.slideshare.net/nmeisenzahl/continuous-lifecycle-enhance-your-compliance-and-governance-with-policybased-cicd)
- [demo-opa-terraform-validation](https://gitlab.com/nico-meisenzahl/demo-opa-terraform-validation)
- [demo-opa-cicd-validation](https://github.com/nmeisenzahl/demo-opa-cicd-validation)

## Ecosystem Updates

The OPA Project is always changing, check out the latest updates and features for OPA and some of the sub-projects.

### [OPA Release v0.35.0](https://github.com/open-policy-agent/opa/releases/tag/v0.35.0)

- Early Exit Optimization improves performance in many policy types
- New net.lookup_ip_addr built-in function to resolve host IP addresses
- Massive performance improvement in decision logging compression

### [OPA Release v0.34.0](https://github.com/open-policy-agent/opa/releases/tag/v0.34.0)

- A new in operator for checking membership and for iteration
- New [print](/blog/introducing-the-opa-print-function-809da6a13aee) function for debugging
- New opa inspect command for quickly checking contents of a [bundle](https://www.openpolicyagent.org/docs/latest/management-bundles/)

### [Gatekeeper Release v3.7.0](https://github.com/open-policy-agent/gatekeeper/releases/tag/v3.7.0)

- Mutation has graduated to Beta! 🎉
- Added ModifySet mutator 📐

### [Conftest Release v0.28.3](https://github.com/open-policy-agent/conftest/releases/tag/v0.28.3)

- The OPA [print](/blog/introducing-the-opa-print-function-809da6a13aee) function is now supported in Conftest!

### [Kube-mgmt Release v3.1.0](https://github.com/open-policy-agent/kube-mgmt/releases/tag/3.1.0)

- Support extra environment variables in opa and kube-mgmt containers

## Community Spotlights

![Developer-Guy community spotlight for Cosign OPA integration](/img/blog/the-opa-monthly-newsletter-448a995a1f5e/6.png)

- The one and only [Developer-Guy](https://github.com/developer-guy) has been working tirelessly to add OPA policy functionality to [Cosign](https://github.com/sigstore/cosign), Check out the [PR](https://github.com/sigstore/cosign/pull/641) to see the awesome work to connect the two projects.

## What happened this month?

- Meetup: [OPA London Meetup](https://www.meetup.com/london-opa-meetup/events/281522329)
- Meetup: [OPA Stockholm Meetup](https://www.meetup.com/stockholm-opa-meetup/events/281066231/)
- Talk: [WTF is Cloud Native talk](https://www.youtube.com/watch?v=RwsyMLyl8O0)
- Talk: [API Authorization with Open Policy Agent](https://www.infracloud.io/cloud-native-talks/api-authorization-with-open-policy-agent-opa/)
- Blog: [Connecting OPA with AWS Lambda](/blog/serverless-policy-enforcement-connecting-opa-and-aws-lambda-e624f7176a3)
- Blog: [Automated Manifest File Validation Using Open Policy Agent and GitHub Actions](https://medium.com/@ravindursr/automated-manifest-file-validation-using-open-policy-agent-and-github-actions-697fa9fd74f0)

## What's coming up next month?

A list of community meetings, meetups, and conferences.

### [OPA Bi-Weekly](https://docs.google.com/document/d/1v6l2gmkRKAn5UIg3V2QdeeCcXMElxsNzEzDkVlWDVg8/edit?usp=sharing)

- Dec 7th at 10 AM PT
- Dec 21st at 10 AM PT

### [Gatekeeper Weekly](https://docs.google.com/document/d/1A1-Q-1OMw3QODs1wT6eqfLTagcGmgzAJAjJihiO3T48/edit)

- Dec 2nd, 2 PM PT
- Dec 8th, 9 AM PT
- Dec 15th, 2 PM PT
- Dec 22nd, 9 AM PT

## Let us know how we did

This was our very first edition of the OPA Newsletter, we really hope you enjoyed it! While we tried our best to find all the latest and greatest activities in the community we surely missed a lot as well. Want to share some cool content, have an OPA shoutout to make, want to speak at a conference, or host a meetup? Let us know by sending an email to: [opa_newsletter@styra.com](mailto:opa_newsletter@styra.com).

If you're new to OPA or to the community check out these community resources to get started.

- Chat with the community on [Slack](https://slack.openpolicyagent.org/)
- Ask for help and support on [GitHub Discussions](https://github.com/open-policy-agent/feedback/discussions)
