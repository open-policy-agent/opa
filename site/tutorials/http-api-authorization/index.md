---
sort_order: 1002
nav_id: MAIN_TUTORIALS
tutorial_id: HTTP_API_AUTHORIZATION
layout: tutorials

title: HTTP API Authorization
---

{% contentfor header %}
# HTTP API Authorization

Anything that exposes an HTTP API (whether an individual microservice or an application as a whole) needs to control who can run those APIs and when.  OPA makes it easy to write fine-grained, context-aware policies to implement API authorization.

{% endcontentfor %}

{% contentfor body %}

## Goals

In this tutorial, you'll use a simple HTTP web server that accepts any HTTP GET
request that you issue and echoes the OPA decision back as text. Both OPA and
the web server will be run as containers.

For this tutorial, our desired policy is:

* People can see their own salaries (`GET /finance/salary/{user}` is permitted for `{user}`)
* A manager can see their direct reports' salaries (`GET /finance/salary/{user}` is permitted for `{user}`'s manager)

## Prerequisites

This tutorial requires [Docker Compose](https://docs.docker.com/compose/install/) to run a demo web server along with OPA.

## Steps

### 1. Bootstrap the tutorial environment using Docker Compose.

First, create a docker-compose.yml file that runs OPA and the demo web server.

```shell
cat >docker-compose.yml <<EOF
version: '2'
services:
  opa:
    image: openpolicyagent/opa:0.4.9
    ports:
      - 8181:8181
    command:
      - "run"
      - "--server"
      - "--log-level=debug"
  api_server:
    image: openpolicyagent/demo-restful-api:latest
    ports:
      - 5000:5000
    environment:
      - OPA_ADDR=http://opa:8181
      - POLICY_PATH=/v1/data/httpapi/authz
EOF
```
{: .opa-collapse--ignore}

Then run `docker-compose` to pull and run the containers.

```shell
docker-compose -f docker-compose.yml up
```

### 2. Load a simple policy into OPA.

In another terminal, create a simple policy. The policy below allows users to
request their own salary as well as the salary of their direct subordinates.

```shell
cat >example.rego <<EOF
package httpapi.authz

# bob is alice's manager, and betty is charlie's.
subordinates = {"alice": [], "charlie": [], "bob": ["alice"], "betty": ["charlie"]}

# HTTP API request
import input as http_api

default allow = false

# Allow users to get their own salaries.
allow {
  http_api.method = "GET"
  http_api.path = ["finance", "salary", username]
  username = http_api.user
}

# Allow managers to get their subordinates' salaries.
allow {
  http_api.method = "GET"
  http_api.path = ["finance", "salary", username]
  subordinates[http_api.user][_] = username
}
EOF
```
{: .opa-collapse--ignore}

Then load the policy via OPA's REST API.

```shell
curl -X PUT --data-binary @example.rego \
  localhost:8181/v1/policies/example
```

### 3. Check that `alice` can see her own salary.

The following command will succeed.

```shell
curl --user alice:password localhost:5000/finance/salary/alice
```

### 4. Check that `bob` can see `alice`'s salary (because `bob` is `alice`'s manager.)

```shell
curl --user bob:password localhost:5000/finance/salary/alice
```

### 5. Check that `bob` CANNOT see `charlie`'s salary.

`bob` is not `charlie`'s manager, so the following command will fail.

```shell
curl --user bob:password localhost:5000/finance/salary/charlie
```

### 6. Change the policy.

Suppose the organization now includes an HR department. The organization wants
members of HR to be able to see any salary. Let's extend the policy to handle
this.

```shell
cat >example-hr.rego <<EOF
package httpapi.authz

import input as http_api

# Allow HR members to get anyone's salary.
allow {
  http_api.method = "GET"
  http_api.path = ["finance", "salary", _]
  hr[_] = http_api.user
}

# David is the only member of HR.
hr = [
  "david",
]
EOF
```
{: .opa-collapse--ignore}

Upload the new policy to OPA.

```shell
curl -X PUT --data-binary @example-hr.rego \
  http://localhost:8181/v1/policies/example-hr
```

For the sake of the tutorial we included `manager_of` and `hr` data directly
inside the policies. In real-world scenarios that information would be imported
from external data sources.

### 7. Check that the new policy works.
Check that `david` can see anyone's salary.

```shell
curl --user david:password localhost:5000/finance/salary/alice
curl --user david:password localhost:5000/finance/salary/bob
curl --user david:password localhost:5000/finance/salary/charlie
curl --user david:password localhost:5000/finance/salary/david
```

## Wrap Up

Congratulations for finishing the tutorial!

You learned a number of things about API authorization with OPA:

* OPA gives you fine-grained policy control over APIs once you set up the
  server to ask OPA for authorization.
* You write allow/deny policies to control which APIs can be executed by whom.
* You can import external data into OPA and write policies that depend on
  that data.

The code for this tutorial can be found in the
[open-policy-agent/contrib](https://github.com/open-policy-agent/contrib)
repository.

{% endcontentfor %}
