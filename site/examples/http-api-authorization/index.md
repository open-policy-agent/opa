---
sort_order: 1001
nav_id: MAIN_EXAMPLES
xmp_id: DOCKER_AUTHORIZATION
layout: examples

title: API Authorization
---

{% contentfor header %}
# API Authorization

Anything that exposes an HTTP API (whether an individual microservice or an application as a whole) needs to control who can run those APIs and when.  OPA makes it easy to write fine-grained, context-aware policies to implement API authorization.

{% endcontentfor %}

{% contentfor body %}

## Goals

In this example, you'll use a simple HTTP web server that accepts any HTTP GET
request that you issue and echoes the OPA decision back as text.  Both OPA and
the web server will be run as containers.

For this example, our desired policy is:

* People can see their own salaries (GET /finance/salary/<user> is permitted for <user>)
* A manager can see their direct reports' salaries (GET /finance/salary/<user> is permitted for <user>'s manager)


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

### 2. Build Webserver and OPA containers

```shell
$ cd contrib/api_authz
$ make
```


### 3. Start the containers
The repository contains a docker-compose file to make it easy to configure and start the containers.

```shell
$ make up
```

### 4. Prepare to make HTTP requests to a container
You'll be sending HTTP requests to a docker container, so it's useful to
setup an alias to the IP that docker uses.  Open a different terminal from the one
you used to run `make up`.

```shell
# The following line is only for Mac users using docker machine
$ docker_ip=`docker-machine ip default`
# Linux users and Mac users not using docker machine should use the following
$ docker_ip=localhost
```

### 5. Check that `alice` can see her own salary

The following command will succeed.

```
$ curl --user alice:password $docker_ip:5000/finance/salary/alice
```

### 6. Check that `bob` can see `alice`'s salary (because `bob` is `alice`'s manager)

```
$ curl --user bob:password $docker_ip:5000/finance/salary/alice
```

### 7. Check that 'bob' CANNOT see `charlie`'s salary

`bob` is not `charlie`'s manager, so the following command will fail.

```
$ curl --user bob:password $docker_ip:5000/finance/salary/charlie
```

### 8. Change the policy

Allow HR to see anyone's salary, and include `frank` in the group of HR
representatives.  To do so, upload that policy to OPA.

```shell
$ cat >example.rego <<EOF
package httpapi.authz

manager = {"alice": "bob"}
hr = ["frank"]

import input as http_api

default allow = false

# allow people to see their own salaries
allow {
  http_api.method = "GET"
  http_api.path = ["finance", "salary", username]
  username = http_api.user
}

# allow managers to see salaries
allow {
  http_api.method = "GET"
  http_api.path = ["finance", "salary", username]
  manager[username] = http_api.user
}

# allow HR to see salaries
allow {
  http_api.method = "GET"
  http_api.path = ["finance", "salary", username]
  hr[_] = http_api.user
}
EOF
```

When we upload, we upload to the name of the policy module, not the path at which
the policy exists.  If you dig into the docker-compose file, you'll see that the
module name is `api_authz.rego`.  (Using the policy-module name is necessary
because multiple modules can contribute to the same policy path.)

```shell
$ curl -X PUT --data-binary @example.rego http://localhost:8181/v1/policies/api_authz.rego
```

For the sake of the demo we included `manager` and `hr` directly inside the policy.
But in reality that information would be imported from external datasources.

### 9. Check that the new policy works
Check that `frank` can see anyone's salary.

```
$ curl --user frank:password $docker_ip:5000/finance/salary/alice
$ curl --user frank:password $docker_ip:5000/finance/salary/bob
$ curl --user frank:password $docker_ip:5000/finance/salary/charlie
$ curl --user frank:password $docker_ip:5000/finance/salary/frank
```

## Wrap Up
Congratulations for finishing the tutorial!  You learned a number of things
about API authorization with OPA

* OPA gives you fine-grained policy control over APIs once you set up the
  server to ask OPA for authorization
* You write allow/deny policies to control which APIs can be executed by whom
* You can import external data into OPA and write policies that depend on
  that data.


{% endcontentfor %}
