# Docker Authorization

Docker’s out-of-the-box authorization model is all or nothing. But many users
require finer-grained access control and Docker’s plugin infrastructure allows
us to do so.

This is an excellent opportunity to see how to policy enable an existing
service.

## Goals

This tutorial helps you get started with OPA and introduces you to core concepts
in OPA.

> Policy enabling an application decouples the policy implementation from the
> business logic so that administrators can define policy without changing the
> application while still keeping up with the size, complexity, and dynamic
> nature of modern applications.

For the purpose of this tutorial, we want to use OPA to enforce a policy that
prevents users from running insecure containers.

This tutorial illustrates two key concepts:

  1. OPA policy definition is decoupled from the implementation of the service
     (in this case Docker). The administrator is empowered to define and manage
     policies without requiring changes to any of the apps.

  2. Both the data relevant to policy and the policy definitions themselves can
     change rapidly.

## Prerequisites

This tutorial requires:

  * Docker Engine 18.03.0-ce or newer
  * Docker API version 1.30 or newer
  * `root` or `sudo` access

The tutorial has been tested on the following platforms:

  * Ubuntu 16.04 (64-bit)

If you are using a different distro, OS, or architecture, the steps will be the
same. However, there may be slight differences in the commands you need to run.

## Steps

### 1. Create an empty policy definition that will allow all requests.

```shell
$ mkdir policies && cat >policies/example.rego <<EOF
package docker.authz

allow = true
EOF
```

This policy defines a single rule named `allow` that always produces the
decision `true`. Once all of the components are running, we will come back to
the policy.

### 2. Run the opa-docker-authz plugin and then open another terminal.

```shell
$ docker run -d --restart=always \
    -v $PWD/policies:/policies \
    -v /run/docker/plugins:/run/docker/plugins \
    openpolicyagent/opa-docker-authz:0.2 \
    -policy-file /policies/example.rego
```

### 3. Reconfigure Docker.

Docker must include the following command-line argument:

```shell
--authorization-plugin=opa-docker-authz
```

On Ubuntu 16.04 with systemd, this can be done as follows (requires root):

```shell
$ sudo mkdir -p /etc/systemd/system/docker.service.d
$ sudo tee -a /etc/systemd/system/docker.service.d/override.conf > /dev/null <<EOF
[Service]
ExecStart=
ExecStart=/usr/bin/dockerd -H fd:// --authorization-plugin=opa-docker-authz
EOF
$ sudo systemctl daemon-reload
$ sudo service docker restart
```

If you are using a different Linux distribution or you are not running systemd,
the process will be slightly different.

### 4. Run a simple Docker command to make sure everything is still working.

```shell
$ docker ps
```

If everything is setup correctly, the command should exit successfully. You can
expect to see log messages from OPA and the plugin.

### 5. Test that the policy definition is working.

Let’s modify our policy to **deny** all requests:

```shell
$ cat >policies/example.rego <<EOF
package docker.authz

allow = false
EOF
```

In OPA, rules defines the content of documents. Documents be boolean values
(true/false) or they can represent more complex structures using arrays,
objects, strings, etc.

In the example above we modified the policy to always return `false` so that
requests will be rejected.

```shell
$ docker ps
```

The output should be:

```
Error response from daemon: authorization denied by plugin opa-docker-authz: request rejected by administrative policy
```

To learn more about how rules define the content of documents, see: [How Does OPA Work?](/how-does-opa-work.md)

With this policy in place, users will not be able to run any Docker commands. Go
ahead and try other commands such as `docker run` or `docker pull`. They will
all be rejected.

Now let's change the policy so that it's a bit more useful.

### 6. Update the policy to reject requests with the unconfined [seccomp](https://en.wikipedia.org/wiki/Seccomp) profile:

```shell
$ cat >policies/example.rego <<EOF
package docker.authz

default allow = false

allow {
    not deny
}

deny {
    seccomp_unconfined
}

seccomp_unconfined {
    # This expression asserts that the string on the right-hand side is equal
    # to an element in the array SecurityOpt referenced on the left-hand side.
    input.Body.HostConfig.SecurityOpt[_] == "seccomp:unconfined"
}
EOF
```

### 7. Test the policy is working by running a simple container:

```shell
$ docker run hello-world
```

Now try running the same container but disable seccomp (which should be
prevented by the policy):

```shell
$ docker run --security-opt seccomp:unconfined hello-world
```

Congratulations! You have successfully prevented containers from running without
seccomp!

The rest of the tutorial shows how you can grant fine grained access to specific
clients.

### <a name="identify-user"></a> 8. Identify the user in Docker requests.

> Back up your existing Docker configuration, just in case. You can replace your
> original configuration after you are done with the tutorial.

```shell
$ mkdir -p ~/.docker
$ cp ~/.docker/config.json ~/.docker/config.json~
```

To identify the user, include an HTTP header in all of the requests sent to the
Docker daemon:

```shell
$ cat >~/.docker/config.json <<EOF
{
    "HttpHeaders": {
        "Authz-User": "bob"
    }
}
EOF
```

> Docker does not currently provide a way to authenticate clients. But in Docker
> 1.12, clients can be authenticated using TLS and there are plans to include
> other means of authentication. For the purpose of this tutorial, we assume that
> an authentication system is place.

### 9. Update the policy to include basic user access controls.

```shell
$ cat >policies/example.rego <<EOF
package docker.authz

default allow = false

# allow if the user is granted read/write access.
allow {
    user_id := input.Headers["Authz-User"]
    user := users[user_id]
    not user.readOnly
}

# allow if the user is granted read-only access and the request is a GET.
allow {
    user_id := input.Headers["Authz-User"]
    users[user_id].readOnly
    input.Method == "GET"
}

# users defines permissions for the user. In this case, we define a single
# attribute 'readOnly' that controls the kinds of commands the user can run.
users = {
    "bob": {"readOnly": true},
    "alice": {"readOnly": false},
}
EOF
```

### 10. Attempt to run a container.

Because the configured user is `"bob"`, the request is rejected:

```shell
$ docker run hello-world
```

### 11. Change the user to "alice" and re-run the container.

```shell
$ cat > ~/.docker/config.json <<EOF
{
    "HttpHeaders": {
        "Authz-User": "alice"
    }
}
EOF
```

Because the configured user is `"alice"`, the request will succeed:

```shell
$ docker run hello-world
```

### 12. Restore your original configuration.

See: “[Reconfigure Docker.](#reconfigure-docker)” and “[Identify the user in Docker requests.](#identify-user)”.

That's it!
