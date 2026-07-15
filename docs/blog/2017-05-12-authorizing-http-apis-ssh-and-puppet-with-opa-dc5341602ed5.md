---
title: "Authorizing HTTP APIs, SSH, and Puppet with OPA"
authors: ["tsandall"]
date: 2017-05-12
slug: authorizing-http-apis-ssh-and-puppet-with-opa-dc5341602ed5
---

_This is a short post that shows how you can use the Open Policy Agent (OPA) project to enforce authorization policies across HTTP APIs, SSH, and Puppet. If you're interested in policy, authorization, compliance, or other related topics, check out [openpolicyagent.org](http://www.openpolicyagent.org/) or come chat with us on [Slack](http://slack.openpolicyagent.org)._

One goal of OPA is to solve authorization (who can do what) across the stack. To achieve this goal, OPA provides a **simple HTTP API to integrate at enforcement points and a** high-level declarative language to codify authorization policies. The policy language (Rego) is domain-agnostic and let's you define rich, fine-grained access controls over arbitrary JSON data.

When you write authorization policy in Rego, you're writing assertions over the state of the world represented as JSON. The state available to the authorization policy is provided either as **input to the authorization query or** pushed from an external data source and stored inside OPA. Because external state can be pushed into OPA, policies can leverage all kinds of context when making their authorization decisions.

Recently we built a handful of authorization integrations that use OPA at different points in the stack. As part of this effort we're reaching out to other projects that are looking to solve authorization in their domain. We've already built several integrations and examples spanning multiple layers:

- [Micro-service API authorization with Linkerd](https://github.com/open-policy-agent/contrib/tree/master/linkerd_authz)
- [SSH and sudo authorization with a custom PAM module](https://github.com/open-policy-agent/contrib/tree/master/pam_authz)
- [Provisioning authorization with Puppet](https://github.com/open-policy-agent/contrib/tree/master/puppet_example)

Let's look at some examples.

## HTTP API Authorization

This simple example shows how to limit read access to an employee's salary in a web app.

```rego
package httpapi.authz

default allow = false

# Allow users to get their own salaries.
allow {
    input.method = "GET"
    input.path = ["finance", "salary", user]
    user = input.user
}

# Allow managers to get their subordinates' salaries.
allow {
    input.method = "GET"
    input.path = ["finance", "salary", user]
    manager_of[user] = input.user
}
```

In this case, the policy allows exactly two people to access an employee's salary:

- The employee themselves (first rule)
- The manager of the employee (second rule)

This is a simplistic example but it helps show how OPA lets you leverage arbitrary data to make policy decisions. In this case, the second rule contains a reference to data ("manager_of") that maps an employee to their manager. For example:

```
{"alice": "bob", "charlie": "betty"}
```

It's worth pointing out that "manager_of" mapping could be defined statically or _dynamically_ in the policy itself. Defining "manager_of" statically would look very familiar:

```
manager_of = {"alice": "bob", "charlie": "betty"}
```

Alternatively, we could define "manager_of" dynamically based on some other data source (e.g., WorkDay, LDAP, etc.). For example:

```rego
package httpapi.authz

manager_of[employee] = manager {
    data.employees[employee].team = team_id
    data.teams[team_id].lead = manager
}
```

## SSH Authorization (using Linux-PAM)

This example shows how to restrict SSH access to users who have contributed to services running on individual hosts. Again, this policy shows how we can leverage external data to make policy decisions.

```rego
package ssh.authz

default allow = false

# Allow access to any user that has the "admin" role.
allow {
    data.roles["admin"][_] = input.user
}

# Allow access to any user who contributed to the code running on the host.
allow {
    data.hosts[input.host_identity.host_id].contributors[_] = input.user
}
```

In this case, the "roles" and "hosts" refer to external data loaded into OPA:

```json
{
  "hosts": {
    "frontend": {
      "contributors": [
        "frontend-dev"
      ]
    },
    "backend": {
      "contributors": [
        "backend-dev"
      ]
    }
  },
  "roles": {
    "admin": [
      "ops"
    ]
  }
}
```

With PAM you can also control who can run sudo commands. Since our PAM module offloads authorization decisions to OPA, we can extend our authorization policy to cover sudo access without changing any code in the enforcement point. This example shows how you can restrict sudo access to users with an _admin_ role:

```rego
package sudo.authz

default allow = false

# Allow sudo access to any user that has the "admin" role.
allow {
    data.roles["admin"][_] = input.user
}
```

## Puppet Authorization

Finally, let's look at how OPA can be used to enforce authorization decisions over more complex data structures such as Puppet catalogs.

In this case, we assume:

- **The infrastructure team is responsible for config stored inside /etc/infra
- **The app team is responsible for config stored inside /etc/app

```rego
package puppet.authz

default allow = false

allow { not deny }

deny {
    resource = catalog.resources[resource_index]
    resource.type = "File"
    startswith(resource.title, "/etc/infra")
    resource_author[resource_index] = email
    not infra_team[email]
}

deny {
    resource = catalog.resources[resource_index]
    resource.type = "File"
    startswith(resource.title, "/etc/infra")
    resource_author[resource_index] = email
    not infra_team[email]
}
```

This policy combines data from Puppet and Git (blame) to determine if an infrastructure team member has modified files belonging to the app team (or vice-versa).

## Wrap Up

If you're interested in trying out these examples, check out the OPA documentation:

- [HTTP API Authorization (Python)](http://www.openpolicyagent.org/tutorials/http-api-authorization/)
- [SSH and sudo Authorization](http://www.openpolicyagent.org/tutorials/ssh-sudo-authorization/)

We also have examples showing Puppet and Linkerd-based micro-service authorization that can be found in the [open-policy-agent/contrib](http://github.com/open-policy-agent/contrib) repository.

In upcoming posts we'll dive into more detail on authorization use cases such as conflict resolution, consistency guarantees, performance, visibility, and so on.

Thanks for reading!
