---
title: "Open Policy Agent 2021 Survey Summary"
authors: ["tsandall"]
date: 2021-08-31
slug: open-policy-agent-2021-survey-summary-e749bbd7b824
---

![OPA 2021 community survey results banner](/img/blog/open-policy-agent-2021-survey-summary-e749bbd7b824/banner.webp)

_…happy OPA 2021 survey from Cal [credit: @eileen_kemp]_

Last month we surveyed the OPA community to learn more about user adoption and help us plan and improve the project. We received over 300 responses from users across financial services, healthcare, public sector, automotive, cloud technology providers and more. This post highlights some of the survey results.

## Use Cases and Adoption

### OPA adoption driven by authorization use cases across the stack

Like last year, we used the survey to gauge use case adoption among respondents. We're interested in understanding where and why companies are deploying OPA because it helps us steer the project's long-term roadmap in the right direction. This year we asked respondents about the high-level goals they're trying to achieve by using OPA. We found that implementation of internal compliance and governance rules was the most common goal, however, nearly 60% of respondents indicated two or more goals being highly relevant.

| Goal                            | % of Respondents |
| ------------------------------- | ---------------- |
| Internal compliance/governance  | 64%              |
| Operational excellence          | 49%              |
| Implementing end-user IAM       | 44%              |
| External compliance (e.g., PCI) | 28%              |

In terms of use cases (e.g., Kubernetes admission control, Microservice authorization, etc.), the results were similar to the previous year with 50% of respondents indicating they use OPA for two or more use cases:

| # of Use Cases | % of Respondents |
| -------------- | ---------------- |
| 1              | 48%              |
| 2              | 34%              |
| 3              | 13%              |
| 4+             | 3%               |

Kubernetes admission control continues to be the most common use case for OPA with 54% of respondents indicating they run OPA or OPA Gatekeeper to enforce various policies on their clusters:

| Use Case                     | % of Respondents |
| ---------------------------- | ---------------- |
| Kubernetes admission control | 54%              |
| Application authorization    | 39%              |
| Microservice authorization   | 39%              |
| Terraform validation         | 25%              |
| Other                        | 5%               |

### From experiments to production in 6 months (or less)

The survey showed the distribution of respondents OPA usage maturity was roughly equal:

| Stage               | % of Respondents |
| ------------------- | ---------------- |
| Experimentation     | 33%              |
| Pre-production & QA | 32%              |
| Production          | 35%              |

What was more interesting was that about half of respondents indicated they had only been using OPA since January 2021. Of those users, nearly 40% had already reached production. Furthermore, the survey results show that most respondents reached production within 6 months. Beyond that, the percentage of users that are still in experimental stages drops to single digits:

|                | Experimentation | Pre-prod/QA | Production |
| -------------- | --------------- | ----------- | ---------- |
| < 3 months     | 54%             | 28%         | 14%        |
| 3-6 months     | 22%             | 54%         | 25%        |
| 6-12 months    | 4%              | 38%         | 58%        |
| Over 12 months | 7%              | 15%         | 76%        |

These results are encouraging and also give us high-level metrics to improve on — ideally the time to production with OPA will continue to decrease as we improve the user experience and harden the project.

The survey results also highlighted a range of deployment sizes for production users. The following chart breaks down the deployment size responses by use case:

| Use Case                     | &lt;10 | 10-50 | 50-200 | &gt;200 |
| ---------------------------- | ------ | ----- | ------ | ------- |
| Kubernetes admission control | 42%    | 31%   | 13%    | 12%     |
| Terraform validation         | 43%    | 25%   | 18%    | 12%     |
| Microservice authorization   | 37%    | 37%   | 12%    | 11%     |
| Application authorization    | 44%    | 32%   | 10%    | 11%     |

### Policy library adoption is growing

The survey asked users about various features in OPA and one of the most encouraging bits of information was that policy library adoption is growing within platform authorization use cases, like Kubernetes admission control and Terraform plan validation. Specifically, we found that nearly 60% of Kubernetes admission control users rely on the official [gatekeeper-library](https://github.com/open-policy-agent/gatekeeper-library) policies that implement various best practices as well as PSP. We also found that nearly 30% of users that run OPA to validate Terraform plans rely on various open source policy libraries.

## OPA Feedback

In addition to gauging adoption we also used the survey to solicit feedback about the project.

### Debugging needs some love

After poring over the feedback comments, we found that the most common area for improvement is _debugging_. As with all surveys, some comments were non-specific, however multiple respondents requested better [tracing modes](https://github.com/open-policy-agent/opa/issues/2089) and explanation presentation formats. Improved [debug output](https://github.com/open-policy-agent/opa/issues/3319) support was another common request, and respondents also mentioned a desire for an [interactive debugger](https://github.com/open-policy-agent/opa/issues/3191) similar to what you find in typical programming languages.

### SDKs for various languages

Aside from debugging, the next most common request was better SDK support for OPA in various languages. Several respondents indicated interest in [Wasm-based SDKs](https://www.openpolicyagent.org/docs/latest/wasm/) for OPA, and others requested regular SDKs for Java, NodeJS and other languages. One of the reasons we haven't developed SDKs for OPA yet is because the OPA API is _extremely simple_ (e.g., you can query OPA for decisions with a single HTTP POST request). However, with the Wasm compiler in OPA improving with every release, and the Wasm ecosystem growing rapidly, it feels like it's time to invest into language-specific integration libraries.

## Wrap Up

Thanks to everyone who completed the survey! The OPA t-shirts for completing the survey will be shipped soon. If you have not filled out the survey but would like to do so, you can still [complete the OPA survey](https://form.typeform.com/to/pL0jDuyT). As always, if you have questions or feedback, we're available on Slack, GitHub, etc.
