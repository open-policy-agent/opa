# ![logo](./logo/logo.png) Open Policy Agent

[![Slack Status](http://slack.openpolicyagent.org/badge.svg)](http://slack.openpolicyagent.org) [![Build Status](https://travis-ci.org/open-policy-agent/opa.svg?branch=master)](https://travis-ci.org/open-policy-agent/opa) [![Go Report Card](https://goreportcard.com/badge/open-policy-agent/opa)](https://goreportcard.com/report/open-policy-agent/opa)

The Open Policy Agent (OPA) is an open source, general-purpose policy engine
that enables unified, context-aware policy enforcement across the entire stack.

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
