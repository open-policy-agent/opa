# SSH and sudo Authorization

Host-level access controls are an important part of every organization's
security strategy. Using [Linux-PAM](http://tldp.org/HOWTO/User-Authentication-HOWTO/x115.html) and OPA
we can extend policy-based access control to SSH and sudo.

## Goals

This tutorial shows how you can use OPA and Linux-PAM to enforce fine-grained,
host-level access controls over SSH and sudo.

Linux-PAM can be configured to delegate authorization decisions to plugins
(shared libraries). In this case, we have created an OPA-based plugin that can
be configured to authorize SSH and sudo access. The OPA-based Linux-PAM plugin
used in this tutorial can be found at [open-policy-agent/contrib](https://github.com/open-policy-agent/contrib).

For this tutorial, our desired policy is:

* Admins can SSH into any host and run sudo commands.
* Normal users can SSH into hosts that they have *contributed* to.
* Normal users cannot run sudo commands.

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

First, create a docker-compose.yml file that runs OPA and the containers that
represent our backend and frontend hosts.

```shell
cat > docker-compose.yml <<EOF
version: '2'
services:
  opa:
    image: openpolicyagent/opa:0.5.1
    ports:
      - 8181:8181
    command:
      - "run"
      - "--server"
      - "--log-level=debug"
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
EOF
```

The docker-compose.yml file requires two other local files:
`frontend_host_id.json` and `backend_host_id.json`. These files are mounted
into the containers representing our hosts. The content of the file provides
*context* that the PAM module provides as input when executing queries
against OPA.

Create the extra files required by docker-compose.yml:

```shell
echo '{"host_id": "frontend"}' > frontend_host_id.json
echo '{"host_id": "backend"}' > backend_host_id.json
```

> In real-world scenarios, these files could contain arbitrary infomration that we want to expose to the policy.

Finally, run `docker-compose` to pull and run the containers.

```shell
docker-compose -f docker-compose.yml up
```

### 2. Load policies and data into OPA.

In another terminal, load the policies and data into OPA that will control access to the hosts.

First, create the policy that will control SSH access to hosts.

```shell
cat >ssh_authz.rego <<EOF
package ssh.authz

# By default, users are not authorized.
default allow = false

# Allow access to any user that has the "admin" role.
allow {
    data.roles["admin"][_] = input.user
}

# Allow access to any user who contributed to the code running on the host.
allow {
    data.hosts[input.host_identity.host_id].contributors[_] = input.user
}

# If the user is not authorized, then include an error message in the response.
errors["request denied by administrative policy"] {
    not allow
}
EOF
```

Push this policy into OPA:

```shell
curl -X PUT --data-binary @ssh_authz.rego \
  localhost:8181/v1/policies/ssh_authz
```

Next, create the policy that will control sudo access on the hosts.

```shell
cat >sudo_authz.rego <<EOF
package sudo.authz

# By default, users are not authorized.
default allow = false

# Allow sudo access to any user that has the "admin" role.
allow {
    data.roles["admin"][_] = input.user
}

# If the user is not authorized, include this error message in the response.
errors["user does not have role admin"] {
    not allow
}
EOF
```

Push this policy into OPA:

```shell
curl -X PUT --data-binary @sudo_authz.rego \
  localhost:8181/v1/policies/sudo_authz
```

Finally, load the data that represents our roles and contributors into OPA.

```shell
curl -X PUT localhost:8181/v1/data/roles -d \
'{
    "admin": ["ops"]
}'
```

```shell
curl -X PUT localhost:8181/v1/data/hosts -d \
'{
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
}'
```

### 3. Prepare SSH key to login to hosts.

This tutorial uses a special Docker image named `openpolicyagent/demo-pam` to simulate an SSH server. This image contains:

1. Pre-created Linux accounts for our users.
2. Pre-populated authorized_keys files for our users.

You must have the correct private key to SSH into the containers. Run the
following command to write the private key to a local file:

```shell
cat >ssh_authz.key <<EOF
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA5o4tuzLqNq8ZASzVuDxlzop/Odn2JKn48i8q7Yr2pl9gBUcx
cfpBIAoCh+v9GY3dBPSyyx6XSYyA+7ajSm8hQn167jX0WLItcz7r5vRYB3+aGfeo
sys8OXaloG+aaTPvK711HS7FxQLX9EHBzCHgVKW0438HT7vuPFmB5KRKRG5b6s9K
CUx4lig/J+gFYF3bxviBoL95y3PEY174pzqXVrnEpjCQqNCcytOSBYjEsu8Tq/72
KDL8EcUl4Q++lfN9ksPa5SsxlvQB+z/cJ1fEwMsdSN9RrxBeNtG8wLpk15S+Guki
UNqCYb60IOKm0nFSag5O0zhxePX0zyN+ar7NzwIDAQABAoIBAQCZ9Y/sVk+5PKxB
8KK3aP3DMxFKnJaWXTr03zKXdhjHeSEx5RzLtAYRUx3ljl1x1x4k1RMgOMlmQAFS
FeBtMFDRieGxeS42nKVlNDtr+vdd6oQJmyx4mQKajPSFcoF2h0vLtbSjTDydFw0G
+3Ji0qxvWki1Mnq7cA/jFRJ8kIlXr+XLUMrmM6aeOtQKGMZa9Tu0Jfd+2eaiajfm
3UdN6j5dHp5svtQHfrE1u9P7r9Tk+RE7jfUYLUPgCsl6OoReWyS9o583APSlDxxm
SsksDGSetCh96MugaxAN3+hI9wzr6nnZs2CK3Vff2/Nm2szh2PimNwl/ik/i0uKt
fCyxKijBAoGBAP0frr/RZh0Mt1EgUmGXdbuHlj7J5jmQuqEvARr3JISNYySkEA+V
IcHJbH6q9dnMjT+f0xXR2b9V8RC9ucd5OIan4T9MHltanPaBWBPRUHxExOOhqPbW
7rrLODH2vAkgGbHI6HFPyU/K7x2HavJ3XEVtx/FgvMKy/kT64HJndup3AoGBAOks
2KvG/QyOjo/kpW5SR7cudQdPM3E/L/Cuc6MUep0q1UJHWuDVDa8zlfSigEoITE/7
/AvqbBKP/O1O7T423ncEqvw3mHTFMpryJ1XU+mMF+cg024+NqZpKHfyjw8gOepHg
Wa9mZNSLar8J4hkI2ZrIkQ3I2NuZANmjvZ4UgjVpAoGAfAurwtsmtLPHnp09YhAs
pTNEIQ8moS1ZGKaFXyagocj8Pjecm1ZVTbedUNINW6gPzI9Rjc7ibA787VxdD/FL
D0p0a2WtNs3IQFGQzV11mQDGkFtoB1e7dJUku++TpNEzZlnz95vHJzBnUExNz/dI
o8myA4uJ1cyMKVfc6JPlxe8CgYB8QMyZBOmVhmXLodDR8ACNSbFNGtRT1ZMLUzsF
vQT1uXx43CM+SeoH4ZpYCTwJt1BLEwElrF64qYfjQTrE+2Ii1Bb1Xf7cwrSLwtxZ
LavblrSbDiet4JRvRm2iUfYjJiwEjiPchtjWNhDFClQ0ePXUOGqriMqegnLkhw+l
LFKSeQKBgDs/Vs5AXmj4VAdupEpUS+dJJ4y6wi8lPQtMwNfY/1/DNsoTNibA9u66
cLtyaoaL64UV8TsEPqdbt0Kbj1bqeOyheN0qiCNA5pufvj219a/i9qZW3zHJkwDc
ed4qCYnDpcfRhi+PyFrjFdtw072BdndRlosukmiBSvVguJJYEoTP
-----END RSA PRIVATE KEY-----
EOF
```

Set the file permissions on the private key.

```shell
chmod 600 ssh_authz.key
```

### 4. SSH and sudo as a user with the `admin` role.

First, let's try to access the hosts as the `ops` user. Recall, the `ops` user
has been granted the `admin` role (via the `PUT /data/roles` request above) and
users with the `admin` role can login to any host and perform sudo commands.

Login to the `frontend` host (which has SSH listening on port 2222) and run a command with sudo as the `ops` user.

```shell
ssh -p 2222 ops@localhost \
  -i ssh_authz.key -o StrictHostKeyChecking=no
sudo ls /
exit
```

### 5. SSH as a user without the `admin` role.

Let's try a user without the admin role. Recall, that a non-admin user can SSH
into any host that they have *contributed to*.

The `frontend-dev` user contributed code to the `frontend` host so they should be
able to login.

```shell
ssh -p 2222 frontend-dev@localhost \
  -i ssh_authz.key -o StrictHostKeyChecking=no
```

However, since `frontend-dev` is not an admin, they cannot run sudo commands.

```shell
sudo ls /
exit
```

Furthermore, since `frontend-dev` did not contribute to the code running on the
`backend` host (which has SSH listening on port 2223), they should not be able
to login.

```shell
ssh -p 2223 frontend-dev@localhost \
  -i ssh_authz.key -o StrictHostKeyChecking=no
```

### 6. Elevate a user's rights through policy.

Suppose a user needs to be temporarily granted extra privileges. E.g., the user
may need to debug an issue on a specific production host. In this case, we can
add an another policy to temporarily allow this.

Let's allow the `frontend-dev` user to run sudo commands on the `frontend` host.

```shell
cat >sudo_authz_special.rego <<EOF
package sudo.authz

allow {
  input.host_identity.host_id = "frontend"
  input.user = "frontend-dev"
}
EOF
```

Load this policy into OPA.

```shell
curl -X PUT --data-binary @sudo_authz_special.rego \
  localhost:8181/v1/policies/sudo_authz_special
```

Confirm that the user `frontend-dev` can login to the `frontend` host and run sudo commands.

```shell
ssh -p 2222 frontend-dev@localhost \
  -i ssh_authz.key -o StrictHostKeyChecking=no
sudo ls /
exit
```

Lastly, remove the policy from OPA and confirm the user's original rights are restored.

```shell
curl -i -X DELETE localhost:8181/v1/policies/sudo_authz_special
```

```shell
ssh -p 2222 frontend-dev@localhost \
  -i ssh_authz.key -o StrictHostKeyChecking=no
sudo ls /
exit
```

## Wrap Up

Congratulations for finishing the tutorial!

 You learned a number of things about SSH with OPA:

* OPA gives you fine-grained policy control over SSH and sudo on any host where you've
  installed OPA and its PAM module.
* You write allow/deny policies to control who has access to what.
* You can import external data into OPA and write policies that depend on.
  that data.

The code for this tutorial can be found in the
[open-policy-agent/contrib](https://github.com/open-policy-agent/contrib)
repository.
