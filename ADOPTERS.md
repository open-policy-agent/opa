# Adopters

This is a list of production adopters of OPA (in alphabetical order):

* [bol.com](https://www.bol.com/) uses OPA for a mix of
  validating and mutating admission control use cases in their
  Kubernetes clusters. Use cases include patching image pull secrets,
  load balancer properties, and tolerations based on contextual
  information stored on namespaces. OPA is deployed on multiple
  clusters with ~100 nodes and ~300 namespaces total.

* [Chef](https://www.chef.io/) integrates OPA to implement IAM-style
  access control and enumerate user->resource permissions in Chef
  Automate V2. The integration utilizes OPA's Partial Evaluation
  feature to reduce evaluation time (in exchange for higher update
  latency.)

* [Cloudflare](https://www.cloudflare.com/) uses OPA as a validating
  admission controller to prevent conflicting Ingresses in their
  Kubernetes clusters that host a mix of production and test
  workloads.

* [Intuit](https://www.intuit.com/company/) uses OPA as a validating
  and mutating admission controller to implement various security,
  multi-tenancy, and risk management policies across approximately 50
  clusters and 1,000 namespaces. For more information on how Intuit
  uses OPA see [this talk from KubeCon Seattle 2018](https://youtu.be/CDDsjMOtJ-c?t=980).

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

* [SAP/InfraBox](https://github.com/SAP/Infrabox) integrates OPA to
  implement authorization over HTTP API resources. OPA policies
  evaluate user and permission data replicated from Postgres to make
  access control decisions over projects, collaborators, jobs,
  etc. SAP/Infrabox is used in production within SAP and has several
  external users.

* [Tremolo Security](https://www.tremolosecurity.com/) uses OPA at a
  London-based financial services company to inject annotations and
  volume mount parameters into Kubernetes Pods so that workloads can
  connect to off-cluster CIFS drives and SQL Server
  instances. Policies are based on external context sourced from
  OpenUnison. Ability to validate policies offline is a huge win
  because the clusters are air-gapped. For more information on how
  Tremolo Security uses OPA see [this blog post](https://www.tremolosecurity.com/beyond-rbac-in-openshift-open-policy-agent/).

* [Very Good Security (VGS)](https://www.vgs.io/) integrates OPA to
  implement a fine-grained permission system and enumerate
  user->resource permissions in their product. The backend is
  architected as a collection of (polyglot) microservices running on
  Kubernetes that offload policy decisions to OPA sidecars. VGS has
  implemented a synchronization protocol on top of the Bundle and
  Status APIs so that the system can determine when permission updates
  have propagated. For more details on the VGS use case see [this blog
  post](https://blog.verygoodsecurity.com/posts/building-a-fine-grained-permission-system-in-a-distributed-environment).

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

* [ORY Keto](https://github.com/ory/keto) replaced their internal
  decision engine with OPA. By leveraging OPA, ORY Keto was able to
  simplify their access control server implementation while retaining
  the ability to easily add high-level models like ACLs and RBAC. In
  December 2018, ~850 ORY Keto instances were running in a mix of
  pre-production and production environments.

Other adopters that have gone into production or various stages of
testing include:

* [Cisco](https://www.cisco.com/)
* [Nefeli Networks](https://nefeli.io)
* [Pinterest](https://www.pinterest.com/)
* [SolarWinds](https://www.solarwinds.com/) via [Lee Calcote](https://github.com/leecalcote)
* [State Street Corporation](http://www.statestreet.com/)

If you have adopted OPA and would like to be included in this list,
feel free to submit a PR.
