# ![logo](./logo/logo.png) Open Policy Agent

[![Slack Status](http://slack.openpolicyagent.org/badge.svg)](http://slack.openpolicyagent.org) [![Build Status](https://travis-ci.org/open-policy-agent/opa.svg?branch=master)](https://travis-ci.org/open-policy-agent/opa) [![Go Report Card](https://goreportcard.com/badge/open-policy-agent/opa)](https://goreportcard.com/report/open-policy-agent/opa)
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopen-policy-agent%2Fopa.svg?type=shield)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopen-policy-agent%2Fopa?ref=badge_shield)

The Open Policy Agent (OPA) is an open source, general-purpose policy engine that enables unified, context-aware policy enforcement across the entire stack.

OPA is hosted by the [Cloud Native Computing Foundation](https://cncf.io) (CNCF) as a sandbox level project. If you are an organization that wants to help shape the evolution of technologies that are container-packaged, dynamically-scheduled and microservices-oriented, consider joining the CNCF. For details read the CNCF [announcement](https://www.cncf.io/blog/2018/03/29/cncf-to-host-open-policy-agent-opa/).

## Want to learn more about OPA?

- See [openpolicyagent.org](http://www.openpolicyagent.org) to get started with documentation and tutorials.
- See [blog.openpolicyagent.org](https://blog.openpolicyagent.org) for blog posts about OPA and policy.
- Join the conversation on [Slack](http://slack.openpolicyagent.org).

## Want to get OPA?

- See [Docker Hub](https://hub.docker.com/r/openpolicyagent/opa/tags/) for Docker images.
- See [GitHub releases](https://github.com/open-policy-agent/opa/releases) for binary releases and changelogs.

## Want to integrate OPA?

* See
  [![GoDoc](https://godoc.org/github.com/open-policy-agent/opa?status.svg)](https://godoc.org/github.com/open-policy-agent/opa/rego)
  to integrate OPA with services written in Go.
* See [REST API](http://www.openpolicyagent.org/docs/rest-api.html) to
  integrate OPA with services written in other languages.


## Want to contribute to OPA?

* See [DEVELOPMENT.md](./docs/devel/DEVELOPMENT.md) to build and test OPA itself.
* See [CONTRIBUTING.md](./CONTRIBUTING.md) to get started.
* Use [GitHub Issues](https://github.com/open-policy-agent/opa/issues) to request features or file bugs.
* Join weekly meetings every Tuesday at 10:00 (Pacific Timezone):
    * [Meeting Notes](https://docs.google.com/document/d/1v6l2gmkRKAn5UIg3V2QdeeCcXMElxsNzEzDkVlWDVg8/edit?usp=sharing)
    * [Google Hangouts](https://plus.google.com/hangouts/_/styra.com/opa-weekly)
    * [Calendar Invite](https://calendar.google.com/event?action=TEMPLATE&tmeid=N3AzamxzYWY2MG9wa2J0cmFjODNzaXI3MDhfMjAxODAxMTZUMTgwMDAwWiB0b3JpbkBzdHlyYS5jb20&tmsrc=torin%40styra.com&scp=ALL)

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
[REPL](http://www.openpolicyagent.org/docs/get-started.html) which was built to
make it easy to develop and test policies.

For concrete examples of how to integrate OPA with systems like [Kubernetes](http://www.openpolicyagent.org/docs/kubernetes-admission-control.html), [Terraform](http://www.openpolicyagent.org/docs/terraform.html), [Docker](http://www.openpolicyagent.org/docs/docker-authorization.html), [SSH](http://www.openpolicyagent.org/docs/ssh-and-sudo-authorization.html), and more, see [openpolicyagent.org](http://www.openpolicyagent.org).

### Example: API Authorization

This example shows how you can enforce access controls over salary information
served by a simple HTTP API. In this example, users are allowed to access their
own salary as well as the salary of anyone who reports to them.

```ruby
allow {
    input.method = "GET"
    input.path = ["salary", id]
    input.user_id = id
}

allow {
    input.method = "GET"
    input.path = ["salary", id]
    managers = data.management_chain[id]
    id = managers[_]
}
```

#### Example Queries

**Is someone allowed to access their own salary?**

```ruby
> input = {"method": "GET", "path": ["salary", "bob"], "user_id": "bob"}
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
> input = {"method": "GET", "path": ["salary", "bob"], "user_id": "alice"}
> allow
false
```

**Is Janet allowed to access Bob's salary?**

```ruby
> input = {"method": "GET", "path": ["salary", "alice"], "user_id": "janet"}
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
> input = {"app": {"tags": {"requires-pci-level": "3", "requires-eu": "true"}}}
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

- How Netflix Is Solving Authorization Across Their Cloud @ CloudNativeCon US 2017: [video](https://www.youtube.com/watch?v=R6tUNpRpdnY), [slides](https://www.slideshare.net/TorinSandall/how-netflix-is-solving-authorization-across-their-cloud).
- Policy-based Resource Placement in Kubernetes Federation @ LinuxCon Beijing 2017: [slides](https://www.slideshare.net/TorinSandall/policybased-resource-placement-across-hybrid-cloud), [screencast](https://www.youtube.com/watch?v=hRz13baBhfg&feature=youtu.be).
- Enforcing Bespoke Policies In Kubernetes @ KubeCon US 2017: [video](https://www.youtube.com/watch?v=llDI8VvkUj8), [slides](https://www.slideshare.net/TorinSandall/enforcing-bespoke-policies-in-kubernetes).
- Istio's Mixer: Policy Enforcement with Custom Adapters @ CloudNativeCon US 2017: [video](https://www.youtube.com/watch?v=czZLXUqzd24), [slides](https://www.slideshare.net/TorinSandall/istios-mixer-policy-enforcement-with-custom-adapters-cloud-nativecon-17).
- The Open Policy Agent Project @ Netflix OSS Meetup Season 5 Episode 1 (2017): [video](https://www.youtube.com/watch?v=SfpsnlQf5bY&feature=youtu.be&t=33m45s), [slides](https://www.slideshare.net/aspyker/netflix-oss-meetup-season-5-episode-1).


## License
[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2Fopen-policy-agent%2Fopa.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2Fopen-policy-agent%2Fopa?ref=badge_large)