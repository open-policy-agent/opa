---
title: Docker
kind: tutorial
weight: 1
---

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

  * Docker Engine 18.06.0-ce or newer
  * Docker API version 1.38 or newer
  * `root` or `sudo` access
  * Nginx, or any capable [bundle](https://www.openpolicyagent.org/docs/latest/management-bundles/) server

The tutorial has been tested on the following platforms:

  * Ubuntu 20.04 (64-bit)

If you are using a different distro, OS, or architecture, the steps will be the
same. However, there may be slight differences in the commands you need to run.

## Steps

Several of the steps below require `root` or `sudo` access. When you are
modifying files under `/etc/docker` or signalling the Docker daemon to
restart, you will need root access.

### 1. Create an empty policy definition that will allow all requests.

**authz.rego**:

```live:docker_authz:module:read_only
package docker.authz

allow := true
```

This policy defines a single rule named `allow` that always produces the
decision `true`. Once all the components are running, we will come back to
the policy.

### 2. Create policy bundle and OPA configuration.

For the purpose of this example, we are going to use [Nginx](https://www.openpolicyagent.org/docs/latest/management-bundles/#nginx)
to serve bundles from the same machine Docker is running on.

With nginx running, simply build the policy bundle placed into the nginx web root directory.

```shell
opa build --bundle --output /var/www/html/bundle.tar.gz .
```

Next, create an OPA configuration file pointing to the bundle.

```yaml
services:
  authz:
    url: http://localhost

bundles:
  authz:
    service: authz
    resource: bundle.tar.gz

# Optional - Print decisions in the Docker logs. Configure a remote service for production use cases.
decision_logs:
  console: true
```

Save the above file as `config.yaml`. We'll need to place this somewhere where the plugin can find it.
The `/etc/docker` directory will be mounted as `/opa` in the container running the plugin, so let's create a
sub-directory for our configuration file there.

```shell
sudo mkdir -p /etc/docker/config
sudo mv config.yaml /etc/docker/config/
```

### 3. Install the opa-docker-authz plugin.

Install the `opa-docker-authz` plugin and point it to the config file just created.

```shell
docker plugin install openpolicyagent/opa-docker-authz-v2:0.8 opa-args="-config-file /opa/config/config.yaml"
```

You need to configure the Docker daemon to use the plugin for authorization.

```shell
cat > /etc/docker/daemon.json <<EOF
{
    "authorization-plugins": ["openpolicyagent/opa-docker-authz-v2:0.8"]
}
EOF
```

Signal the Docker daemon to reload the configuration file.

```shell
kill -HUP $(pidof dockerd)
```

### 4. Run a simple Docker command to make sure everything is still working.

```shell
docker ps
```

If everything is set up correctly, the command should exit successfully. You can
expect to see log messages from OPA and the plugin.

### 5. Test that the policy definition is working.

Let’s modify our policy to **deny** all requests:

**authz.rego**:

```live:docker_authz_deny_all:module:read_only
package docker.authz

allow := false
```

Rebuild the bundle and save it in the Nginx document root directory.

```shell
opa build --bundle --output /var/www/html/bundle.tar.gz .
```

In OPA, rules defines the content of documents. Documents be boolean values
(true/false) or they can represent more complex structures using arrays,
objects, strings, etc.

In the example above we modified the policy to always return `false` so that
requests will be rejected.

```shell
docker ps
```

The output should be:

```shell
Error response from daemon: authorization denied by plugin opa-docker-authz: request rejected by administrative policy
```

To learn more about how rules define the content of documents, see: [How Does OPA Work?](../#overview)

With this policy in place, users will not be able to run any Docker commands. Go
ahead and try other commands such as `docker run` or `docker pull`. They will
all be rejected.

Now let's change the policy so that it's a bit more useful.

### 6. Update the policy to reject requests with the unconfined [seccomp](https://en.wikipedia.org/wiki/Seccomp) profile:

**authz.rego**:

```live:docker_authz_deny_unconfined:module:openable
package docker.authz

default allow := false

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
```

Again, rebuild the bundle and save it in the Nginx document root directory.

```shell
opa build --bundle --output /var/www/html/bundle.tar.gz .
```

The plugin queries the `allow` rule to authorize requests to Docker. The `input`
document is set to the attributes passed from Docker.

```live:docker_authz_deny_unconfined:query:hidden
allow
```

```live:docker_authz_deny_unconfined:input
{
  "AuthMethod": "",
  "Body": {
    "AttachStderr": true,
    "AttachStdin": false,
    "AttachStdout": true,
    "Cmd": null,
    "Domainname": "",
    "Entrypoint": null,
    "Env": [],
    "HostConfig": {
      "AutoRemove": false,
      "Binds": null,
      "BlkioDeviceReadBps": null,
      "BlkioDeviceReadIOps": null,
      "BlkioDeviceWriteBps": null,
      "BlkioDeviceWriteIOps": null,
      "BlkioWeight": 0,
      "BlkioWeightDevice": [],
      "CapAdd": null,
      "CapDrop": null,
      "Cgroup": "",
      "CgroupParent": "",
      "ConsoleSize": [
        0,
        0
      ],
      "ContainerIDFile": "",
      "CpuCount": 0,
      "CpuPercent": 0,
      "CpuPeriod": 0,
      "CpuQuota": 0,
      "CpuRealtimePeriod": 0,
      "CpuRealtimeRuntime": 0,
      "CpuShares": 0,
      "CpusetCpus": "",
      "CpusetMems": "",
      "DeviceCgroupRules": null,
      "Devices": [],
      "DiskQuota": 0,
      "Dns": [],
      "DnsOptions": [],
      "DnsSearch": [],
      "ExtraHosts": null,
      "GroupAdd": null,
      "IOMaximumBandwidth": 0,
      "IOMaximumIOps": 0,
      "IpcMode": "",
      "Isolation": "",
      "KernelMemory": 0,
      "Links": null,
      "LogConfig": {
        "Config": {},
        "Type": ""
      },
      "MaskedPaths": null,
      "Memory": 0,
      "MemoryReservation": 0,
      "MemorySwap": 0,
      "MemorySwappiness": -1,
      "NanoCpus": 0,
      "NetworkMode": "default",
      "OomKillDisable": false,
      "OomScoreAdj": 0,
      "PidMode": "",
      "PidsLimit": 0,
      "PortBindings": {},
      "Privileged": false,
      "PublishAllPorts": false,
      "ReadonlyPaths": null,
      "ReadonlyRootfs": false,
      "RestartPolicy": {
        "MaximumRetryCount": 0,
        "Name": "no"
      },
      "SecurityOpt": null,
      "ShmSize": 0,
      "UTSMode": "",
      "Ulimits": null,
      "UsernsMode": "",
      "VolumeDriver": "",
      "VolumesFrom": null
    },
    "Hostname": "",
    "Image": "hello-world",
    "Labels": {},
    "NetworkingConfig": {
      "EndpointsConfig": {}
    },
    "OnBuild": null,
    "OpenStdin": false,
    "StdinOnce": false,
    "Tty": false,
    "User": "",
    "Volumes": {},
    "WorkingDir": ""
  },
  "Headers": {
    "Content-Length": "1470",
    "Content-Type": "application/json",
    "User-Agent": "Docker-Client/18.06.1-ce (linux)"
  },
  "Method": "POST",
  "Path": "/v1.38/containers/create",
  "User": ""
}
```

For the input above, the value of `allow` is:

```live:docker_authz_deny_unconfined:output
```

> Many of the examples in the documentation are interactive. Try editing the
> input above by setting `Body.HostConfig.SecurityOpt` to
> `["seccomp:unconfined"]`.

### 7. Test the policy is working by running a simple container:

```shell
docker run hello-world
```

Now try running the same container but disable seccomp (which should be
prevented by the policy):

```shell
docker run --security-opt seccomp:unconfined hello-world
```

Congratulations! You have successfully prevented containers from running without
seccomp!

The rest of the tutorial shows how you can grant fine-grained access to specific
clients.

### <a name="identify-user"></a> 8. Identify the user in Docker requests.

> Back up your existing Docker configuration, just in case. You can replace your
> original configuration after you are done with the tutorial.

```shell
mkdir -p ~/.docker
cp ~/.docker/config.json ~/.docker/config.json~
```

To identify the user, include an HTTP header in all of the requests sent to the
Docker daemon:

```shell
cat >~/.docker/config.json <<EOF
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

```live:docker_authz_users:module:read_only,openable
package docker.authz

default allow := false

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
users := {
    "bob": {"readOnly": true},
    "alice": {"readOnly": false},
}
```

### 10. Attempt to run a container.

Because the configured user is `"bob"`, the request is rejected:

```shell
docker run hello-world
```

### 11. Change the user to "alice" and re-run the container.

```shell
cat > ~/.docker/config.json <<EOF
{
    "HttpHeaders": {
        "Authz-User": "alice"
    }
}
EOF
```

Because the configured user is `"alice"`, the request will succeed:

```shell
docker run hello-world
```

That's it!
