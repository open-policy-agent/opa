---
title: "Note from Teemu, Tim, and Torin to the Open Policy Agent community"
authors: ["timhinrichs"]
date: 2025-08-20
slug: note-from-teemu-tim-and-torin-to-the-open-policy-agent-community-2dbbfe494371
---

![Banner announcing OPA creators joining Apple](/img/blog/note-from-teemu-tim-and-torin-to-the-open-policy-agent-community-2dbbfe494371/banner.webp)

Today we're excited to announce that the creators of Open Policy Agent (along with many team members from Styra) have joined Apple to continue our decade-long mission of delivering an open source solution to unifying policy across the cloud-native stack.

Apple is an enthusiastic OPA user, utilizing it as a key component of its authorization infrastructure to manage a vast portfolio of global-scale cloud services. Today's announcement demonstrates Apple's commitment to the OPA project by making a larger investment in the technology and the community.

Open Policy Agent has a rich and vibrant community of end-user organizations, vendors, and individuals each contributing ideas, integrations, docs, and code so anyone in the world can use OPA to enforce the policies they care about. We've been fortunate to see such a community grow over the last decade, and look forward to continuing our contributions to the project as active community members.

We've compiled a set of FAQs below to address questions.

**What does this mean for the Open Policy Agent project?**
Open Policy Agent remains a CNCF graduated open source project and there are no changes to the project governance or licensing.

**What does this mean for Open Policy Agent maintainers?**
There is no change to the list of maintainers, except for an organization change from Styra to Apple for the maintainers that are joining Apple.

**What will happen to the tools I use under the Styra GitHub?**
We've initiated the community process for these repositories to be included in the CNCF OPA GitHub organization with the goal of deeper collaboration with the open source community:

- Styra's commercial distribution of OPA, EOPA: an optimized version of OPA designed for data heavy workloads with data-filtering functionality that was previously only available to enterprise customers.
- OPA Control Plane: a new control plane for OPA capable of building bundles from git and additional datasources and deploying them to S3 on AWS, GCP, and Azure.
- SDKs: SDKs for integrating with OPA including TypeScript, React, UCAST-Prisma, C#, ASP.NET, Java, and Springboot.
- Regal: a linter for OPA's policy language Rego (partly written in Rego itself).

Other tools will be evaluated for contribution in the future and remain publicly available for the community.

**What will happen to the OPA website and Rego Playground?**
The OPA website continues to remain available and managed by the CNCF and broader OPA community. The Rego Playground continues to be operated by Styra with no changes to current functionality.

**What is the planned roadmap for OPA?**
We're excited to continue the development of OPA with the same monthly release schedule. The 2025 OPA roadmap includes the following categories of work:

- Language extensions (and/or/not, keywords in refs, string interpolation, partial-set-functions, ellipsis)
- Type checking improvements
- Tooling improvements (streaming OPA test results, debugger attach to trace, rule tracing)
- Partial evaluation improvements (redundant expression elimination, 'in' handling)
- Performance (multiple expression indexing, faster loading of dependency-free bundles)
- Decision API and logging (congestion back-pressure, logging metadata, logging to disk)
