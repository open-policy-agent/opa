# Adopters

<!-- Hello! If you are using OPA and contributing to this file, thank you! -->
<!-- Please keep lines shorter than 80 characters (or so.) Links can go long. -->

This is a list of organizations that have spoken publicly about their adoption or
production users that have added themselves (in alphabetical order):

* [Atlassian](https://www.atlassian.com/) uses OPA in a heterogeneous cloud
  environment for microservice API authorization. OPA is deployed per-host and
  inside of their Slauth (AAA) system. Policies are tagged and categorized
  (e.g., platform, service, etc.) and distributed via S3. Custom log infrastructure
  consumes decision logs. For more information see this talk from [OPA Summit 2019](https://www.youtube.com/watch?v=nvRTO8xjmrg).

* [Bisnode](https://www.bisnode.com) uses OPA for a wide range of use cases,
  including microservice authorization, fine grained kubernetes authorization,
  validating and mutating admission control and CI/CD pipeline testing. Built
  and maintains some OPA related tools and libraries, primarily to help
  integrate OPA in the Java/JVM ecosystem, [see `github.com/Bisnode`](https://github.com/Bisnode).

* [bol.com](https://www.bol.com/) uses OPA for a mix of
  validating and mutating admission control use cases in their
  Kubernetes clusters. Use cases include patching image pull secrets,
  load balancer properties, and tolerations based on contextual
  information stored on namespaces. OPA is deployed on multiple
  clusters with ~100 nodes and ~300 namespaces total.

* [BNY Mellon](https://www.bnymellon.com/) uses OPA as a sidecar to enforce access
  control over applications based on external context coming from AD and other
  internal services. For more information see this talk from [QCon 2019](https://www.infoq.com/presentations/opa-spring-boot-hocon/).

* [Capital One](https://www.capitalone.com/) uses OPA to enforce a variety of
  admission control policies across their Kubernetes clusters including image
  registry whitelisting, label requirements, resource requirements, container
  privileges, etc. For more information see this talk from [KubeCon US 2018](https://www.youtube.com/watch?v=CDDsjMOtJ-c&t=6m35s)
  and this talk from [OPA Summit 2019](https://www.youtube.com/watch?v=vkvWZuqSk5M).

* [Chef](https://www.chef.io/) integrates OPA to implement IAM-style
  access control and enumerate user->resource permissions in Chef
  Automate V2. The integration utilizes OPA's Partial Evaluation
  feature to reduce evaluation time (in exchange for higher update
  latency.) A high-level description can be found [in this blog
  post](https://blog.chef.io/2019/01/24/introducing-the-chef-automate-identity-access-management-version-two-iam-v2-beta/),
  and the code is Open Source, [see
  `github.com/chef/automate`](https://github.com/chef/automate/tree/master/components/authz-service).

* [cluetec.de](https://cluetec.de) primarily uses OPA to enforce fine-grained authorization
  and data-filtering policies in its Spring-based microservices and multi-tenant SaaS. Policies
  are mapped to tenant-specific domains and used to enrich the database queries without any code
  modifications. OPA is also used to enforce admission control policies and RBAC in multi-tenant
  Kubernetes clusters.

* [Cloudflare](https://www.cloudflare.com/) uses OPA as a validating
  admission controller to prevent conflicting Ingresses in their
  Kubernetes clusters that host a mix of production and test
  workloads.

* [ControlPlane](https://control-plane.io) uses OPA to enforce enterprise-friendly
  policy for safe adoption of Kubernetes, Istio, and cloud services. OPA policies
  are validated and tested individually and en masse with unit tests and conftest.
  This enables developers to validate local changes against production policies,
  minimise engineering feedback loops, and reduce CI cycle time. Policies are
  tested as "SDLC guardrails", then re-validated at deployment time by a range of
  OPA-based admission controllers, covering single-tenant environments and hard
  multi-tenancy configurations.

* [Fugue](https://fugue.co) is a cloud security SaaS that uses OPA to
  classify compliance violations and security risks in AWS and Azure
  accounts and generate compliance reports and notifications.

* [Goldman Sachs](https://www.goldmansachs.com/) uses OPA to enforce admission control
  policies in their multi-tenant Kubernetes clusters as well as for _provisioning_
  RBAC, PV, and Quota resources that are central to the security and operation of
  these clusters. For more information see this talk from [KubeCon US 2019](https://www.youtube.com/watch?v=lYHr_UaHsYQ).

* [Intuit](https://www.intuit.com/company/) uses OPA as a validating
  and mutating admission controller to implement various security,
  multi-tenancy, and risk management policies across approximately 50
  clusters and 1,000 namespaces. For more information on how Intuit
  uses OPA see [this talk from KubeCon Seattle 2018](https://youtu.be/CDDsjMOtJ-c?t=980).

* [Jetstack](https://www.jetstack.io) uses OPA on customer projects to validate
  resources deployed to Kubernetes environments are conformant with
  organization rules. This has involved both validating and mutating resources
  as well as the following related projects: conftest, konstraint, and
  Gatekeeper. Jetstack also uses OPA via the Golang API in _Jetstack Secure_ to
  automate the checking of resources against our best practice recommendations.

* [Medallia](https://www.medallia.com/) uses OPA to audit AWS
  resources for compliance violations. The policies search across
  state from Terraform and AWS APIs to identify security violations
  and identify high-risk configurations. The policies ingest 1,000s of
  AWS resources to generate the final report.

* [Netflix](https://www.netflix.com) uses OPA as a method of enforcing
  access control in microservices across a variety of languages and
  frameworks for thousands of instances in their cloud
  infrastructure. Netflix takes advantage of OPA's ability to bring in
  contextual information and data from remote resources in order to
  evaluate policies in a flexible and consistent manner. For a
  description of how Netflix has architected access control with OPA
  check out [this talk from KubeCon Austin 2017](https://www.youtube.com/watch?v=R6tUNpRpdnY).

* [Pinterest](https://www.pinterest.com/) uses OPA to solve multiple policy-related use cases
  including access control in Kafka, Envoy, and Jenkins! At peak, their Kafka-OPA
  integration handles ~400K QPS without caching. With caching the system
  handles ~8.5M QPS. For more information see this talk from [OPA Summit 2019](https://www.youtube.com/watch?v=LhgxFICWsA8).

* [Plex Systems](https://www.plex.com) uses OPA to enforce policy throughout
  their entire release process; from local development to continuous production
  audits. The CI/CD pipelines at Plex leverage [conftest](https://github.com/instrumenta/conftest),
  a policy enforcement tool that relies on OPA, to automatically reject changes that do not adhere
  to defined policies. Plex also uses
  [Gatekeeper](https://github.com/open-policy-agent/gatekeeper), a Kubernetes policy controller, as
  a means to enforce policies within their Kubernetes clusters. The general-purpose nature of OPA
  has enabled Plex to have a consistent means of policy enforcement,
  no matter the environment.

* [Splash]([https://splashthat.com) uses OPA to handle fine-grained authorization
  across its entire platform, implemented as both a sidecar in Kubernetes and a separate
  container on bare instances. Policies and datasets are recompiled and updated based
  on changes to users' roles and permissions.

* [SAP/InfraBox](https://github.com/SAP/Infrabox) integrates OPA to
  implement authorization over HTTP API resources. OPA policies
  evaluate user and permission data replicated from Postgres to make
  access control decisions over projects, collaborators, jobs,
  etc. SAP/Infrabox is used in production within SAP and has several
  external users.

* [T-Mobile](https://www.t-mobile.com) uses OPA as a core component for their
  [MagTape](https://github.com/tmobile/magtape/) project that enforces best
  practices and secure configurations across their fleet of Kubernetes
  clusters (more info in [this blog post](https://opensource.t-mobile.com/blog/posts/rolling-out-the-magenta-tape/)).
  T-Mobile also leverages OPA to enforce authorization workflows within their
  Corporate Delivery Platform (CI/CD).

* [Tremolo Security](https://www.tremolosecurity.com/) uses OPA at a
  London-based financial services company to inject annotations and
  volume mount parameters into Kubernetes Pods so that workloads can
  connect to off-cluster CIFS drives and SQL Server
  instances. Policies are based on external context sourced from
  OpenUnison. Ability to validate policies offline is a huge win
  because the clusters are air-gapped. For more information on how
  Tremolo Security uses OPA see [this blog post](https://www.tremolosecurity.com/beyond-rbac-in-openshift-open-policy-agent/).

* [Tripadvisor](http://tripadvisor.com/) uses OPA to enforce
  admission control policies in Kubernetes. In the process of rolling out OPA,
  they created an integration testing framework that verifies clusters are accepting
  and rejecting the right objects when OPA is deployed. For more information see
  this talk from [OPA Summit 2019](https://www.youtube.com/watch?v=X09c1eXvCFM).

* [Very Good Security (VGS)](https://www.vgs.io/) integrates OPA to
  implement a fine-grained permission system and enumerate
  user->resource permissions in their product. The backend is
  architected as a collection of (polyglot) microservices running on
  Kubernetes that offload policy decisions to OPA sidecars. VGS has
  implemented a synchronization protocol on top of the Bundle and
  Status APIs so that the system can determine when permission updates
  have propagated. For more details on the VGS use case see these blog posts:
  [part 1](https://blog.verygoodsecurity.com/posts/building-a-fine-grained-permission-system-in-a-distributed-environment),
  [part 2](https://blog.verygoodsecurity.com/posts/building-a-fine-grained-permissions-system-in-a-distributed-environment).

* [Yelp](https://www.yelp.com/) use OPA and Envoy to enforce authorization policies
  across a fleet of microservices that evolved out of a monolithic architecture.
  For more information see this talk from [KubeCon US 2019](https://www.youtube.com/watch?v=Z6aN3Smt-9M).

In addition, there are several production adopters that prefer to
remain anonymous.

* **A Fortune 100 company** uses OPA to implement validating admission
  control and fine-grained authorization policies on ~10 Kubernetes
  clusters with ~1,000 nodes. They also integrate OPA into their PKI
  as part of a Certificate RA that serves these clusters.

This is a list of adopters in early stages of production or
pre-production (in alphabetical order):

* [Cyral](https://www.cyral.com/) is a venture-funded data security
  company. Still in stealth mode but using OPA to manage and enforce
  fine-grained authorization policies.

* [build.security](https://build.security/) is a venture-funded cyber security
  company, making it easy for developers to build role-based and attribute-based
  access controls to their applications and services. build.security is leveraging
  OPA and rego at their core technology.

* [ORY Keto](https://github.com/ory/keto) replaced their internal
  decision engine with OPA. By leveraging OPA, ORY Keto was able to
  simplify their access control server implementation while retaining
  the ability to easily add high-level models like ACLs and RBAC. In
  December 2018, ~850 ORY Keto instances were running in a mix of
  pre-production and production environments.

* [Scalr](https://scalr.com/) is a remote operations backend for Terraform
  that helps users scale their Terraform usage through automation and collaboration.
  Scalr uses OPA](https://docs.scalr.com/en/latest/opa.html) to validate Terraform
  code against organization standards and allows for approvals prior to a Terraform apply.

* [Spacelift](https://spacelift.io) is a specialized CI/CD platform
  for infrastructure-as-code. Spacelift is [using OPA](https://docs.spacelift.io/concepts/policy) to provide flexible,
  fine-grained controls at various application decision points, including
  automated code review, defining access levels or blocking execution of
  unwanted code.

Other adopters that have gone into production or various stages of
testing include:

* [Cisco](https://www.cisco.com/)
* [Nefeli Networks](https://nefeli.io)
* [SolarWinds](https://www.solarwinds.com/) via [Lee Calcote](https://github.com/leecalcote)
* [State Street Corporation](http://www.statestreet.com/)

If you have adopted OPA and would like to be included in this list,
feel free to submit a PR.
