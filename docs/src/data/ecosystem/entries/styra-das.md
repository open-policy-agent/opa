---
title: Styra Declarative Authorization Service
subtitle: Policy as Code Control Plane
software:
- styra-das
- kubernetes
- envoy
- terraform
labels:
  category: authorization
  type: poweredbyopa
tutorials:
- https://docs.styra.com/getting-started
- https://docs.styra.com/tutorials/kubernetes/introduction
- https://docs.styra.com/tutorials/envoy/introduction
- https://docs.styra.com/tutorials/ssh/introduction
- https://docs.styra.com/tutorials/terraform/introduction
- https://docs.styra.com/tutorials/entitlements/introduction
- https://academy.styra.com/courses/opa-rego
code:
- https://github.com/StyraInc/das-opa-samples
- https://github.com/StyraInc/example-policy-management
- https://github.com/StyraInc/entitlements-samples
inventors:
- styra
blogs:
- https://blog.styra.com/blog/six-of-my-favorite-styra-declarative-authorization-service-das-features
- https://blog.styra.com/blog/styra-declarative-authorization-service-expands-service-mesh-use-case
- https://blog.styra.com/blog/opa-styra-terraform-protect-your-cloud-investment
- https://www.styra.com/blog/how-to-write-your-first-rules-in-rego-the-policy-language-for-opa
videos:
- title: Securing Microservices-Based Apps with Dynamic Traffic Authz
  speakers:
  - name: Kurt Roekle
    organization: styra
  venue: online
  link: https://www.youtube.com/watch?v=9F-Zyn9j25g
- title: 'Policy Management Across the Cloud-Native Stack: Styra DAS for Terraform'
  speakers:
  - name: Kurt Roekle
    organization: styra
  venue: online
  link: https://www.youtube.com/watch?v=3K0RqIvNfAc
docs_features:
  status-api:
    note: |
      Styra DAS can receive status updates from OPA instances via the status
      API. See the
      [documentation here](https://docs.styra.com/das/policies/policy-organization/systems/view-opa-status)
      for more information.
  envoy:
    note: |
      Styra DAS provides an out-of-the-box integration for writing Envoy
      authorization policies. See the
      [tutorial](https://docs.styra.com/das/systems/envoy/tutorials)
      here.
  policy-testing:
    note: |
      DAS supports the running of tests alongside Rego policy in its UI.
      Read documentation about
      [testing Rego policies in DAS](https://docs.styra.com/das/policies/policy-authoring/test-policies).
  language-tooling:
    note: |
      DAS supports the writing, testing and debugging of Rego policies in a UI.
      Read about
      [Writing Policies](https://docs.styra.com/das/policies/policy-authoring/write-policies)
      and
      [Testing Policies](https://docs.styra.com/das/policies/policy-authoring/test-policies)
      in the DAS documentation.
  external-data:
    note: |
      The [Bundle Registry](https://docs.styra.com/das/policies/bundles/bundle-registry)
      feature of DAS uses OPA's Bundle API to distribute policy and data updates to
      OPA instances.
  opa-bundles:
    note: |
      Styra DAS exposes a Bundle Service compatible API to the OPA instances
      that it manages. Delta bundles are also supported. Read the
      [Bundle Registry Docs](https://docs.styra.com/das/policies/bundles/bundle-registry)
      to learn more.
  opa-bundles-discovery:
    note: |
      Styra DAS exposes a Bundle Service compatible API to the OPA instances
      that it manages. Discovery bundles are also supported. Read the
      [Bundle Registry Docs](https://docs.styra.com/das/policies/bundles/bundle-registry)
      to learn more.
  terraform:
    note: |
      Styra DAS has native support for the validation of Terraform code and
      plans via a prebuilt 'system-type', this is
      [documented here](https://docs.styra.com/das/systems/terraform/overview).
  kubernetes:
    note: |
      Styra DAS has native support for mutating and validating Kubernetes
      at admission time via a prebuilt 'system-type', this is
      [documented here](https://docs.styra.com/das/systems/kubernetes/overview).
  decision-logging:
    note: |
      Styra DAS can aggregate and index OPA decision logs. Exporting to
      object storage, and Kafka is also supported. View
      [the documentation](https://docs.styra.com/das/observability-and-audit/decision-logs/overview)
      here.
---
Styra DAS provides a single pane of glass for authorization and policy across the cloud-native ecosystem of software systems. Beyond a simple control plane, Styra DAS pushes OPA’s potential, providing powerful impact analysis, policy authoring, and decision logging.
