# Adopters

<!-- Hello! If you are using OPA and contributing to this file, thank you! -->
<!-- Please keep lines shorter than 80 characters (or so.) Links can go long. -->

This is a list of organizations that have spoken publicly about their adoption or
production users that have added themselves (in alphabetical order):

* [2U, Inc](https://2u.com) has incorporated OPA into their SDLC for both Terraform and Kubernetes deployments.
  Shift left!

* [Appsflyer](https://www.appsflyer.com/) uses OPA to make consistent
  authorization decisions by hundreds of microservices for UI and API data
  access. All authorization decisions are delegated to OPA that is deployed as a
  central service. The decisions are driven by flexible policy rules that take
  into consideration data privacy regulations and policies, data consents and
  application level access permissions. For more information, see the [Appsflyer
  Engineering Blog post](https://medium.com/appsflyer/authorization-solution-for-microservices-architecture-a2ac0c3c510b).

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
  registry allowlisting, label requirements, resource requirements, container
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

* [Digraph](https://www.getdigraph.com) is a developer-first cloud compliance platform
  that uses OPA to let security teams detect and resolve non-compliant infrastructure
  changes before they're deployed to production, and produce audit trails to eliminate
  manual work and accelerate audit processes like SOC and ISO.

* [Fugue](https://fugue.co) is a cloud security SaaS that uses OPA to
  classify compliance violations and security risks in AWS and Azure
  accounts and generate compliance reports and notifications.

* [Goldman Sachs](https://www.goldmansachs.com/) uses OPA to enforce admission control
  policies in their multi-tenant Kubernetes clusters as well as for _provisioning_
  RBAC, PV, and Quota resources that are central to the security and operation of
  these clusters. For more information see this talk from [KubeCon US 2019](https://www.youtube.com/watch?v=lYHr_UaHsYQ).

* [Google Cloud](https://cloud.google.com/) uses OPA to validate Google Cloud
  product's configurations in several products and tools, including
  [Anthos Config Management](https://cloud.google.com/anthos/config-management),
  [GKE Policy Automation](https://github.com/google/gke-policy-automation) or
  [Config Validator](https://github.com/GoogleCloudPlatform/policy-library). See
  [Creating policy-compliant Google Cloud resources article](https://cloud.google.com/architecture/policy-compliant-resources)
  for example use cases.

* [Infracost](https://www.infracost.io/) shows cloud cost estimates for Terraform.
  It uses OPA to enable users to create cost policies, and setup guardrails such
  as "this change puts the monthly costs above $10K, which is the budget for this
  product. Consider asking the team lead to review it". See [the docs](https://www.infracost.io/docs/features/cost_policies/) for details.

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

* [Mercari](https://www.mercari.com/) uses OPA to enforce admission control
  policies in their multi-tenant Kubernetes clusters. It helps maintain
  the governance of the cluster, checking that developers are following
  the best practices in the admission controller. They also use [confest](https://github.com/open-policy-agent/conftest) to
  enforce policies in their CI/CD pipeline.

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

* [Terminus Software](https://terminus.com/) uses OPA for microservice authorization.

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

* [VNG Cloud](https://www.vngcloud.vn/en/home) [Identity and Access Management (IAM)](https://iam.vngcloud.vn/)
  use OPA as a policy-based decision engine for authorization. IAM provides administrators with fine-grained 
  access control to VNG Cloud resources and help centralize and manage permissions to access resources. 
  Specifically, OPA is integrated to evaluate policies to make the decision about denying or allowing incoming requests.
  
* [Wiz](https://www.wiz.io/) helps every organization rapidly remove the most critical
  risks in their cloud estate. It simply connects in minutes, requires zero agents, and
  automatically correlates the entire security stack to uncover the most pressing issues.
  Wiz policies leverage Open Policy Agent (OPA) for a unified framework across the
  cloud-native stack. Whether for configurations, compliance, IaC, and more, OPA enables
  teams to move faster in the cloud. For more information on how Wiz uses OPA, [contact Wiz](https://www.wiz.io/contact/).

* [Xenit AB](https://www.xenit.se/) uses OPA to implement fine-grained control
  over resource formulation in its managed Kubernetes service as well as several
  customer-specific implementations. For more information, see the Kubernetes Terraform library [OPA Gatekeeper module](https://github.com/XenitAB/terraform-modules/tree/main/modules/kubernetes/opa-gatekeeper) and
  [OPA Gatekeeper policy library](https://github.com/XenitAB/gatekeeper-library).

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

* [Aserto](https://www.aserto.com/) is a venture-backed developer API company
  that helps developers easily build permissions and roles into their SaaS
  applications. Aserto uses OPA as its core engine, and has contributed projects
  such as [Open Policy Registry](https://openpolicyregistry.io) and
  [OPA Runtime](https://github.com/aserto-dev/runtime) that make it easier for
  developers to incorporate OPA policies and the OPA engine into their applications.

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

* [Permit.io](https://permit.io) Uses a combination of OPA and OPAL
  to power fine-grained authorization policies at the core of the Permit.io platform.
  Permit.io leverages the power of OPA's Rego language,
  generating new Rego code on the fly from its UI policy editor.
  The team behind Permit.io contributes to the OPA ecosystem - creating opens-source projects like
  [OPAL- making OPA event-driven)](https://github.com/permitio/opal)
  and [OPToggles - sync Frontend with open-policy](https://github.com/permitio/OPToggles).

* [Scalr](https://scalr.com/) is a remote operations backend for Terraform
  that helps users scale their Terraform usage through automation and collaboration.
  [Scalr uses OPA](https://docs.scalr.com/en/latest/opa.html) to validate Terraform
  code against organization standards and allows for approvals prior to a Terraform apply.

* [Spacelift](https://spacelift.io) is a specialized CI/CD platform
  for infrastructure-as-code. Spacelift is [using OPA](https://docs.spacelift.io/concepts/policy) to provide flexible,
  fine-grained controls at various application decision points, including
  automated code review, defining access levels or blocking execution of
  unwanted code.

* [Magda](https://github.com/magda-io/magda) is a federated, Kubernetes-based, open-source data catalog system. Working as Magda's central authorisation policy engine, OPA helps not only the API endpoint authorisation. Magda also uses its partial evaluation feature to translate datasets authorisation decisions to other database-specific DSLs (e.g. SQL or Elasticsearch DSL) and use them for dataset authorisation enforcement in different databases.

Other adopters that have gone into production or various stages of
testing include:

* [Cisco](https://www.cisco.com/)
* [Nefeli Networks](https://nefeli.io)
* [SolarWinds](https://www.solarwinds.com/) via [Lee Calcote](https://github.com/leecalcote)
* [State Street Corporation](http://www.statestreet.com/)
* [PITS Global Data Recovery Services](https://www.pitsdatarecovery.net/)

If you have adopted OPA and would like to be included in this list,
feel free to submit a PR updating this file or
[open an issue](https://github.com/open-policy-agent/opa/issues/new?assignees=&labels=adopt-opa&template=adopt-opa.yaml&title=organization_name+has+adopted+OPA).
