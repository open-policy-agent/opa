# ![logo](./logo/logo-144x144.png) Open Policy Agent

[![Build Status](https://github.com/open-policy-agent/opa/workflows/Post%20Merge/badge.svg?branch=main)](https://github.com/open-policy-agent/opa/actions) [![Go Report Card](https://goreportcard.com/badge/open-policy-agent/opa)](https://goreportcard.com/report/open-policy-agent/opa) [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1768/badge)](https://bestpractices.coreinfrastructure.org/projects/1768) [![Netlify Status](https://api.netlify.com/api/v1/badges/4a0a092a-8741-4826-a28f-826d4a576cab/deploy-status)](https://app.netlify.com/sites/openpolicyagent/deploys)

Open Policy Agent (OPA) is an open source, general-purpose policy engine that enables unified, context-aware policy enforcement across the entire stack.

OPA is proud to be a graduated project in the [Cloud Native Computing Foundation](https://cncf.io) (CNCF) landscape. For details read the CNCF [announcement](https://www.cncf.io/announcements/2021/02/04/cloud-native-computing-foundation-announces-open-policy-agent-graduation/).

## Want to connect with the community or get support for OPA?

- Join the [OPA Slack](https://slack.openpolicyagent.org) for day-to-day conversations with the OPA community.
- Need Support? Check out the [Community Discussions](https://github.com/orgs/open-policy-agent/discussions) to ask questions.

## Want to learn more about OPA?

- Go to [openpolicyagent.org](https://www.openpolicyagent.org) to get started with documentation and tutorials.
- Browse [blog.openpolicyagent.org](https://blog.openpolicyagent.org) for news about OPA, community, policy and authorization.
- Watch OPA's [YouTube](https://www.youtube.com/channel/UClDMRN5HlqD3di5MMf-SV4A) channel for videos about OPA.
- Try OPA with the [Rego Playground](https://play.openpolicyagent.org) to experiment with policies and share your work.
- View the [OPA Roadmap](https://docs.google.com/presentation/d/16QV6gvLDOV3I0_guPC3_19g6jHkEg3X9xqMYgtoCKrs/edit?usp=sharing) to see a high-level snapshot of OPA features in-progress and planned.
- Check out the [ADOPTERS.md](./ADOPTERS.md) file for a list of production adopters. Does your organization use OPA in production? Support the OPA project by submitting a PR to add your organization to the list with a short description of your OPA use cases!

## Want to download OPA?

- [Docker Hub](https://hub.docker.com/r/openpolicyagent/opa/tags/) for Docker images.
- [GitHub releases](https://github.com/open-policy-agent/opa/releases) for binary releases and changelogs.

## Want to integrate OPA?

* See the high-level [Go SDK](https://www.openpolicyagent.org/docs/latest/integration/#integrating-with-the-go-sdk) or the low-level Go API
  [![GoDoc](https://godoc.org/github.com/open-policy-agent/opa?status.svg)](https://godoc.org/github.com/open-policy-agent/opa/rego)
  to integrate OPA with services written in Go.
* See [REST API](https://www.openpolicyagent.org/docs/rest-api.html) to
  integrate OPA with services written in other languages.
* See the [integration docs](https://www.openpolicyagent.org/docs/latest/integration/) for more options.

## Want to contribute to OPA?

* Read the [Contributing Guide](https://www.openpolicyagent.org/docs/latest/contributing/) to learn how to make your first contribution.
* Use [#contributors](https://openpolicyagent.slack.com/archives/C02L1TLPN59) in Slack to talk to other contributors and OPA maintainers.
* File a [GitHub Issue](https://github.com/open-policy-agent/opa/issues) to request features or report bugs.

## How does OPA work?

OPA gives you a high-level declarative language to author and enforce policies
across your stack.

With OPA, you define _rules_ that govern how your system should behave. These
rules exist to answer questions like:

* Can user X call operation Y on resource Z?
* What clusters should workload W be deployed to?
* What tags must be set on resource R before it's created?

You integrate services with OPA so that these kinds of policy decisions do not
have to be *hardcoded* in your service. Services integrate with OPA by
executing _queries_ when policy decisions are needed.

When you query OPA for a policy decision, OPA evaluates the rules and data
(which you give it) to produce an answer. The policy decision is sent back as
the result of the query.

For example, in a simple API authorization use case:

* You write rules that allow (or deny) access to your service APIs.
* Your service queries OPA when it receives API requests.
* OPA returns allow (or deny) decisions to your service.
* Your service _enforces_ the decisions by accepting or rejecting requests accordingly.

For concrete examples of how to integrate OPA with systems like [Kubernetes](https://www.openpolicyagent.org/docs/kubernetes-admission-control.html), [Terraform](https://www.openpolicyagent.org/docs/terraform.html), [Docker](https://www.openpolicyagent.org/docs/docker-authorization.html), [SSH](https://www.openpolicyagent.org/docs/ssh-and-sudo-authorization.html), and more, see [openpolicyagent.org](https://www.openpolicyagent.org).

## Presentations

- OPA maintainers talk @ Kubecon NA 2022: [video](https://www.youtube.com/watch?v=RMiovzGGCfI)
- Open Policy Agent (OPA) Intro & Deep Dive @ Kubecon EU 2022: [video](https://www.youtube.com/watch?v=MhyQxIp1H58)
- Open Policy Agent Intro @ KubeCon EU 2021: [Video](https://www.youtube.com/watch?v=2CgeiWkliaw)
- Using Open Policy Agent to Meet Evolving Policy Requirements @ KubeCon NA 2020: [video](https://www.youtube.com/watch?v=zVuM7F_BTyc)
- Applying Policy Throughout The Application Lifecycle with Open Policy Agent @ CloudNativeCon 2019: [video](https://www.youtube.com/watch?v=cXfsaE6RKfc)
- Open Policy Agent Introduction @ CloudNativeCon EU 2018: [video](https://youtu.be/XEHeexPpgrA), [slides](https://www.slideshare.net/TorinSandall/opa-the-cloud-native-policy-engine)
- Rego Deep Dive @ CloudNativeCon EU 2018: [video](https://youtu.be/4mBJSIhs2xQ), [slides](https://www.slideshare.net/TorinSandall/rego-deep-dive)
- How Netflix Is Solving Authorization Across Their Cloud @ CloudNativeCon US 2017: [video](https://www.youtube.com/watch?v=R6tUNpRpdnY), [slides](https://www.slideshare.net/TorinSandall/how-netflix-is-solving-authorization-across-their-cloud).
- Policy-based Resource Placement in Kubernetes Federation @ LinuxCon Beijing 2017: [slides](https://www.slideshare.net/TorinSandall/policybased-resource-placement-across-hybrid-cloud), [screencast](https://www.youtube.com/watch?v=hRz13baBhfg&feature=youtu.be)
- Enforcing Bespoke Policies In Kubernetes @ KubeCon US 2017: [video](https://www.youtube.com/watch?v=llDI8VvkUj8), [slides](https://www.slideshare.net/TorinSandall/enforcing-bespoke-policies-in-kubernetes)
- Istio's Mixer: Policy Enforcement with Custom Adapters @ CloudNativeCon US 2017: [video](https://www.youtube.com/watch?v=czZLXUqzd24), [slides](https://www.slideshare.net/TorinSandall/istios-mixer-policy-enforcement-with-custom-adapters-cloud-nativecon-17)

## Security Audit

A third party security audit was performed by Cure53, you can see the full report [here](SECURITY_AUDIT.pdf)

## Reporting Security Vulnerabilities

Please report vulnerabilities by email to [open-policy-agent-security](mailto:open-policy-agent-security@googlegroups.com).
We will send a confirmation message to acknowledge that we have received the
report and then we will send additional messages to follow up once the issue
has been investigated.
