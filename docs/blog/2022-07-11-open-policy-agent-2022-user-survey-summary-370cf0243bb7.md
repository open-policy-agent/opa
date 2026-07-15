---
title: "Open Policy Agent 2022 User Survey Summary"
authors: ["peteroneilljr"]
date: 2022-07-11
slug: open-policy-agent-2022-user-survey-summary-370cf0243bb7
---

![Banner image for the OPA 2022 user survey summary](/img/blog/open-policy-agent-2022-user-survey-summary-370cf0243bb7/banner.webp)

It's that time of year again! We have polled the Open Policy Agent (OPA) community to learn a bit more about what members are working on, their goals and how we can improve the project in the future. This year we had over 240 respondents, from various industries ranging from Software, Finance, E-commerce, Security, and more. With this new data set, we can learn if OPA usage has changed from the previous year, what features and tools are utilized the most and how to improve the OPA project as a whole for the community. To start, let's compare last year's survey results to this year's to see how things have changed or remained consistent:

> [Last Year's Survey](/blog/open-policy-agent-2021-survey-summary-e749bbd7b824)

## Year-over-year numbers

Within a couple of percentage points, the number of use cases and respondents' implementation goals show similar results to last year. Of the users that responded that they have over four use cases, 70% of those reporting have used OPA for a year or longer. This shows us that as OPA usage matures in an organization, users gain confidence in adding additional use cases, helping them achieve their higher-level goals.

![Table comparing 2022 vs 2021 use cases and implementation goals percentages](/img/blog/open-policy-agent-2022-user-survey-summary-370cf0243bb7/2.webp)

Almost 43% of respondents are in production with their OPA usage. This is a noticeable improvement from last year. With the addition of the Evaluating option, we can assume those users would have chosen experimentation given last year's choices, making the other two possibilities a few percentage points lower than the current year.

![Table comparing 2022 vs 2021 stage percentages](/img/blog/open-policy-agent-2022-user-survey-summary-370cf0243bb7/3.webp)

The last metric we highlighted in last year's survey is the time to production, showing that 40% of users reach production within six months. This year we are seeing about 27% of users in the production phase by this point. However, 50% of the users in this time frame are in a pre-production phase, which is a substantial amount.

![Table cross-tabulating how long respondents have used OPA against stage](/img/blog/open-policy-agent-2022-user-survey-summary-370cf0243bb7/4.webp)

## Policy libraries

We've seen a slight increase in usage for the Gatekeeper policy library from 57% to 62%, for the respondents that indicated they're using OPA for Kubernetes Admission Control. However, overall we are seeing 50% of respondents indicating that they are not using any external policy libraries. As policy libraries grow around specific use cases we can expect this number to increase.

## Feedback

Last year's request for better debugging tools led to creating two issues, rule-level tracing, and the print function. The Print Function was released in v0.34.0 and happily adopted by the community. Rule-level tracing still needs assistance from the community; perhaps you can help the community and submit a PR?

As we did with the previous year's survey, we asked the community for feedback to see what improvements would improve their OPA experience. The number one request was for more examples; nearly 33% of respondents asked for examples of specific or complex configurations and tutorials/sample data to go with them. About 12% of respondents asked for more integrations with AWS, such as the AWS CloudFormation integration that came out in June. And another 10% of users asked for additional debugging capabilities.

## Learning tools

The official OPA documentation is the most used resource by the community, with over 90% of respondents using it, followed by the Rego Playground at 66%. The OPA docs are consistently evolving and receive updates as new features roll out, but as with most open source projects, we need the community's help to keep the docs up to date. As for the Rego playground, we maintain this tool in the hopes that it helps users debug problems and collaborate on new policies. If you see any way that we can improve it, please let us know by creating a feature request.

## Monitoring

One surprising discovery from this year's responses is that 36% of users don't track OPA decisions, and 39% don't monitor their OPA status. While these metrics are accessible via OPA's management APIs, perhaps the docs can be spruced up with some new tutorials on configuring monitoring and logging!

## Wrap up

To sum it up, we saw consistency in the implementation goals and number of use cases for OPA with a slight uptick in the overall number of users in production. The utilization of policy libraries seems to have dropped to half of what it was last year. Debugging remains a high-priority area where users wish to see additional improvements, along with more examples and tutorials for the documentation. The OPA Docs and Rego playground take home the gold for most valuable resources, but they could use a few more examples to help community members configure monitoring and logging.

Thanks for your participation in this year's OPA User Survey. If you've sent us your mailing address, you can expect your t-shirt to arrive in your mailbox soon!

_photo credit Kayla_

_Happy OPA 2022 Survey from Charlie!_
