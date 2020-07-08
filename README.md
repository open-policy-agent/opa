# ![logo](./logo/logo-144x144.png) Open Policy Agent

[![Slack Status](http://slack.openpolicyagent.org/badge.svg)](https://slack.openpolicyagent.org) [![Build Status](https://github.com/open-policy-agne/opa/workflows/Post%20Merge/badge.svg)](https://github.com/open-policy-agent/opa/actions) [![Go Report Card](https://goreportcard.com/badge/open-policy-agent/opa)](https://goreportcard.com/report/open-policy-agent/opa) [![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/1768/badge)](https://bestpractices.coreinfrastructure.org/projects/1768) [![Netlify Status](https://api.netlify.com/api/v1/badges/4a0a092a-8741-4826-a28f-826d4a576cab/deploy-status)](https://app.netlify.com/sites/openpolicyagent/deploys)

The Open Policy Agent (OPA) is an open source, general-purpose policy engine that enables unified, context-aware policy enforcement across the entire stack.

OPA is hosted by the [Cloud Native Computing Foundation](https://cncf.io) (CNCF) as an incubating-level project. If you are an organization that wants to help shape the evolution of technologies that are container-packaged, dynamically-scheduled and microservices-oriented, consider joining the CNCF. For details read the CNCF [announcement](https://www.cncf.io/blog/2019/04/02/toc-votes-to-move-opa-into-cncf-incubator/).

## Want to learn more about OPA?

- See [openpolicyagent.org](https://www.openpolicyagent.org) to get started with documentation and tutorials.
- See [blog.openpolicyagent.org](https://blog.openpolicyagent.org) for blog posts about OPA and policy.
- See [ADOPTERS.md](./ADOPTERS.md) for a list of production OPA adopters and use cases.
- See [the Roadmap slides](https://docs.google.com/presentation/d/16QV6gvLDOV3I0_guPC3_19g6jHkEg3X9xqMYgtoCKrs/edit?usp=sharing) for a snapshot of high-level OPA features in-progress and planned.
- Try [play.openpolicyagent.org](https://play.openpolicyagent.org) to experiment with OPA policies.
- Join the conversation on [Slack](https://slack.openpolicyagent.org).

## Want to get OPA?

- See [Docker Hub](https://hub.docker.com/r/openpolicyagent/opa/tags/) for Docker images.
- See [GitHub releases](https://github.com/open-policy-agent/opa/releases) for binary releases and changelogs.

## Want to integrate OPA?

* See
  [![GoDoc](https://godoc.org/github.com/open-policy-agent/opa?status.svg)](https://godoc.org/github.com/open-policy-agent/opa/rego)
  to integrate OPA with services written in Go.
* See [REST API](https://www.openpolicyagent.org/docs/rest-api.html) to
  integrate OPA with services written in other languages.


## Want to contribute to OPA?

* See [DEVELOPMENT.md](./docs/devel/DEVELOPMENT.md) to build and test OPA itself.
* See [CONTRIBUTING.md](./CONTRIBUTING.md) to get started.
* Use [GitHub Issues](https://github.com/open-policy-agent/opa/issues) to request features or file bugs.
* Join bi-weekly meetings every other Tuesday at 10:00 (Pacific Timezone):
    * [Meeting Notes](https://docs.google.com/document/d/1v6l2gmkRKAn5UIg3V2QdeeCcXMElxsNzEzDkVlWDVg8/edit?usp=sharing)
    * [Zoom](https://zoom.us/j/97827947600)
    * [Calendar Invite](https://calendar.google.com/event?action=TEMPLATE&tmeid=MnRvb2M4amtldXBuZ2E1azY0MTJndjh0ODRfMjAxODA5MThUMTcwMDAwWiBzdHlyYS5jb21fY28zOXVzc3VobnE2amUzN2l2dHQyYmNiZGdAZw&tmsrc=styra.com_co39ussuhnq6je37ivtt2bcbdg%40group.calendar.google.com&scp=ALL)

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

The examples below show different kinds of policies you can define with OPA as
well as different kinds of queries your system can execute against OPA. The
example queries are executed inside OPA's
[REPL](https://www.openpolicyagent.org/docs/get-started.html) which was built to
make it easy to develop and test policies.

For concrete examples of how to integrate OPA with systems like [Kubernetes](https://www.openpolicyagent.org/docs/kubernetes-admission-control.html), [Terraform](https://www.openpolicyagent.org/docs/terraform.html), [Docker](https://www.openpolicyagent.org/docs/docker-authorization.html), [SSH](https://www.openpolicyagent.org/docs/ssh-and-sudo-authorization.html), and more, see [openpolicyagent.org](https://www.openpolicyagent.org).

### Example: API Authorization

This example shows how you can enforce access controls over salary information
served by a simple HTTP API. In this example, users are allowed to access their
own salary as well as the salary of anyone who reports to them.

The management chain is represented in JSON and stored in a file (`data.json`):

```json
{
    "management_chain": {
        "bob": [
            "ken",
            "janet"
        ],
        "alice": [
            "janet"
        ]
    }
}
```

Start OPA and load the `data.json` file:

```bash
opa run data.json
```

Inside the REPL you can define rules and execute queries. Paste the following rules into the REPL.

```ruby
default allow = false

allow {
    input.method = "GET"
    input.path = ["salary", id]
    input.user_id = id
}

allow {
    input.method = "GET"
    input.path = ["salary", id]
    managers = data.management_chain[id]
    input.user_id = managers[_]
}
```

#### Example Queries

**Is someone allowed to access their own salary?**

```ruby
> input := {"method": "GET", "path": ["salary", "bob"], "user_id": "bob"}
> allow
true
```

**Display the management chain for Bob:**

```ruby
> data.management_chain["bob"]
[
    "ken",
    "janet"
]
```

**Is Alice allowed to access Bob's salary?**

```ruby
> input := {"method": "GET", "path": ["salary", "bob"], "user_id": "alice"}
> allow
false
```

**Is Janet allowed to access Bob's salary?**

```ruby
> input := {"method": "GET", "path": ["salary", "bob"], "user_id": "janet"}
> allow
true
```

### Example: App Placement

This example shows how you can enforce where apps are deployed inside a simple
orchestrator. In this example, apps must be deployed onto clusters that satisfy
PCI and jurisdiction requirements.

```ruby
app_placement[cluster_id] {
    cluster = data.clusters[cluster_id]
    satisfies_jurisdiction(input.app, cluster)
    satisfies_pci(input.app, cluster)
}

satisfies_jurisdiction(app, cluster) {
    not app.tags["requires-eu"]
}

satisfies_jurisdiction(app, cluster) {
    app.tags["requires-eu"]
    startswith(cluster.region, "eu-")
}

satisfies_pci(app, cluster) {
    not app.tags["requires-pci-level"]
}

satisfies_pci(app, cluster) {
    level = to_number(app.tags["requires-pci-level"])
    level >= cluster.tags["pci-level"]
}
```

#### Example Queries

**Where will this app be deployed?**

```ruby
> input := {"app": {"tags": {"requires-pci-level": "3", "requires-eu": "true"}}}
> app_placement
[
    "prod-eu"
]
```

**Display clusters in EU region:**

```ruby
> startswith(data.clusters[cluster_id].region, "eu-")
+------------+
| cluster_id |
+------------+
| "prod-eu"  |
| "test-eu"  |
+------------+
```

**Display all clusters:**

```ruby
> data.clusters[cluster_id]
+------------+------------------------------------------------+
| cluster_id |           data.clusters[cluster_id]            |
+------------+------------------------------------------------+
| "prod-eu"  | {"region":"eu-central","tags":{"pci-level":2}} |
| "prod-us"  | {"region":"us-east"}                           |
| "test-eu"  | {"region":"eu-west","tags":{"pci-level":4}}    |
| "test-us"  | {"region":"us-west"}                           |
+------------+------------------------------------------------+
```

### Example: SSH Auditing

This example shows how you can audit who has SSH access to hosts within
different clusters. We will assume that SSH access is granted via group access
in LDAP.

```ruby
import data.ldap
import data.clusters

ssh_access[[cluster_name, host_id, user_id]] {
    host_id = clusters[cluster_name].hosts[_]
    group_id = ldap.users[user_id].groups[_]
    group_id = clusters[cluster_name].groups[_]
}

prod_users = {user_id | ssh_access[["prod", _, user_id]]}
```

#### Example Queries

**Who can access production hosts?**

```ruby
> prod_users
[
  "alice",
  "bob"
]
```

**Display all LDAP users:**

```ruby
> data.ldap.users[user_id]
+-------------------------------+---------+
|   data.ldap.users[user_id]    | user_id |
+-------------------------------+---------+
| {"groups":["dev","platform"]} | "alice" |
| {"groups":["dev","ops"]}      | "bob"   |
| {"groups":["dev"]}            | "janet" |
+-------------------------------+---------+
```

**Display all cluster/group pairs:**

```ruby
> data.clusters[cluster_id].groups[_] = group_id
+------------+------------+
| cluster_id |  group_id  |
+------------+------------+
| "test"     | "dev"      |
| "test"     | "ops"      |
| "prod"     | "ops"      |
| "prod"     | "platform" |
+------------+------------+
```

**Does Janet have access to the test cluster?**

```ruby
> ssh_access[["test", _, "janet"]]
true
```

**What are the addresses of the hosts in the test cluster that Janet can access?**

```ruby
> ssh_access[["test", host_id, "janet"]]; addr = data.hosts[host_id].addr
+------------+------------+
|    addr    |  host_id   |
+------------+------------+
| "10.0.0.1" | "host-abc" |
| "10.0.0.2" | "host-cde" |
| "10.0.0.3" | "host-efg" |
+------------+------------+
```

## Further Reading

### Presentations

- Open Policy Agent Introduction @ CloudNativeCon EU 2018: [video](https://youtu.be/XEHeexPpgrA), [slides](https://www.slideshare.net/TorinSandall/opa-the-cloud-native-policy-engine)
- Rego Deep Dive @ CloudNativeCon EU 2018: [video](https://youtu.be/4mBJSIhs2xQ), [slides](https://www.slideshare.net/TorinSandall/rego-deep-dive)
- How Netflix Is Solving Authorization Across Their Cloud @ CloudNativeCon US 2017: [video](https://www.youtube.com/watch?v=R6tUNpRpdnY), [slides](https://www.slideshare.net/TorinSandall/how-netflix-is-solving-authorization-across-their-cloud).
- Policy-based Resource Placement in Kubernetes Federation @ LinuxCon Beijing 2017: [slides](https://www.slideshare.net/TorinSandall/policybased-resource-placement-across-hybrid-cloud), [screencast](https://www.youtube.com/watch?v=hRz13baBhfg&feature=youtu.be).
- Enforcing Bespoke Policies In Kubernetes @ KubeCon US 2017: [video](https://www.youtube.com/watch?v=llDI8VvkUj8), [slides](https://www.slideshare.net/TorinSandall/enforcing-bespoke-policies-in-kubernetes).
- Istio's Mixer: Policy Enforcement with Custom Adapters @ CloudNativeCon US 2017: [video](https://www.youtube.com/watch?v=czZLXUqzd24), [slides](https://www.slideshare.net/TorinSandall/istios-mixer-policy-enforcement-with-custom-adapters-cloud-nativecon-17).

## Security

### Security Audit

A third party security audit was performed by Cure53, you can see the full report [here](SECURITY_AUDIT.pdf)

### Reporting Security Vulnerabilities

Please report vulnerabilities by email to [open-policy-agent-security](mailto:open-policy-agent-security@googlegroups.com).
We will send a confirmation message to acknowledge that we have received the
report and then we will send additional messages to follow up once the issue
has been investigated.
