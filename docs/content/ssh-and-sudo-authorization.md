---
title: SSH and sudo
kind: tutorial
weight: 1
---

Host-level access controls are an important part of every organization's
security strategy. Using [Linux-PAM](http://tldp.org/HOWTO/User-Authentication-HOWTO/x115.html) and OPA
we can extend policy-based access control to SSH and sudo.

## Goals

This tutorial shows how you can use OPA and Linux-PAM to enforce fine-grained,
host-level access controls over SSH and sudo.

Linux-PAM can be configured to delegate authorization decisions to plugins
(shared libraries). In this case, we have created an OPA-based plugin that can
be configured to authorize SSH and sudo access. The OPA-based Linux-PAM plugin
used in this tutorial can be found at [open-policy-agent/contrib](https://github.com/open-policy-agent/contrib/tree/main/pam_opa).

For this tutorial, our desired policy is:

* Admins can SSH into any host and run sudo commands.
* Normal users can SSH into hosts that they have *contributed* to and run sudo commands.

Furthermore, we'll assume we have the following set of users and hosts:

* `frontend-dev` is a developer who contributes to the app running on the `frontend` host.
* `backend-dev` is a developer who contributes to the app running on the `backend` host.
* `ops` is an administrator for the organization.

Authentication (verifying user identity) is outside the scope of OPA's
responsibility so this tutorial relies on identities being statically
defined. In real-world scenarios authentication can be delegated to SSH itself
(authorized_keys) or other identity management systems.

Let's get started.

## Prerequisites

This tutorial requires [Docker Compose](https://docs.docker.com/compose/install/) to run dummy SSH hosts along
with OPA. The dummy SSH hosts are just containers running sshd inside.

## Steps

### 1. Bootstrap the tutorial environment using Docker Compose.

First, create a `tutorial-docker-compose.yaml` file that runs OPA and the containers that
represent our backend and frontend hosts.

**tutorial-docker-compose.yaml**:

```yaml
version: '2'
services:
  opa:
    image: openpolicyagent/opa:{{< current_docker_version >}}
    ports:
      - "8181:8181"
    # WARNING: OPA is NOT running with an authorization policy configured. This
    # means that clients can read and write policies in OPA. If you are
    # deploying OPA in an insecure environment, be sure to configure
    # authentication and authorization on the daemon. See the Security page for
    # details: https://www.openpolicyagent.org/docs/security.html.
    command:
      - "run"
      - "--server"
      - "--set=decision_logs.console=true"
      - "--set=services.nginx.url=http://bundle_server"
      - "--set=bundles.nginx.service=nginx"
      - "--set=bundles.nginx.resource=bundles/bundle.tar.gz"
    depends_on:
      - bundle_server
  frontend:
    image: openpolicyagent/demo-pam
    ports:
      - "2222:22"
    volumes:
      - ./frontend_host_id.json:/etc/host_identity.json
  backend:
    image: openpolicyagent/demo-pam
    ports:
      - "2223:22"
    volumes:
      - ./backend_host_id.json:/etc/host_identity.json
  bundle_server:
    image: nginx:1.20.0-alpine
    ports:
      - 8888:80
    volumes:
      - ./bundles:/usr/share/nginx/html/bundles
```

The `tutorial-docker-compose.yaml` file requires two other local files:
`frontend_host_id.json` and `backend_host_id.json`. These files are mounted
into the containers representing our hosts. The content of the file provides
*context* that the PAM module provides as input when executing queries
against OPA.

Create the extra files required by tutorial-docker-compose.yaml:

```shell
echo '{"host_id": "frontend"}' > frontend_host_id.json
echo '{"host_id": "backend"}' > backend_host_id.json
```

> In real-world scenarios, these files could contain arbitrary information that we want to expose to the policy.

Finally, run `docker-compose` to pull and run the containers.

```shell
docker-compose -f tutorial-docker-compose.yaml up
```
This tutorial uses a special Docker image named `openpolicyagent/demo-pam` to simulate an SSH server.
This image contains pre-created Linux accounts for our users, and the required PAM module is
pre-configured inside the `sudo` and `sshd` files in `/etc/pam.d/`.

### 2. Create a Bundle for the policies and data.

In another terminal, create the policies and data that OPA will use to control access to the hosts.

First, create folder called bundles and cd into it.

```bash
mkdir bundles
cd bundles
```

Next, create a policy that will tell the PAM module to collect context that is required for authorization.
For more details on what this policy should look like, see [this documentation](https://github.com/open-policy-agent/contrib/tree/main/pam_opa/pam#pull).

**pull.rego**:

```live:ssh_pull:module:read_only
package pull

# Which files should be loaded into the context?
files := ["/etc/host_identity.json"]

# Which environment variables should be loaded into the context?
env_vars := []
```


Create the policies that will authorize SSH and sudo requests.
The `input` which makes up the authorization context in the policy below will also
include some default values, such as the username making the request. See
[this documentation](https://github.com/open-policy-agent/contrib/tree/main/pam_opa/pam#authz)
to get a better understanding of what the `input` to the authorization policy will look like.

Unlike the *pull* policy, we'll create separate *authz* policies
for SSH and `sudo` for more fine-grained control.
In production, it makes more sense to have this separation for *display* and *pull* as well.

Create the SSH authorization policy. It should allow admins to SSH into all hosts,
and non-admins to only SSH into hosts that they contributed code to.

**sshd_authz.rego**:

```live:sshd_authz:module:read_only
package sshd.authz

import input.pull_responses
import input.sysinfo

import data.hosts

# By default, users are not authorized.
default allow := false

# Allow access to any user that has the "admin" role.
allow {
    data.roles["admin"][_] == input.sysinfo.pam_username
}

# Allow access to any user who contributed to the code running on the host.
#
# This rule gets the "host_id" value from the file "/etc/host_identity.json".
# It is available in the input under "pull_responses" because we
# asked for it in our pull policy above.
#
# It then compares all the contributors for that host against the username
# that is asking for authorization.
allow {
    hosts[pull_responses.files["/etc/host_identity.json"].host_id].contributors[_] == sysinfo.pam_username
}

# If the user is not authorized, then include an error message in the response.
errors["Request denied by administrative policy"] {
    not allow
}
```



Create the `sudo` authorization policy. It should allow only admins to use `sudo`.

**sudo_authz.rego**:

```live:sudo_authz:module:read_only
package sudo.authz

# By default, users are not authorized.
default allow := false

# Allow access to any user that has the "admin" role.
allow {
    data.roles["admin"][_] == input.sysinfo.pam_username
}

# If the user is not authorized, then include an error message in the response.
errors["Request denied by administrative policy"] {
    not allow
}
```



Now we need to create the data that represents our roles, hots, and contributors into OPA.

Create a folder called roles, and the following data file.

```shell
mkdir roles
cat <<EOF > roles/data.json
{
    "admin": ["ops"]
}
EOF

```

Create a folder called hosts, and the following data file.
```shell
mkdir hosts
cat <<EOF > hosts/data.json
{
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
}
EOF
```

Finally create the bundle for the bundle server to use.

```bash
opa build -b .
```

Now you should have the following file structure setup.

```
.
└── tutorial-docker-compose.yaml
├── backend_host_id.json
├── frontend_host_id.json
├── bundles
│   ├── bundle.tar.gz
│   ├── pull.rego
│   ├── sshd_authz.rego
│   ├── sudo_authz.rego
│   ├── hosts
│   │   └── data.json
│   ├── roles
│   │   └── data.json
```


### 3. SSH and sudo as a user with the `admin` role.

First, let's try to access the hosts as the `ops` user. Recall, the `ops` user
has been granted the `admin` role (via the `PUT /data/roles` request above) and
users with the `admin` role can login to any host and perform sudo commands.

Login to the `frontend` host (which has SSH listening on port 2222) and run a command with sudo as the `ops` user.

```shell
ssh -p 2222 ops@localhost \
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null

sudo ls /
exit
```

You will see a lot of verbose logs from `sudo` as the PAM module goes through the motions.
This is intended so you can study how the PAM module works.
You can disable verbose logging by changing the `log_level` argument in the PAM
configuration. For more details see
[this documentation](https://github.com/open-policy-agent/contrib/tree/main/pam_opa/pam#configuration).

### 4. SSH as a user without the `admin` role.

Let's try a user without the admin role. Recall, that a non-admin user can SSH
into any host that they have *contributed to*.

The `frontend-dev` user contributed code to the `frontend` host so they should be
able to login.

```shell
ssh -p 2222 frontend-dev@localhost \
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null
```

Only admins can use `sudo`, so you shouldn't be able to run `sudo ls /`.

Since `frontend-dev` did not contribute to the code running on the
`backend` host (which has SSH listening on port 2223), they should not be able
to login.

```shell
ssh -p 2223 frontend-dev@localhost \
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null
```

### 5. Elevate a user's rights through policy.

Suppose you have a ticketing system for elevation, where you generate tickets for users
that need elevated rights, send the ticket to the user, and expire those tickets when
their rights should be removed.

Let's mock the current state of this simple ticketing system's API with some data.

```shell
mkdir elevate
cat <<EOF > elevate/data.json
{
  "tickets": {
    "frontend-dev": "1234"
  }
}
EOF
```

This means that for now, if the `frontend-dev` user can provide ticket number `1234`,
they should be able to SSH into all servers.

Let's write policy to ensure that this happens.

First, we need to make the PAM module take input from the user.

**display.rego**:

```live:display:module:read_only
package display

# What should be prompted to the user?
display_spec := [
  {
    "message": "Please enter an elevation ticket if you have one:",
    "style": "prompt_echo_on",
    "key": "ticket"
  }
]
```



Then we need to make sure that the authorization takes this input into account.

**sudo_authz_elevated.rego**:

```live:sudo_authz/elevate:module:read_only
# A package can be defined across multiple files.
package sudo.authz

import data.elevate
import input.sysinfo
import input.display_responses

# Allow this user if the elevation ticket they provided matches our mock API
# of an internal elevation system.
allow {
    elevate.tickets[sysinfo.pam_username] == display_responses.ticket
}
```

Now we need to build a new bundle for OPA to use.

```shell
opa build -b .
```

Confirm that the user `frontend-dev` can indeed use `sudo`.

```shell
ssh -p 2222 frontend-dev@localhost \
  -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null

sudo ls /
```

You should be prompted with the message that we defined in our *display* policy
for both the SSH and `sudo` authorization cycles.
This happens because the *display* policy is shared by the PAM configurations of SSH and `sudo`.
In production, it is more practical to use separate policy packages for each PAM configuration.

We have not defined the SSH *authz* policy to work with elevation, so you can enter any value
into the prompt that comes up for for SSH.

For `sudo`, enter the ticket number `1234` to get access.

Lastly, update the mocked elevation API and confirm the user's original rights are restored.

```shell
cat <<EOF > elevate/data.json
{
  "tickets": {}
}
EOF
```

Once again, build the bundle with this new data

```bash
opa build -b .
```

You will find that running `sudo ls /` as the `frontend-dev` user is disallowed again.

It is possible to configure the *display* policy to only make the PAM module prompt for the
elevation ticket when our mock API has a non-empty `tickets` object. So when there are no
elevated users, there will be no prompt for a ticket. This can be done using the Rego
[`count` aggregate](../policy-reference/#aggregates).

## Wrap Up

Congratulations for finishing the tutorial!

 You learned a number of things about SSH with OPA:

* OPA gives you fine-grained access control over SSH, `sudo`, and any other application that uses PAM.
  Although this tutorial used the some of the same policies for both
  SSH and sudo, you should use separate, fine-grained policies for each application that supports PAM.
* Writing allow/deny policies to control who has access to what using context from the user and host.
* Importing external data into OPA and writing policies that depend on that data.

The code for the PAM module used in this tutorial can be found in the
[open-policy-agent/contrib](https://github.com/open-policy-agent/contrib)
repository.
