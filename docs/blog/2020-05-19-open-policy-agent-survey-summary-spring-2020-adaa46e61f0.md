---
title: "Open Policy Agent Survey Summary (Spring 2020)"
authors: ["tsandall"]
date: 2020-05-19
slug: open-policy-agent-survey-summary-spring-2020-adaa46e61f0
---

![OPA Spring 2020 user survey summary banner](/img/blog/open-policy-agent-survey-summary-spring-2020-adaa46e61f0/banner.webp)

Last month we surveyed the OPA community to learn more about user adoption and help us plan and improve the OPA project. We received 204 responses (up 175% from the last survey in April 2019) from over 150 organizations, with 91% of respondents indicating they are in some stage of OPA adoption (i.e., experimentation, pre-production, production.) This post highlights what we learned from the survey results.

## Use Cases and Adoption

### Most organizations use OPA for multiple use cases

OPA's general-purpose, domain-agnostic architecture is paying off — 51% of respondent organizations use OPA for at least two use cases, with 29% using it for three or more use cases, such as Kubernetes Admission Control, Microservice API Authorization, Application Authorization, Cloud Security and so on. Similarly, only 15% of respondent organizations indicated they _only_ use OPA for Kubernetes Admission Control.

This data supports our own experience from working with users — the typical adoption path involves identifying OPA as a good solution for a specific problem. From there, users realize they can apply it elsewhere, across the stack. Going forward we will continue to focus on features and improvements that help solve a broad set of use cases.

| # of Use Cases | % of Orgs Using OPA |
| -------------- | ------------------- |
| 1              | 39%                 |
| 2              | 21%                 |
| 3              | 17%                 |
| 4              | 7%                  |
| 5 or more      | 6%                  |

### Application Authorization is becoming a dominant use case for OPA

43% of respondent organizations indicated they are in some stage of OPA adoption for Application Authorization. In the next survey, we will dedicate a larger section to this use case as it's clearly becoming another pillar for OPA (the other two being Kubernetes Admission Control and Microservice API Authorization). It's unsurprising that OPA is quickly being adopted for Application Authorization because the policies you need to enforce at the top of the stack are typically more sophisticated than those you enforce at the service/transport layer. Going forward, we will continue to add features that improve support for Application Authorization like data fetching, data filtering and UI preflight checks.

| Use Case                  | % of Orgs Using OPA for X |
| ------------------------- | ------------------------- |
| Kubernetes                | 54%                       |
| Application Authorization | 43%                       |
| Microservices             | 36%                       |
| Terraform                 | 25%                       |
| Data Stores               | 7%                        |
| Other                     | 17%                       |

### Production usage continues to grow

56% of respondent organizations use OPA in production for Kubernetes Admission Control and 47% use OPA in production for Microservice API Authorization. This is not particularly new, but it reinforces the fact that OPA is being used to solve policy, authorization and security use cases across the stack, in production. Going forward we will continue to focus on work that improves stability and performance. We also plan to define a support policy that clarifies what to expect in terms of backporting (e.g., we will guarantee to backport fixes to N-M releases when asked; backports to older requests will be best-effort.)

| Stage          | % of Orgs Using OPA |
| -------------- | ------------------- |
| Production     | 47%                 |
| Pre-production | 20%                 |
| Experimenting  | 24%                 |

### Microservice API Authorization requires scalable policy authoring

35% of respondent organizations use OPA to enforce (or plan to enforce) policies across more than 100 distinct microservices. This makes sense given that microservice architectures often align with organization boundaries. This implies that policy authoring and distribution need to be scaled across many teams. Going forward we will continue to develop tooling that helps scale the authoring and distribution process (e.g., code generating Rego boilerplate from Open API specifications, the new "opa build" command for producing bundles, etc.).

| # of Microservices | % of Orgs Using OPA for Microservices |
| ------------------ | ------------------------------------- |
| 1-10               | 13%                                   |
| 11-25              | 24%                                   |
| 26-50              | 21%                                   |
| 51-100             | 6%                                    |
| more than 100      | 35%                                   |

Policy _and_ runtime portability are important. The survey results showed that 57% of respondents use OPA for more than one Microservice API Authorization use case category (i.e., ingress, egress, or service-to-service) and 45% of respondents use more than one kind of OPA deployment architecture (e.g., library, sidecar, service). Moving forward, we will continue to invest in providing strong support for multiple deployment architectures.

| Microservices Use Case | % of Orgs Using OPA for Microservices |
| ---------------------- | ------------------------------------- |
| egress                 | 40%                                   |
| ingress                | 68%                                   |
| service to service     | 81%                                   |

| OPA Deployment Type | % of Orgs Using OPA for Microservices |
| ------------------- | ------------------------------------- |
| Go Library          | 37%                                   |
| Service             | 42%                                   |
| Sidecar             | 65%                                   |

## OPA Feedback

In addition to soliciting use case feedback, we also asked users to provide feedback on OPA features, Rego, gaps in the OPA ecosystem and so on. The results were positive and reinforce our effort to improve content on openpolicyagent.org.

### The Rego playground and testing support should be more prominent

65% of respondents said they use the Rego playground or features like policy testing that accelerate policy authoring. 10% were not aware that features like policy testing and coverage exist. Ideally, 100% of users would leverage the test framework, so moving forward, we'll focus on making testing more prominent and drive more users to try out playground and VS Code features, like interactive evaluation, that are invaluable during debugging.

### Rego's learning curve

52% of respondents said they are comfortable with Rego or "okay" with their skill level. 27% said they need occasional help. 16% say they struggle. **68% say they were able to learn Rego in under a week.** Given that Rego is based on programming language paradigms that are foreign to most developers, these numbers are understandable.

Going forward, we will continue to invest in answering questions on Stack Overflow and improving examples and documentation on the website. Interestingly, the number of people who struggle with Rego was twice as high among those that use OPA for Kubernetes and Terraform compared to Application Authorization. Perhaps this is (at least partially) due to the complex deeply-nested data structures that policies have to be expressed over within those environments.

### Examples are the biggest gap in the docs

The survey asked users: _"how can we improve the tutorials and documentation in OPA?"_ By far, the most common request is for **more policy examples**. Going forward we plan to focus on curating and organizing examples across a range of use cases, as well as building out dedicated sections for specific use cases like [application authorization and IAM](https://github.com/open-policy-agent/opa/issues/2098).

## Wrap Up

In the future, we plan to run the survey on a more regular basis with more consistent questions, so that we can compare historical results and observe trends over time. If you filled out the survey, thank you! We know you're excited to receive the t-shirts, and we're working on sending them your way! Due to the current situation, it might take a bit longer than we had expected. If you have not yet filled out the survey, you can still [fill out the survey](https://styra.typeform.com/to/QrYJh8). As always, if you have any questions or additional feedback, we're available on the Slack, GitHub, etc.

_…even doggos love OPA [credit: [@webbergs](https://twitter.com/webbergs), idea: [@the_dvorkin](https://twitter.com/the_dvorkin)]_
