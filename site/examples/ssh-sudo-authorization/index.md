---
sort_order: 1001
nav_id: MAIN_EXAMPLES
xmp_id: SSH_SUDO_AUTHORIZATION
layout: examples

title: SSH and sudo Authorization
---

{% contentfor header %}
# SSH and sudo Authorization

Many times it's convenient to have fine-grained, policy-based control over who can
login to each of your servers and containers.

This is an excellent opportunity to see how to policy enable SSH and sudo using a PAM module.

{% endcontentfor %}

{% contentfor body %}

## Goals

To demonstrate the benefits of the OPA PAM module, we created a docker image
with an ssh server and the PAM module installed and configured. The instructions
that follow take you through the steps of building the docker image, adding
policy, and sshing into the box.

What you'll see is that you can flexibly write and change policy that controls
who can ssh into the box and who can elevate to sudo rights without writing a
line of code, modifying LDAP groups, changing configuration management tools,
digging around inside the server's internals, or changing keys.  More
importantly, you use the same language for controlling ssh as you do for
controlling sudo, which is the same language you use for controlling Kubernetes,
docker, and any of the other systems integrated with OPA.

In this example, we want to enforce the following policy.

* Admin users can ssh into any server
* Developers can ssh into servers whose role match the developers role
* Admin users can use sudo
* No other users can use sudo


## Prerequisites

This example requires:

  * [docker-compose](https://docs.docker.com/compose/install/

The example has been tested on the following platforms:

  * Mac OS X 10.11 with Docker 17.04

If you are using a different distro, OS, or architecture, the steps will be the same. However, there may be slight differences in the commands you need to run.

## Steps

### 1. Download the contrib repo from github.
All of the source code for the PAM module, and docker containers for the demo can be found in the `open-policy-agent/contrib` repository on github.

```shell
$ git clone https://github.com/open-policy-agent/contrib.git
```

### 2. Build the PAM module and docker containers for the servers

```shell
$ cd contrib/pam_authz
$ make
```

After this completes, `docker images` will show the
following images: `openpolicyagent/demo-pam` and `openpolicyagent/pam-builder`. The builder
image is used for compiling the PAM module, and the other image is our
example server with the PAM module installed.

### 3. Start the servers and OPA containers
The two docker-containers above call into
OPA for policy decisions, so for the demo we have 3 containers (2 "servers" and
1 OPA).  To simplify startup, we included a docker-compose file. Make sure that
you have [docker-compose](https://docs.docker.com/compose/install/) installed,
and then use `make` to run `docker-compose`.

```shell
$ make up
```

### 4. Prepare to SSH

Using the docker-compose file, OPA is preconfigured with policies for ssh and
sudo authorization. When a user attempts to connect to the ssh server,
authentication is performed first. For our demo, we use standard Linux users and
ssh keys for this. Once a user is authenticated, they are authorized by the PAM
module. To see this in action, we pre-configured the server containers with a
few users and group those users into two categories: engineering or admin.  The
key you will use for sshing can be found in `docker/keys/id_rsa`.  Make sure that
permissions are set correctly on that file:

```shell
$ chmod 0600 docker/keys/id_rsa
```

Because this demo has you sshing into docker containers, you will ssh into the
IP address that docker is using (aliased as `docker_ip` shown below), and you
will use port number 2222 for the web-app container and 2223 for the backend
container. Usually the servers/containers you will install the PAM module onto
will have public IPs or hostnames, and you'll ssh into them in the usual way,
providing just the IP or hostname.

For convenience, set up an alias to the docker IP as shown below.  First
open up a new terminal.

```
# The following line is only for Mac users using docker machine
$ docker_ip=`docker-machine ip default`
# Linux users and Mac users not using docker machine should use the following
$ docker_ip=localhost
```

### 5. SSH and sudo as Admin user

Now let's try and access the servers using ssh. Recall that users in the admin
group (as defined in policy) have ssh access to any server and can perform sudo
commands. The user `ops-dev` is an admin, so run the following commands and see
that they all succeed.

```shell
# ssh into webapp, with sudo
$ ssh -i docker/keys/id_rsa -p 2222 ops-dev@$docker_ip
$ sudo ls /
$ logout
# ssh into backend, with sudo
$ ssh -i docker/keys/id_rsa -p 2223 ops-dev@$docker_ip
$ sudo ls /
$ logout
```

### 6. SSH and sudo as Non-admin user

Let's try a user not in the admin group. Recall that a non-admin can ssh into
any server running an app they wrote code for.   The user `web-dev` contributes
to the webapp server, but not the backend. `web-dev` can ssh into servers with
the `webapp` role, but not any other servers. Further, this user should not be
able to use sudo.

```shell
# ssh into webapp, but no sudo
$ ssh -i docker/keys/id_rsa -p 2222 web-dev@$docker_ip
$ sudo ls /
$ logout
# no ssh into backend
$ ssh -i docker/keys/id_rsa -p 2223 web-dev@$docker_ip
```

### 7. Elevate rights and SSH again

To this point, the policy could have been hard coded into the PAM module. OPA,
however, allows us to dynamically modify the policies and the data that policies
rely on. Consider the user "Pam". In our initial data, Pam has contributed to
the WebApp, so can only ssh into the container with the webapp role. Suppose
that Pam makes changes to the backend as well, and so Pam should be added to the
list of Backend contributers. We use OPA's API to update the data like so:

```
curl -X PUT $docker_ip:8181/v1/data/io/vcs/contributors -d \
'{
    "WebApp": {
        "Contributors": ["web-dev", "Pam", "Hans"]
    },
    "Backend": {
        "Contributors": ["web-dev", "backend-dev", "Stan", "Hans"]
    }
}
'
```

Now see that the `web-dev` can ssh into the Backend server (but cannot sudo).

```shell
$ ssh -i docker/keys/id_rsa -p 2223 web-dev@$docker_ip
```


### 8. Elevate rights for a different user

As another example, suppose that a bug occurs in production and Pam needs sudo
access to debug. We can update the list of admins to give Pam sudo access like
so:

```
curl -X PUT $docker_ip:8181/v1/data/io/directory/users -d \
'{
    "engineering": ["web-dev", "backend-dev" ,"ops-dev", "Stan", "Hans", "Pam", "Sam", "Jan"],
    "admin": ["Pam", "ops-dev"]
}'
```

Now Pam has ssh and sudo access on all servers.

```shell
$ ssh -i docker/keys/id_rsa -p 2223 Pam@$docker_ip
```

## Orchestration of Multiple Servers

This example shows how OPA and PAM can be used for authorization on a
single server, but in practice policy needs to be enforced across many servers.
Because OPA is a host-local daemon, we recommend using a standalone, centralized
service (like etcd) to store policy and data centrally and a side-car (or
wrapper) for OPA that keeps all the OPAs up to date with that central service.
(OPA was purpose-built to be a host-local daemon to ensure high-availability and
high-performance even in the presence of network partitions and leaves the
problem of policy/data replication up to the environment in which it is
deployed.)


## Wrap Up
Congratulations for finishing the tutorial!  You learned a number of things
about SSH with OPA.

* OPA gives you fine-grained policy control over SSH and sudo on any server where you've
  installed OPA and its PAM module
* You write allow/deny policies to control who has access to what
* You can import external data into OPA and write policies that depend on
  that data.




{% endcontentfor %}
